package service

import (
	"context"
	"encoding/json"

	"github.com/standardsoftware/culture_points_mall/internal/modules/achievements/domain"
	pointssvc "github.com/standardsoftware/culture_points_mall/internal/modules/points/service"
	valuessvc "github.com/standardsoftware/culture_points_mall/internal/modules/values/service"
)

type Service struct {
	Repo   *Wrap
	Points *pointssvc.Service
	Values *valuessvc.Service
}

type repoIface interface {
	ListBadges(ctx context.Context, tenantID int64) ([]domain.Badge, error)
	ListUserBadgeIDs(ctx context.Context, userID int64) ([]int64, error)
	AwardBadge(ctx context.Context, userID, badgeID int64) error
}

type Wrap struct {
	Inner repoIface
}

func New(repo *Wrap, points *pointssvc.Service, values *valuessvc.Service) *Service {
	return &Service{Repo: repo, Points: points, Values: values}
}

func (s *Service) CheckTriggers(ctx context.Context, tenantID, userID, dimensionID int64) ([]int64, error) {
	badges, err := s.Repo.Inner.ListBadges(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	owned, err := s.Repo.Inner.ListUserBadgeIDs(ctx, userID)
	if err != nil {
		return nil, err
	}
	ownedSet := make(map[int64]bool, len(owned))
	for _, id := range owned {
		ownedSet[id] = true
	}

	scores, _, _, err := s.Points.GetUserScores(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	scoreByDim := make(map[int64]int, len(scores))
	for _, sc := range scores {
		scoreByDim[sc.DimensionID] = sc.TotalScore
	}
	earned, err := s.Points.GetEarnedTotal(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	spent, err := s.Points.GetSpentTotal(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	hasActivity, err := s.Points.HasActivityParticipation(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}

	var newly []int64
	for _, b := range badges {
		if ownedSet[b.ID] {
			continue
		}
		var rule domain.Rule
		if err := json.Unmarshal(b.RuleJSON, &rule); err != nil {
			continue
		}
		// dimensionID 参数对全局里程碑勋章不再过滤；按规则类型判定。
		unlocked := false
		switch rule.Type {
		case "accumulated":
			unlocked = scoreByDim[b.DimensionID] >= rule.Threshold
		case "first_signin":
			unlocked = hasActivity
		case "earned_total":
			unlocked = earned >= rule.Threshold
		case "spent_total":
			unlocked = spent >= rule.Threshold
		}
		if unlocked {
			if err := s.Repo.Inner.AwardBadge(ctx, userID, b.ID); err != nil {
				return nil, err
			}
			newly = append(newly, b.ID)
		}
	}
	return newly, nil
}

// CheckNew 结算并返回本次「新解锁」的勋章完整信息，供 H5 全局庆祝弹窗使用。
// 复用 CheckTriggers（它只在首次满足条件时返回该勋章，已拥有的不会再返回）。
func (s *Service) CheckNew(ctx context.Context, tenantID, userID int64) ([]domain.Badge, error) {
	newlyIDs, err := s.CheckTriggers(ctx, tenantID, userID, 0)
	if err != nil {
		return nil, err
	}
	if len(newlyIDs) == 0 {
		return nil, nil
	}
	all, err := s.Repo.Inner.ListBadges(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]domain.Badge, len(all))
	for _, b := range all {
		byID[b.ID] = b
	}
	out := make([]domain.Badge, 0, len(newlyIDs))
	for _, id := range newlyIDs {
		if b, ok := byID[id]; ok {
			out = append(out, b)
		}
	}
	return out, nil
}

func (s *Service) ListMyBadges(ctx context.Context, tenantID, userID int64) ([]domain.Badge, map[int64]bool, error) {
	all, err := s.Repo.Inner.ListBadges(ctx, tenantID)
	if err != nil {
		return nil, nil, err
	}
	ids, err := s.Repo.Inner.ListUserBadgeIDs(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	owned := make(map[int64]bool, len(ids))
	for _, id := range ids {
		owned[id] = true
	}
	return all, owned, nil
}

func (s *Service) AwardBadge(ctx context.Context, userID, badgeID int64) error {
	return s.Repo.Inner.AwardBadge(ctx, userID, badgeID)
}

// BadgeView 是带"已获得/进度"的勋章视图，供前端勋章墙展示。
type BadgeView struct {
	Badge           domain.Badge
	Earned          bool
	ProgressCurrent int // 当前累计值（仅 earned_total / spent_total 有意义）
	ProgressTarget  int // 解锁阈值（0 表示无进度条，如首次类）
}

// ListMyBadgeViews 返回带进度的勋章列表。结算/授予由全局 BadgeCelebration（POST /me/badges/check）统一负责，
// 这里不再懒结算——否则会在「达成弹窗」之前把勋章悄悄发掉，就没有达成提示了。
func (s *Service) ListMyBadgeViews(ctx context.Context, tenantID, userID int64) ([]BadgeView, error) {
	all, owned, err := s.ListMyBadges(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	earned, err := s.Points.GetEarnedTotal(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	spent, err := s.Points.GetSpentTotal(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	views := make([]BadgeView, 0, len(all))
	for _, b := range all {
		cur, tgt := 0, 0
		var rule domain.Rule
		if json.Unmarshal(b.RuleJSON, &rule) == nil {
			switch rule.Type {
			case "earned_total":
				cur, tgt = earned, rule.Threshold
			case "spent_total":
				cur, tgt = spent, rule.Threshold
			}
		}
		if tgt > 0 && cur > tgt {
			cur = tgt
		}
		views = append(views, BadgeView{Badge: b, Earned: owned[b.ID], ProgressCurrent: cur, ProgressTarget: tgt})
	}
	return views, nil
}
