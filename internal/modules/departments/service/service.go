package service

import (
	"context"

	"github.com/standardsoftware/culture_points_mall/internal/modules/departments/domain"
	"github.com/standardsoftware/culture_points_mall/internal/modules/departments/repository"
)

type Service struct{ Repo *repository.GormRepo }

func New(r *repository.GormRepo) *Service { return &Service{Repo: r} }

func (s *Service) List(ctx context.Context, tenantID int64) ([]domain.Department, error) {
	return s.Repo.List(ctx, tenantID)
}

func (s *Service) GetByID(ctx context.Context, tenantID, id int64) (*domain.Department, error) {
	return s.Repo.GetByID(ctx, tenantID, id)
}
