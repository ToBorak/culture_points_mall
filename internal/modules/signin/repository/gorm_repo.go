package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/standardsoftware/culture_points_mall/internal/modules/signin/domain"
)

type GormRepo struct{ DB *gorm.DB }

func New(db *gorm.DB) *GormRepo { return &GormRepo{DB: db} }

func (r *GormRepo) CreateCode(ctx context.Context, c *domain.SigninCode) error {
	return r.DB.WithContext(ctx).Create(c).Error
}

func (r *GormRepo) CreateRecord(ctx context.Context, rec *domain.SigninRecord) error {
	return r.DB.WithContext(ctx).Create(rec).Error
}

func (r *GormRepo) HasUserSignedIn(ctx context.Context, activityID, userID int64) (bool, error) {
	var cnt int64
	err := r.DB.WithContext(ctx).Model(&domain.SigninRecord{}).
		Where("activity_id = ? AND user_id = ? AND result = 'passed'", activityID, userID).
		Count(&cnt).Error
	return cnt > 0, err
}
