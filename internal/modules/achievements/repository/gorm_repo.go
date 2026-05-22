package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/standardsoftware/culture_points_mall/internal/modules/achievements/domain"
)

type GormRepo struct{ DB *gorm.DB }

func New(db *gorm.DB) *GormRepo { return &GormRepo{DB: db} }

func (r *GormRepo) ListBadges(ctx context.Context, tenantID int64) ([]domain.Badge, error) {
	var rows []domain.Badge
	err := r.DB.WithContext(ctx).Where("tenant_id = ?", tenantID).Find(&rows).Error
	return rows, err
}

func (r *GormRepo) ListUserBadgeIDs(ctx context.Context, userID int64) ([]int64, error) {
	var ids []int64
	err := r.DB.WithContext(ctx).
		Table("user_badges").Select("badge_id").
		Where("user_id = ?", userID).Scan(&ids).Error
	return ids, err
}

func (r *GormRepo) AwardBadge(ctx context.Context, userID, badgeID int64) error {
	return r.DB.WithContext(ctx).Exec(
		`INSERT IGNORE INTO user_badges (user_id, badge_id) VALUES (?, ?)`,
		userID, badgeID,
	).Error
}
