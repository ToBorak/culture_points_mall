package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/standardsoftware/culture_points_mall/internal/modules/values/domain"
)

type GormRepo struct{ DB *gorm.DB }

func New(db *gorm.DB) *GormRepo { return &GormRepo{DB: db} }

func (r *GormRepo) ListByTenant(ctx context.Context, tenantID int64) ([]domain.Dimension, error) {
	var rows []domain.Dimension
	err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND enabled = 1", tenantID).
		Order("sort_order, id").
		Find(&rows).Error
	return rows, err
}

func (r *GormRepo) GetByCode(ctx context.Context, tenantID int64, code string) (*domain.Dimension, error) {
	var d domain.Dimension
	err := r.DB.WithContext(ctx).Where("tenant_id = ? AND code = ?", tenantID, code).First(&d).Error
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *GormRepo) Upsert(ctx context.Context, d *domain.Dimension) error {
	return r.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "code"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "keywords", "weight", "sort_order", "enabled"}),
	}).Create(d).Error
}

func (r *GormRepo) SetEnabled(ctx context.Context, tenantID, id int64, enabled bool) error {
	return r.DB.WithContext(ctx).
		Model(&domain.Dimension{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Update("enabled", enabled).Error
}
