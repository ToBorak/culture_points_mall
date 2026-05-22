package service

import (
	"context"
	"errors"
	"time"

	activitiessvc "github.com/standardsoftware/culture_points_mall/internal/modules/activities/service"
	achvsvc "github.com/standardsoftware/culture_points_mall/internal/modules/achievements/service"
	pointssvc "github.com/standardsoftware/culture_points_mall/internal/modules/points/service"
	"github.com/standardsoftware/culture_points_mall/internal/modules/signin/domain"
	"github.com/standardsoftware/culture_points_mall/internal/modules/signin/repository"
)

type Service struct {
	Repo         *repository.GormRepo
	Activities   *activitiessvc.Service
	Points       *pointssvc.Service
	Achievements *achvsvc.Service
	HMACSecret   string
	WindowSecs   int
}

func New(repo *repository.GormRepo, act *activitiessvc.Service, p *pointssvc.Service, a *achvsvc.Service, secret string, windowSecs int) *Service {
	if windowSecs <= 0 {
		windowSecs = 60
	}
	return &Service{Repo: repo, Activities: act, Points: p, Achievements: a, HMACSecret: secret, WindowSecs: windowSecs}
}

type CheckCmd struct {
	TenantID   int64
	UserID     int64
	ActivityID int64
	Code       string
	GPSLat     *float64
	GPSLng     *float64
	QuizExpect string
	QuizAnswer string
}

type CheckResult struct {
	OK            bool
	Reason        string
	TransactionID int64
	NewBadges     []int64
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
	if act.LocationLat != nil && act.LocationLng != nil && act.RadiusM != nil && *act.RadiusM > 0 {
		if cmd.GPSLat == nil || cmd.GPSLng == nil {
			return s.reject(ctx, cmd, "需要 GPS 定位")
		}
		dist := HaversineMeters(*act.LocationLat, *act.LocationLng, *cmd.GPSLat, *cmd.GPSLng)
		if dist > float64(*act.RadiusM) {
			return s.reject(ctx, cmd, "不在活动地点范围内")
		}
	}
	if cmd.QuizExpect != "" && !CheckQuiz(cmd.QuizExpect, cmd.QuizAnswer) {
		return s.reject(ctx, cmd, "答题错误")
	}

	rec := &domain.SigninRecord{ActivityID: cmd.ActivityID, UserID: cmd.UserID, GPSLat: cmd.GPSLat, GPSLng: cmd.GPSLng, QuizAnswer: cmd.QuizAnswer, Result: "passed"}
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
	newBadges, _ := s.Achievements.CheckTriggers(ctx, cmd.TenantID, cmd.UserID, act.DimensionID)
	return &CheckResult{OK: true, TransactionID: tx.ID, NewBadges: newBadges}, nil
}

func (s *Service) reject(ctx context.Context, cmd CheckCmd, reason string) (*CheckResult, error) {
	rec := &domain.SigninRecord{
		ActivityID: cmd.ActivityID, UserID: cmd.UserID,
		GPSLat: cmd.GPSLat, GPSLng: cmd.GPSLng, QuizAnswer: cmd.QuizAnswer,
		Result: "rejected", Reason: reason,
	}
	_ = s.Repo.CreateRecord(ctx, rec)
	return &CheckResult{OK: false, Reason: reason}, nil
}

func (s *Service) CurrentCode(activityID int64) string {
	return CodeFor(activityID, time.Now().Unix()/int64(s.WindowSecs), s.HMACSecret)
}
