package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	gomysql "github.com/go-sql-driver/mysql"

	"github.com/standardsoftware/culture_points_mall/internal/config"
	pointssvc "github.com/standardsoftware/culture_points_mall/internal/modules/points/service"
	"github.com/standardsoftware/culture_points_mall/internal/modules/stars/domain"
)

type Service struct {
	Repo   domain.Repository
	Points *pointssvc.Service
	Cfg    config.StarsCfg
}

func New(repo domain.Repository, points *pointssvc.Service, cfg config.StarsCfg) *Service {
	if cfg.NominatePoints == 0 {
		cfg.NominatePoints = 2
	}
	if cfg.NominatedPoints == 0 {
		cfg.NominatedPoints = 4
	}
	if cfg.WinnerPoints == 0 {
		cfg.WinnerPoints = 8
	}
	if cfg.NominateMonthlyCap == 0 {
		cfg.NominateMonthlyCap = 6
	}
	if cfg.NominatedMonthlyCap == 0 {
		cfg.NominatedMonthlyCap = 16
	}
	return &Service{Repo: repo, Points: points, Cfg: cfg}
}

var (
	ErrSeasonNotOpen       = errors.New("当前季次未开放提报")
	ErrDuplicateNomination = errors.New("你已提报过该对象的同一价值观")
	ErrNomineeNotFound     = errors.New("被提名人不存在")
	ErrNotJudging          = errors.New("季次不在评审阶段")
)

