package service

import (
	"context"
	"errors"
	"time"

	"github.com/standardsoftware/culture_points_mall/internal/modules/activities/domain"
	"github.com/standardsoftware/culture_points_mall/internal/modules/activities/repository"
	usersdomain "github.com/standardsoftware/culture_points_mall/internal/modules/users/domain"
	valuessvc "github.com/standardsoftware/culture_points_mall/internal/modules/values/service"
	"github.com/standardsoftware/culture_points_mall/internal/platform/dingtalk"
)

// UserResolver 按 id 取成员，用于拿到报名用户的钉钉 userid 作为日程组织者/参与人。
// *users/service.Service 满足它。
type UserResolver interface {
	GetByID(ctx context.Context, tenantID, id int64) (*usersdomain.User, error)
}

type Service struct {
	Repo   *repository.GormRepo
	Values *valuessvc.Service
	Ding   dingtalk.Client // 可空：报名时自动写入 / 取消时移除钉钉日历
	Users  UserResolver    // 可空：解析报名用户的钉钉 userid
}

func New(r *repository.GormRepo, v *valuessvc.Service) *Service {
	return &Service{Repo: r, Values: v}
}

// WithDing 注入钉钉客户端，启用「报名自动加入日程 / 取消移除日程」。
func (s *Service) WithDing(c dingtalk.Client) *Service { s.Ding = c; return s }

// WithUsers 注入成员查询，用于解析报名用户的钉钉 userid。
func (s *Service) WithUsers(u UserResolver) *Service { s.Users = u; return s }

type CreateCmd struct {
	TenantID      int64
	DimensionCode string
	Title         string
	StartAt       *time.Time
	EndAt         *time.Time
	Capacity      *int
	PointsReward  int
}

var (
	ErrInvalidDimension = errors.New("dimension code not found")
	ErrActivityClosed   = errors.New("活动已结束，无法报名")
	ErrActivityFull     = errors.New("活动名额已满")
)

