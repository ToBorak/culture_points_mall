package service

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/standardsoftware/culture_points_mall/internal/config"
	"github.com/standardsoftware/culture_points_mall/internal/modules/stars/domain"
	pointssvc "github.com/standardsoftware/culture_points_mall/internal/modules/points/service"
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
		if errors.Is(err, gorm.ErrDuplicatedKey) {
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
