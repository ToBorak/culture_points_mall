package service

import (
	"context"
	"errors"
	"time"

	"github.com/standardsoftware/culture_points_mall/internal/modules/activities/domain"
	"github.com/standardsoftware/culture_points_mall/internal/modules/activities/repository"
	valuessvc "github.com/standardsoftware/culture_points_mall/internal/modules/values/service"
)

type Service struct {
	Repo   *repository.GormRepo
	Values *valuessvc.Service
}

func New(r *repository.GormRepo, v *valuessvc.Service) *Service {
	return &Service{Repo: r, Values: v}
}

type CreateCmd struct {
	TenantID      int64
	DimensionCode string
	Title         string
	StartAt       *time.Time
	EndAt         *time.Time
	Capacity      *int
	LocationLat   *float64
	LocationLng   *float64
	RadiusM       *int
	PointsReward  int
}

var ErrInvalidDimension = errors.New("dimension code not found")

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
		LocationLat:  cmd.LocationLat,
		LocationLng:  cmd.LocationLng,
		RadiusM:      cmd.RadiusM,
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
