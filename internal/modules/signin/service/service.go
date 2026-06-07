package service

import (
	"context"
	"errors"
	"time"

	activitiessvc "github.com/standardsoftware/culture_points_mall/internal/modules/activities/service"
	pointssvc "github.com/standardsoftware/culture_points_mall/internal/modules/points/service"
	"github.com/standardsoftware/culture_points_mall/internal/modules/signin/domain"
	"github.com/standardsoftware/culture_points_mall/internal/modules/signin/repository"
)

type Service struct {
	Repo       *repository.GormRepo
	Activities *activitiessvc.Service
	Points     *pointssvc.Service
	HMACSecret string
	WindowSecs int
}

func New(repo *repository.GormRepo, act *activitiessvc.Service, p *pointssvc.Service, secret string, windowSecs int) *Service {
	if windowSecs <= 0 {
		windowSecs = 60
	}
	return &Service{Repo: repo, Activities: act, Points: p, HMACSecret: secret, WindowSecs: windowSecs}
}

type CheckCmd struct {
	TenantID   int64
	UserID     int64
	ActivityID int64
	Code       string
}

type CheckResult struct {
	OK            bool
	Reason        string
	TransactionID int64
	Points        int
}

var ErrAlreadySignedIn = errors.New("已经签到过本活动")

func (s *Service) Check(ctx context.Context, cmd CheckCmd) (*CheckResult, error) {
	if !ValidCode(cmd.ActivityID, cmd.Code, s.WindowSecs, s.HMACSecret, time.Now()) {
		return s.reject(ctx, cmd, "二维码无效或已过期")
	}
	already, err := s.Repo.HasUserSignedIn(ctx, cmd.ActivityID, cmd.UserID)
	if err != nil {
		return nil, err
	}
	if already {
		return nil, ErrAlreadySignedIn
	}
	act, err := s.Activities.GetByID(ctx, cmd.TenantID, cmd.ActivityID)
	if err != nil {
		return s.reject(ctx, cmd, "活动不存在")
	}

	rec := &domain.SigninRecord{ActivityID: cmd.ActivityID, UserID: cmd.UserID, Result: "passed"}
	if err := s.Repo.CreateRecord(ctx, rec); err != nil {
		return nil, err
	}

	reward := act.PointsReward
	if reward <= 0 {
		reward = 10
	}
	tx, err := s.Points.AddPoints(ctx, pointssvc.AddPointsCmd{
		TenantID: cmd.TenantID, UserID: cmd.UserID, Amount: reward,
		DimensionID: act.DimensionID, ActivityID: &act.ID, Reason: "签到加分 · " + act.Title,
	})
	if err != nil {
		return nil, err
	}
	// 签到通过即视为「已参加」：把报名状态置为 checked_in（无报名记录则补建）。
	_ = s.Activities.MarkCheckedIn(ctx, cmd.ActivityID, cmd.UserID)
	// 勋章授予不在此处自发，统一由全局 BadgeCelebration（POST /me/badges/check）结算，
	// 否则会在「达成弹窗」之前把勋章悄悄发掉，签到首达成就没有弹窗提示。
	return &CheckResult{OK: true, TransactionID: tx.ID, Points: reward}, nil
}

func (s *Service) reject(ctx context.Context, cmd CheckCmd, reason string) (*CheckResult, error) {
	rec := &domain.SigninRecord{
		ActivityID: cmd.ActivityID, UserID: cmd.UserID,
		Result: "rejected", Reason: reason,
	}
	_ = s.Repo.CreateRecord(ctx, rec)
	return &CheckResult{OK: false, Reason: reason}, nil
}

func (s *Service) CurrentCode(activityID int64) string {
	return CodeFor(activityID, time.Now().Unix()/int64(s.WindowSecs), s.HMACSecret)
}