func (s *Service) Create(ctx context.Context, cmd CreateCmd) (*domain.Activity, error) {
	dims, err := s.Values.GetDimensions(ctx, cmd.TenantID)
	if err != nil {
		return nil, err
	}
	var dimID int64
	for _, d := range dims {
		if d.Code == cmd.DimensionCode {
			dimID = d.ID
			break
		}
	}
	if dimID == 0 {
		return nil, ErrInvalidDimension
	}
	a := &domain.Activity{
		TenantID:     cmd.TenantID,
		DimensionID:  dimID,
		Title:        cmd.Title,
		Status:       domain.StatusPublished,
		Capacity:     cmd.Capacity,
		StartAt:      cmd.StartAt,
		EndAt:        cmd.EndAt,
		PointsReward: cmd.PointsReward,
	}
	if err := s.Repo.Create(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *Service) List(ctx context.Context, tenantID int64, status domain.Status) ([]domain.Activity, error) {
	return s.Repo.ListByTenant(ctx, tenantID, status, 50)
}

func (s *Service) GetByID(ctx context.Context, tenantID, id int64) (*domain.Activity, error) {
	return s.Repo.GetByID(ctx, tenantID, id)
}

// ---- 用户侧视图（列表 / 详情 / 报名） ----

// MineView 当前用户与该活动的关系。
type MineView struct {
	Enrolled   bool   `json:"enrolled"`
	Status     string `json:"status"` // "" | enrolled | checked_in | absent
	CheckedIn  bool   `json:"checkedIn"`
	InCalendar bool   `json:"inCalendar"` // 报名是否已自动加入该用户的钉钉日历
}

// ActivityView 嵌入原始活动结构（保持 ID/Title/Status… 等 PascalCase 字段不变，
// 向后兼容 admin 端消费 /api/v1/activities），并追加用户侧聚合字段。
type ActivityView struct {
	domain.Activity
	DimensionCode string   `json:"dimensionCode"`
	DimensionName string   `json:"dimensionName"`
	EnrolledCount int      `json:"enrolledCount"`
	Mine          MineView `json:"mine"`
}

type dimInfo struct{ code, name string }

func (s *Service) dimIndex(ctx context.Context, tenantID int64) map[int64]dimInfo {
	dims, err := s.Values.GetDimensions(ctx, tenantID)
	if err != nil {
		return map[int64]dimInfo{}
	}
	m := make(map[int64]dimInfo, len(dims))
	for _, d := range dims {
		m[d.ID] = dimInfo{code: d.Code, name: d.Name}
	}
	return m
}

func (s *Service) toView(a domain.Activity, idx map[int64]dimInfo, enrolledCount int, en *domain.Enrollment) ActivityView {
	di := idx[a.DimensionID]
	var mine MineView
	if en != nil {
		mine = MineView{Enrolled: true, Status: string(en.Status), CheckedIn: en.Status == domain.EnrollCheckedIn, InCalendar: en.CalendarEventID != ""}
	}
	return ActivityView{
		Activity:      a,
		DimensionCode: di.code,
		DimensionName: di.name,
		EnrolledCount: enrolledCount,
		Mine:          mine,
	}
}

// ListView 返回带报名人数与「我的」状态的活动卡片列表。
func (s *Service) ListView(ctx context.Context, tenantID, userID int64, status domain.Status) ([]ActivityView, error) {
	rows, err := s.Repo.ListByTenant(ctx, tenantID, status, 50)
	if err != nil {
		return nil, err
	}
	idx := s.dimIndex(ctx, tenantID)
	ids := make([]int64, len(rows))
	for i, a := range rows {
		ids[i] = a.ID
	}
	counts, err := s.Repo.CountsByActivityIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	mine, err := s.Repo.EnrollmentsByUserForActivities(ctx, userID, ids)
	if err != nil {
		return nil, err
	}
	out := make([]ActivityView, len(rows))
	for i, a := range rows {
		var en *domain.Enrollment
		if e, ok := mine[a.ID]; ok {
			en = &e
		}
		out[i] = s.toView(a, idx, int(counts[a.ID]), en)
	}
	return out, nil
}

// Detail 返回单个活动的完整视图。活动不存在时返回 gorm.ErrRecordNotFound。
func (s *Service) Detail(ctx context.Context, tenantID, userID, activityID int64) (*ActivityView, error) {
	act, err := s.Repo.GetByID(ctx, tenantID, activityID)
	if err != nil {
		return nil, err
	}
	cnt, err := s.Repo.CountEnrolled(ctx, activityID)
	if err != nil {
		return nil, err
	}
	en, err := s.Repo.GetEnrollment(ctx, activityID, userID)
	if err != nil {
		return nil, err
	}
	v := s.toView(*act, s.dimIndex(ctx, tenantID), int(cnt), en)
	return &v, nil
}

// Enroll 报名（幂等）。校验活动状态与名额；成功返回最新详情视图。
func (s *Service) Enroll(ctx context.Context, tenantID, userID, activityID int64) (*ActivityView, error) {
	act, err := s.Repo.GetByID(ctx, tenantID, activityID)
	if err != nil {
		return nil, err
	}
	if act.Status == domain.StatusClosed {
		return nil, ErrActivityClosed
	}
	existing, err := s.Repo.GetEnrollment(ctx, activityID, userID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		if act.Capacity != nil && *act.Capacity > 0 {
			cnt, err := s.Repo.CountEnrolled(ctx, activityID)
			if err != nil {
				return nil, err
			}
			if cnt >= int64(*act.Capacity) {
				return nil, ErrActivityFull
			}
		}
		if err := s.Repo.Enroll(ctx, activityID, userID); err != nil {
			return nil, err
		}
		// 报名成功后把活动写进该用户的钉钉日历（尽力而为，失败不影响报名结果）。
		s.addToCalendar(ctx, tenantID, userID, act)
	}
	return s.Detail(ctx, tenantID, userID, activityID)
}

// Unenroll 取消报名；成功返回最新详情视图。
func (s *Service) Unenroll(ctx context.Context, tenantID, userID, activityID int64) (*ActivityView, error) {
	if _, err := s.Repo.GetByID(ctx, tenantID, activityID); err != nil {
		return nil, err
	}
	// 取消报名前，先尽力移除报名时自动加入的钉钉日程（失败不影响取消）。
	if en, err := s.Repo.GetEnrollment(ctx, activityID, userID); err == nil && en != nil && en.CalendarEventID != "" {
		s.removeFromCalendar(ctx, tenantID, userID, en.CalendarEventID)
	}
	if err := s.Repo.Unenroll(ctx, activityID, userID); err != nil {
		return nil, err
	}
	return s.Detail(ctx, tenantID, userID, activityID)
}

// addToCalendar 报名成功后把活动写进该用户自己的钉钉日历：组织者与参与人都设为用户本人，
// 事件即落在用户主日历上，取消报名时可用同一 userid 删除。尽力而为，任何前置缺失或失败都静默忽略。
func (s *Service) addToCalendar(ctx context.Context, tenantID, userID int64, act *domain.Activity) {
	if s.Ding == nil || s.Users == nil || act.StartAt == nil {
		return
	}
	u, err := s.Users.GetByID(ctx, tenantID, userID)
	if err != nil || u == nil || u.DingUserID == "" {
		return
	}
	start := *act.StartAt
	end := start.Add(time.Hour)
	if act.EndAt != nil {
		end = *act.EndAt
	}
	eventID, err := s.Ding.CreateCalendarEvent(ctx, dingtalk.CalendarRequest{
		Title:           act.Title,
		Detail:          "文化官活动 · 记得到现场扫码签到领积分",
		StartAt:         start,
		EndAt:           end,
		OrganizerUserID: u.DingUserID,
		UserIDs:         []string{u.DingUserID},
	})
	if err != nil || eventID == "" {
		return
	}
	_ = s.Repo.SetCalendarEventID(ctx, act.ID, userID, eventID)
}

// removeFromCalendar 取消报名时删除报名自动加入的钉钉日程。尽力而为，失败静默忽略。
func (s *Service) removeFromCalendar(ctx context.Context, tenantID, userID int64, eventID string) {
	if s.Ding == nil || s.Users == nil {
		return
	}
	u, err := s.Users.GetByID(ctx, tenantID, userID)
	if err != nil || u == nil || u.DingUserID == "" {
		return
	}
	_ = s.Ding.DeleteCalendarEvent(ctx, u.DingUserID, eventID)
}

// MarkCheckedIn 签到通过后由 signin 模块调用，将报名状态置为 checked_in（无则补建）。
func (s *Service) MarkCheckedIn(ctx context.Context, activityID, userID int64) error {
	return s.Repo.MarkCheckedIn(ctx, activityID, userID)
}

// Delete 删除活动（用于「撤销发布活动」回撤）。
func (s *Service) Delete(ctx context.Context, tenantID, id int64) error {
	return s.Repo.Delete(ctx, tenantID, id)
}

// SetStatus 改活动状态（批量关闭/重新发布），返回改之前的状态供「回撤」还原。
func (s *Service) SetStatus(ctx context.Context, tenantID, id int64, status domain.Status) (domain.Status, error) {
	a, err := s.Repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return "", err
	}
	if err := s.Repo.UpdateStatus(ctx, id, status); err != nil {
		return "", err
	}
	return a.Status, nil
}
