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
