package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/standardsoftware/culture_points_mall/internal/modules/points/domain"
)

type GormRepo struct{ DB *gorm.DB }

func New(db *gorm.DB) *GormRepo { return &GormRepo{DB: db} }

func (r *GormRepo) InsertTransaction(ctx context.Context, tx *domain.Transaction) error {
	return r.DB.WithContext(ctx).Create(tx).Error
}

func (r *GormRepo) IncrementSnapshot(ctx context.Context, tenantID, userID, dimID int64, amount int) error {
	return r.DB.WithContext(ctx).Exec(`
		INSERT INTO user_dimension_scores (user_id, tenant_id, dimension_id, total_score, quarter_score, year_score)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			total_score = total_score + VALUES(total_score),
			quarter_score = quarter_score + VALUES(quarter_score),
			year_score = year_score + VALUES(year_score)
	`, userID, tenantID, dimID, amount, amount, amount).Error
}

func (r *GormRepo) GetSnapshotsByUser(ctx context.Context, tenantID, userID int64) ([]domain.DimensionScore, error) {
	var rows []domain.DimensionScore
	err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Find(&rows).Error
	return rows, err
}

func (r *GormRepo) ListTransactions(ctx context.Context, tenantID, userID int64, cursor int64, limit int) ([]domain.Transaction, error) {
	q := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Order("id DESC").
		Limit(limit)
	if cursor > 0 {
		q = q.Where("id < ?", cursor)
	}
	var rows []domain.Transaction
	err := q.Find(&rows).Error
	return rows, err
}

func (r *GormRepo) GetTotalScore(ctx context.Context, tenantID, userID int64) (int, error) {
	var total int64
	err := r.DB.WithContext(ctx).
		Table("user_dimension_scores").
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Select("COALESCE(SUM(total_score),0)").
		Scan(&total).Error
	return int(total), err
}

// 显式使用 clause 包以避免 import 修剪（保留扩展位）
var _ = clause.OnConflict{}
