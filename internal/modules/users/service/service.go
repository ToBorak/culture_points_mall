package service

import (
	"context"

	"github.com/standardsoftware/culture_points_mall/internal/modules/users/domain"
	"github.com/standardsoftware/culture_points_mall/internal/modules/users/repository"
)

type Service struct{ Repo *repository.GormRepo }

func New(r *repository.GormRepo) *Service { return &Service{Repo: r} }

func (s *Service) GetByID(ctx context.Context, tenantID, id int64) (*domain.User, error) {
	return s.Repo.GetByID(ctx, tenantID, id)
}

func (s *Service) List(ctx context.Context, tenantID int64) ([]domain.User, error) {
	return s.Repo.ListByTenant(ctx, tenantID, 200)
}

func (s *Service) ListWithDept(ctx context.Context, tenantID int64) ([]repository.UserWithDept, error) {
	return s.Repo.ListWithDept(ctx, tenantID)
}
