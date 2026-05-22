package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/standardsoftware/culture_points_mall/internal/modules/activities/domain"
)

type GormRepo struct{ DB *gorm.DB }

func New(db *gorm.DB) *GormRepo { return &GormRepo{DB: db} }

func (r *GormRepo) Create(ctx context.Context, a *domain.Activity) error {
	return r.DB.WithContext(ctx).Create(a).Error
}

func (r *GormRepo) GetByID(ctx context.Context, tenantID, id int64) (*domain.Activity, error) {
	var a domain.Activity
	if err := r.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&a).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *GormRepo) ListByTenant(ctx context.Context, tenantID int64, status domain.Status, limit int) ([]domain.Activity, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	// MySQL 没有 NULLS LAST，用 IS NULL trick
	q := r.DB.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("start_at IS NULL, start_at DESC, id DESC").
		Limit(limit)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var rows []domain.Activity
	err := q.Find(&rows).Error
	return rows, err
}

func (r *GormRepo) UpdateStatus(ctx context.Context, id int64, status domain.Status) error {
	return r.DB.WithContext(ctx).
		Model(&domain.Activity{}).
		Where("id = ?", id).
		Update("status", status).
		Error
}
