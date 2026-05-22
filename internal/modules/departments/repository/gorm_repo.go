package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/standardsoftware/culture_points_mall/internal/modules/departments/domain"
)

type GormRepo struct{ DB *gorm.DB }

func New(db *gorm.DB) *GormRepo { return &GormRepo{DB: db} }

func (r *GormRepo) List(ctx context.Context, tenantID int64) ([]domain.Department, error) {
	var rows []domain.Department
	err := r.DB.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("id").Find(&rows).Error
	return rows, err
}

func (r *GormRepo) GetByID(ctx context.Context, tenantID, id int64) (*domain.Department, error) {
	var d domain.Department
	if err := r.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}
