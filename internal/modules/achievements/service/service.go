package service

import (
	"context"
	"encoding/json"
	"sort"

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
	CountPassedSignins(ctx context.Context, userID int64) (int, error)
	ListPendingBadges(ctx context.Context, userID int64) ([]domain.Badge, error)
	MarkCelebrated(ctx context.Context, userID int64, badgeIDs []int64) error
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
	signinCount, err := s.Repo.Inner.CountPassedSignins(ctx, userID)
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
		case "signin_count":
			unlocked = signinCount >= rule.Threshold
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

// CheckNew 供 H5 全局庆祝弹窗调用：先结算授予（新达成的勋章写入 user_badges 且 celebrated=0），
// 再返回所有「尚未庆祝」的已得勋章。授予与庆祝解耦——返回的勋章只有在前端展示后回执
// （MarkCelebrated）才会落定，否则下次仍会返回，确保竞态/刷新/崩溃下零丢失。
func (s *Service) CheckNew(ctx context.Context, tenantID, userID int64) ([]domain.Badge, error) {
	if _, err := s.CheckTriggers(ctx, tenantID, userID, 0); err != nil {
		return nil, err
	}
	return s.Repo.Inner.ListPendingBadges(ctx, userID)
}

// MarkCelebrated 由前端在勋章弹窗展示后回执调用，落定这些勋章「已庆祝」，之后不再返回。
func (s *Service) MarkCelebrated(ctx context.Context, userID int64, badgeIDs []int64) error {
	return s.Repo.Inner.MarkCelebrated(ctx, userID, badgeIDs)
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
	ProgressCurrent int    // 当前累计值（earned_total / spent_total / signin_count 有意义）
	ProgressTarget  int    // 解锁阈值（0 表示无进度条，如首次类）
	ProgressUnit    string // 进度单位：积分类为"分"，签到类为"次"
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
	signinCount, err := s.Repo.Inner.CountPassedSignins(ctx, userID)
	if err != nil {
		return nil, err
	}
	views := make([]BadgeView, 0, len(all))
	for _, b := range all {
		cur, tgt := 0, 0
		unit := "分"
		var rule domain.Rule
		if json.Unmarshal(b.RuleJSON, &rule) == nil {
			switch rule.Type {
			case "earned_total":
				cur, tgt = earned, rule.Threshold
			case "spent_total":
				cur, tgt = spent, rule.Threshold
			case "signin_count":
				cur, tgt, unit = signinCount, rule.Threshold, "次"
			}
		}
		if tgt > 0 && cur > tgt {
			cur = tgt
		}
		views = append(views, BadgeView{Badge: b, Earned: owned[b.ID], ProgressCurrent: cur, ProgressTarget: tgt, ProgressUnit: unit})
	}
	// 统一展示顺序：首次签到 → 签到次数 → 累计赚取 → 累计消费 → 其它；同类按阈值升序。
	sort.SliceStable(views, func(i, j int) bool {
		oi, ti := badgeOrder(views[i].Badge.RuleJSON)
		oj, tj := badgeOrder(views[j].Badge.RuleJSON)
		if oi != oj {
			return oi < oj
		}
		return ti < tj
	})
	return views, nil
}

// badgeOrder 给勋章一个稳定的展示排序键（类别优先级, 阈值）。
func badgeOrder(raw json.RawMessage) (int, int) {
	var r domain.Rule
	_ = json.Unmarshal(raw, &r)
	order := map[string]int{
		"first_signin": 0,
		"signin_count": 1,
		"earned_total": 2,
		"spent_total":  3,
		"accumulated":  4,
	}
	o, ok := order[r.Type]
	if !ok {
		o = 9
	}
	return o, r.Threshold
}
