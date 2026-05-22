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

	var newly []int64
	for _, b := range badges {
		if ownedSet[b.ID] {
			continue
		}
		if b.DimensionID != dimensionID {
			continue
		}
		var rule domain.Rule
		if err := json.Unmarshal(b.RuleJSON, &rule); err != nil {
			continue
		}
		if rule.Type == "accumulated" && scoreByDim[b.DimensionID] >= rule.Threshold {
			if err := s.Repo.Inner.AwardBadge(ctx, userID, b.ID); err != nil {
				return nil, err
			}
			newly = append(newly, b.ID)
		}
	}
	return newly, nil
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
