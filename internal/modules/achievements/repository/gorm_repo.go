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

// CountPassedSignins 统计用户「成功签到」的不同活动数（按 activity_id 去重），用于签到类勋章。
// 去重避免同一活动若产生多条 passed 记录时把签到进度重复计数。
func (r *GormRepo) CountPassedSignins(ctx context.Context, userID int64) (int, error) {
	var cnt int64
	err := r.DB.WithContext(ctx).
		Table("signin_records").
		Where("user_id = ? AND result = 'passed'", userID).
		Distinct("activity_id").
		Count(&cnt).Error
	return int(cnt), err
}

// ListPendingBadges 返回用户已获得但「尚未庆祝」(celebrated=0) 的勋章，供全局弹窗逐枚展示。
// 「授予」与「庆祝」解耦：授予即写入 user_badges(celebrated=0)，仅当前端展示并回执后才置 1。
// 因此弹窗在途丢失 / 刷新 / 崩溃都不会漏掉——下次结算仍会返回，直到真正展示过为止。
func (r *GormRepo) ListPendingBadges(ctx context.Context, userID int64) ([]domain.Badge, error) {
	var rows []domain.Badge
	err := r.DB.WithContext(ctx).
		Table("badges").
		Select("badges.*").
		Joins("JOIN user_badges ON user_badges.badge_id = badges.id").
		Where("user_badges.user_id = ? AND user_badges.celebrated = 0", userID).
		Order("user_badges.earned_at ASC, badges.id ASC").
		Scan(&rows).Error
	return rows, err
}

// MarkCelebrated 把指定勋章置为「已庆祝」(celebrated=1)，前端弹窗展示后回执调用，之后不再返回。
func (r *GormRepo) MarkCelebrated(ctx context.Context, userID int64, badgeIDs []int64) error {
	if len(badgeIDs) == 0 {
		return nil
	}
	return r.DB.WithContext(ctx).
		Table("user_badges").
		Where("user_id = ? AND badge_id IN ?", userID, badgeIDs).
		Update("celebrated", 1).Error
}
