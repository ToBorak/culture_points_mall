// Package publish 编排"发布活动"：建活动 → 解析参与人（默认全员）→ 建钉钉日程（日历+会议室+可选群推送）。
// 供 HR-Agent 日程表单提交后调用，把活动与日程一次性落地。
package publish

import (
	"context"
	"strconv"
	"strings"
	"time"

	activitiesdomain "github.com/standardsoftware/culture_points_mall/internal/modules/activities/domain"
	activitiessvc "github.com/standardsoftware/culture_points_mall/internal/modules/activities/service"
	scheduledomain "github.com/standardsoftware/culture_points_mall/internal/modules/schedule/domain"
	schedulesvc "github.com/standardsoftware/culture_points_mall/internal/modules/schedule/service"
	usersdomain "github.com/standardsoftware/culture_points_mall/internal/modules/users/domain"
)

// 三个依赖用窄接口表达，便于单测注入假实现；对应的 concrete service 隐式满足。
type ActivityCreator interface {
	Create(ctx context.Context, cmd activitiessvc.CreateCmd) (*activitiesdomain.Activity, error)
}
type SchedulePublisher interface {
	Create(ctx context.Context, cmd schedulesvc.CreateCmd) (*scheduledomain.Schedule, error)
}
type AttendeeLister interface {
	List(ctx context.Context, tenantID int64) ([]usersdomain.User, error)
}

type Service struct {
	Activities ActivityCreator
	Schedule   SchedulePublisher
	Users      AttendeeLister
	H5BaseURL  string // 文化官 H5 对外访问地址；非空时群卡片「查看详情」跳到 H5BaseURL/activities/<活动id>
}

func New(a ActivityCreator, s SchedulePublisher, u AttendeeLister) *Service {
	return &Service{Activities: a, Schedule: s, Users: u}
}

// WithH5BaseURL 注入 H5 对外地址，用于把群卡片「查看详情」按钮指向对应活动详情页。
func (s *Service) WithH5BaseURL(u string) *Service {
	s.H5BaseURL = u
	return s
}

// activityURL 拼活动详情页链接；未配 H5BaseURL 或无活动 id 时返回空串(群卡片按钮回退默认)。
func (s *Service) activityURL(activityID int64) string {
	if s.H5BaseURL == "" || activityID <= 0 {
		return ""
	}
	return strings.TrimRight(s.H5BaseURL, "/") + "/activities/" + strconv.FormatInt(activityID, 10)
}

type Cmd struct {
	TenantID        int64
	CreatedBy       int64
	DimensionCode   string
	Title           string
	StartAt         time.Time
	EndAt           time.Time
	PointsReward    int
	Capacity        *int
	Location        string   // 展示用文案（如会议室名），可空；会议室真正占用走 RoomIDs
	Detail          string   // 活动说明，进日程描述
	RoomIDs         []string // 钉钉会议室 roomId
	GroupIDs        []string // 群机器人 id（推群用）
	PushGroup       bool
	AttendeeAll     bool     // true=全员
	AttendeeUserIDs []string // 非全员时的钉钉 userid 列表
	SkipSchedule    bool     // true=只创建活动，不建钉钉日程（不解析参与人、不推会议室/群）
}

// Stage 单个执行阶段的结果，供上层逐条渲染成对话气泡。
type Stage struct {
	Name   string         `json:"name"`
	OK     bool           `json:"ok"`
	Output map[string]any `json:"output,omitempty"`
	Error  string         `json:"error,omitempty"`
}

type Result struct {
	ActivityID      int64
	Title           string
	CalendarEventID string
	ScheduleStatus  string
	AttendeeCount   int
	ScheduleSkipped bool // 用户选择「不创建日程」时为 true
	Stages          []Stage
}

// Publish 顺序执行各阶段；任一阶段失败即短路返回，错误进 Stage 不抛 Go error，
// 方便上层把"建活动成功、建日程失败"这类部分结果如实展示。
func (s *Service) Publish(ctx context.Context, cmd Cmd) Result {
	var res Result
	res.Title = cmd.Title

	// 阶段1：建活动
	act, err := s.Activities.Create(ctx, activitiessvc.CreateCmd{
		TenantID:      cmd.TenantID,
		DimensionCode: cmd.DimensionCode,
		Title:         cmd.Title,
		StartAt:       &cmd.StartAt,
		EndAt:         &cmd.EndAt,
		Capacity:      cmd.Capacity,
		PointsReward:  cmd.PointsReward,
	})
	if err != nil {
		res.Stages = append(res.Stages, Stage{Name: "create_activity", OK: false, Error: err.Error()})
		return res
	}
	res.ActivityID = act.ID
	res.Stages = append(res.Stages, Stage{Name: "create_activity", OK: true, Output: map[string]any{
		"activity_id": act.ID, "title": act.Title, "status": string(act.Status),
	}})

	// 用户选择「不创建日程」：到此为止，只落地活动本身（也就不会去解析参与人/建日历/推群）。
	if cmd.SkipSchedule {
		res.ScheduleSkipped = true
		return res
	}

	// 解析参与人：全员 = 本租户所有有 ding_user_id 的成员
	attendees := cmd.AttendeeUserIDs
	if cmd.AttendeeAll {
		users, err := s.Users.List(ctx, cmd.TenantID)
		if err != nil {
			res.Stages = append(res.Stages, Stage{Name: "resolve_attendees", OK: false, Error: err.Error()})
			return res
		}
		attendees = make([]string, 0, len(users))
		for _, u := range users {
			if strings.TrimSpace(u.DingUserID) != "" {
				attendees = append(attendees, u.DingUserID)
			}
		}
	}
	res.AttendeeCount = len(attendees)

	// 阶段2：建日程（钉钉日历 + 会议室 + 可选群推送）
	sch, err := s.Schedule.Create(ctx, schedulesvc.CreateCmd{
		TenantID:        cmd.TenantID,
		Title:           cmd.Title,
		StartAt:         cmd.StartAt,
		EndAt:           cmd.EndAt,
		Location:        cmd.Location,
		Detail:          cmd.Detail,
		AttendeeUserIDs: attendees,
		GroupIDs:        cmd.GroupIDs,
		RoomIDs:         cmd.RoomIDs,
		PushCalendar:    true,
		PushGroup:       cmd.PushGroup,
		CreatedBy:       cmd.CreatedBy,
		DetailURL:       s.activityURL(act.ID),
	})
	if err != nil {
		res.Stages = append(res.Stages, Stage{Name: "create_schedule", OK: false, Error: err.Error()})
		return res
	}
	res.CalendarEventID = sch.CalendarEventID
	res.ScheduleStatus = string(sch.Status)
	res.Stages = append(res.Stages, Stage{Name: "create_schedule", OK: true, Output: map[string]any{
		"calendar_event_id": sch.CalendarEventID,
		"status":            string(sch.Status),
		"attendees":         res.AttendeeCount,
		"note":              sch.ResultNote,
	}})
	return res
}