// isDuplicateKey 判断是否为 MySQL 唯一键冲突（1062）。
// GORM 的 ErrDuplicatedKey 翻译需要 TranslateError:true，项目不开启，故直接判断驱动层错误。
func isDuplicateKey(err error) bool {
	var mysqlErr *gomysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

// awardable 计算本次是否还能发分：当月已发 = count*per，发后不超 cap 才发。
func awardable(monthlyCount int64, per, cap int) bool {
	return int(monthlyCount)*per+per <= cap
}

// NominateCmd 提报命令。
type NominateCmd struct {
	TenantID    int64
	SeasonID    int64
	NominatorID int64
	NomineeID   int64 // 0 表示自荐，落库时置为 NominatorID
	DimensionID int64
	CaseText    string
}

func monthStart(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
}

func (s *Service) Nominate(ctx context.Context, cmd NominateCmd) (*domain.Nomination, error) {
	season, err := s.Repo.GetSeason(ctx, cmd.TenantID, cmd.SeasonID)
	if err != nil {
		return nil, err
	}
	if season.Status != domain.SeasonNominating {
		return nil, ErrSeasonNotOpen
	}
	now := time.Now()
	if season.NominateStartAt != nil && now.Before(*season.NominateStartAt) {
		return nil, ErrSeasonNotOpen
	}
	if season.NominateEndAt != nil && now.After(*season.NominateEndAt) {
		return nil, ErrSeasonNotOpen
	}

	nomineeID := cmd.NomineeID
	if nomineeID == 0 {
		nomineeID = cmd.NominatorID
	}
	ok, err := s.Repo.UserExists(ctx, cmd.TenantID, nomineeID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNomineeNotFound
	}

	n := &domain.Nomination{
		TenantID:    cmd.TenantID,
		SeasonID:    cmd.SeasonID,
		NominatorID: cmd.NominatorID,
		NomineeID:   nomineeID,
		DimensionID: cmd.DimensionID,
		CaseText:    cmd.CaseText,
		Status:      domain.NominationSubmitted,
	}
	if err := s.Repo.CreateNomination(ctx, n); err != nil {
		if isDuplicateKey(err) {
			return nil, ErrDuplicateNomination
		}
		return nil, err
	}

	since := monthStart(now)
	// 提报人 +2（封顶 6/月）
	if cnt, err := s.Repo.CountNominationsByNominatorSince(ctx, cmd.TenantID, cmd.NominatorID, since); err == nil &&
		awardable(cnt-1, s.Cfg.NominatePoints, s.Cfg.NominateMonthlyCap) {
		_, _ = s.Points.AddPoints(ctx, pointssvc.AddPointsCmd{
			TenantID:    cmd.TenantID,
			UserID:      cmd.NominatorID,
			Amount:      s.Cfg.NominatePoints,
			DimensionID: cmd.DimensionID,
			Reason:      "文化提报",
		})
	}
	// 被提名人 +4（封顶 16/月）；自荐不重复发被提名分
	if nomineeID != cmd.NominatorID {
		if cnt, err := s.Repo.CountNominationsByNomineeSince(ctx, cmd.TenantID, nomineeID, since); err == nil &&
			awardable(cnt-1, s.Cfg.NominatedPoints, s.Cfg.NominatedMonthlyCap) {
			_, _ = s.Points.AddPoints(ctx, pointssvc.AddPointsCmd{
				TenantID:    cmd.TenantID,
				UserID:      nomineeID,
				Amount:      s.Cfg.NominatedPoints,
				DimensionID: cmd.DimensionID,
				Reason:      "被提名加分",
			})
		}
	}
	return n, nil
}

// SeasonQuota 当前季次 + 本月提报剩余可得积分。
type SeasonQuota struct {
	Season            *domain.Season `json:"season"`
	NominateRemaining int            `json:"nominateRemaining"` // 本月提报还能得多少分
}

// CreateSeason 创建季次；未指定 Status 时默认 nominating。
func (s *Service) CreateSeason(ctx context.Context, sn *domain.Season) error {
	if sn.Status == "" {
		sn.Status = domain.SeasonNominating
	}
	return s.Repo.CreateSeason(ctx, sn)
}

// CurrentSeasonWithQuota 查当前活跃季次并附带调用人本月提报剩余可得积分。
func (s *Service) CurrentSeasonWithQuota(ctx context.Context, tenantID, userID int64) (*SeasonQuota, error) {
	season, err := s.Repo.GetCurrentSeason(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	cnt, err := s.Repo.CountNominationsByNominatorSince(ctx, tenantID, userID, monthStart(time.Now()))
	if err != nil {
		return nil, err
	}
	earned := int(cnt) * s.Cfg.NominatePoints
	remaining := s.Cfg.NominateMonthlyCap - earned
	if remaining < 0 {
		remaining = 0
	}
	return &SeasonQuota{Season: season, NominateRemaining: remaining}, nil
}

// MyNominations 查用户本季次的「我提报的」和「被人提报的」记录。
func (s *Service) MyNominations(ctx context.Context, tenantID, userID, seasonID int64) (submitted, received []domain.Nomination, err error) {
	submitted, err = s.Repo.ListNominationsByNominator(ctx, tenantID, userID, seasonID)
	if err != nil {
		return nil, nil, err
	}
	received, err = s.Repo.ListNominationsByNominee(ctx, tenantID, userID, seasonID)
	return submitted, received, err
}

// AdvanceStatus 推进季次状态（管理员操作）。
func (s *Service) AdvanceStatus(ctx context.Context, tenantID, seasonID int64, status domain.SeasonStatus) error {
	return s.Repo.UpdateSeasonStatus(ctx, tenantID, seasonID, status)
}

// Score 评委对提报打分；季次须处于 judging 阶段。
func (s *Service) Score(ctx context.Context, tenantID, seasonID, nominationID int64, score float64) error {
	season, err := s.Repo.GetSeason(ctx, tenantID, seasonID)
	if err != nil {
		return err
	}
	if season.Status != domain.SeasonJudging {
		return ErrNotJudging
	}
	return s.Repo.UpdateNominationScore(ctx, tenantID, nominationID, score)
}

// ListNominations 列出某季次全部提报（评委视角）。
func (s *Service) ListNominations(ctx context.Context, tenantID, seasonID int64) ([]domain.Nomination, error) {
	return s.Repo.ListNominationsBySeason(ctx, tenantID, seasonID)
}

// Pick 是定榜时的一条当选记录。
type Pick struct {
	UserID             int64
	DimensionID        int64
	SourceNominationID *int64
	Citation           string
}

// SelectWinners 幂等定榜：季次须处于 judging；对每个 pick 三步幂等写入。
// exactly-once 依据：winner 表 uk_season_user_dim 是幂等闸门，
// 只有真正新建（created==true）才发 +WinnerPoints；重跑时 winner 已存在
// -> created==false -> 不重复发分。
func (s *Service) SelectWinners(ctx context.Context, tenantID, seasonID int64, picks []Pick) error {
	season, err := s.Repo.GetSeason(ctx, tenantID, seasonID)
	if err != nil {
		return err
	}
	if season.Status != domain.SeasonJudging {
		return ErrNotJudging
	}
	for _, p := range picks {
		// 1) 幂等建 winner（uk 命中则 created=false）
		var citation *string
		if p.Citation != "" {
			c := p.Citation
			citation = &c
		}
		created, err := s.Repo.CreateWinnerIfAbsent(ctx, &domain.Winner{
			TenantID:           tenantID,
			SeasonID:           seasonID,
			UserID:             p.UserID,
			DimensionID:        p.DimensionID,
			Citation:           citation,
			SourceNominationID: p.SourceNominationID,
		})
		if err != nil {
			return err
		}
		// 2) 幂等置提名 status=selected
		if p.SourceNominationID != nil {
			if err := s.Repo.UpdateNominationStatus(ctx, tenantID, *p.SourceNominationID, domain.NominationSelected); err != nil {
				return err
			}
		}
		// 3) 幂等发评选积分：仅当本次确为新晋 winner 才发（避免重跑重复发）
		if created {
			_, err := s.Points.AddPoints(ctx, pointssvc.AddPointsCmd{
				TenantID:    tenantID,
				UserID:      p.UserID,
				Amount:      s.Cfg.WinnerPoints,
				DimensionID: p.DimensionID,
				Reason:      fmt.Sprintf("评选当选-S%d-D%d", seasonID, p.DimensionID),
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}
