package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/standardsoftware/culture_points_mall/internal/modules/users/domain"
)

type GormRepo struct{ DB *gorm.DB }

func New(db *gorm.DB) *GormRepo { return &GormRepo{DB: db} }

func (r *GormRepo) GetByID(ctx context.Context, tenantID, id int64) (*domain.User, error) {
	var u domain.User
	if err := r.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *GormRepo) ListByDept(ctx context.Context, tenantID, deptID int64) ([]domain.User, error) {
	var rows []domain.User
	err := r.DB.WithContext(ctx).Where("tenant_id = ? AND dept_id = ?", tenantID, deptID).Find(&rows).Error
	return rows, err
}
