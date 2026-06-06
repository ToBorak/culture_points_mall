package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/standardsoftware/culture_points_mall/internal/modules/schedule/domain"
)

type GormRepo struct{ DB *gorm.DB }

func New(db *gorm.DB) *GormRepo { return &GormRepo{DB: db} }

func (r *GormRepo) Create(ctx context.Context, s *domain.Schedule) error {
	return r.DB.WithContext(ctx).Create(s).Error
}

func (r *GormRepo) ListByTenant(ctx context.Context, tenantID int64, limit int) ([]domain.Schedule, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows []domain.Schedule
	err := r.DB.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("id DESC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}
