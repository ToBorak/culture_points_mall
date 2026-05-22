package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/standardsoftware/culture_points_mall/internal/modules/mall/domain"
)

type GormRepo struct{ DB *gorm.DB }

func New(db *gorm.DB) *GormRepo { return &GormRepo{DB: db} }

func (r *GormRepo) ListItems(ctx context.Context, tenantID int64, typ string) ([]domain.Item, error) {
	q := r.DB.WithContext(ctx).Where("tenant_id = ?", tenantID)
	if typ != "" {
		q = q.Where("type = ?", typ)
	}
	var rows []domain.Item
	err := q.Find(&rows).Error
	return rows, err
}

func (r *GormRepo) GetItem(ctx context.Context, tenantID, id int64) (*domain.Item, error) {
	var it domain.Item
	err := r.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&it).Error
	if err != nil {
		return nil, err
	}
	return &it, nil
}

func (r *GormRepo) ListPrizes(ctx context.Context, boxID int64) ([]domain.BlindboxPrize, error) {
	var rows []domain.BlindboxPrize
	err := r.DB.WithContext(ctx).Where("box_item_id = ?", boxID).Find(&rows).Error
	return rows, err
}

func (r *GormRepo) CreateFreeze(ctx context.Context, f *domain.Freeze) error {
	return r.DB.WithContext(ctx).Create(f).Error
}

func (r *GormRepo) MarkConfirmed(ctx context.Context, txID string) error {
	return r.DB.WithContext(ctx).Model(&domain.Freeze{}).
		Where("tx_id = ? AND status = 'try'", txID).
		Update("status", domain.FreezeConfirmed).Error
}

func (r *GormRepo) MarkCancelled(ctx context.Context, txID string) error {
	return r.DB.WithContext(ctx).Model(&domain.Freeze{}).
		Where("tx_id = ? AND status = 'try'", txID).
		Update("status", domain.FreezeCancelled).Error
}

func (r *GormRepo) ListExpiredFreeze(ctx context.Context, now time.Time, limit int) ([]domain.Freeze, error) {
	var rows []domain.Freeze
	err := r.DB.WithContext(ctx).
		Where("status = 'try' AND expires_at < ?", now).
		Limit(limit).Find(&rows).Error
	return rows, err
}

func (r *GormRepo) CreateOrder(ctx context.Context, o *domain.Order) error {
	return r.DB.WithContext(ctx).Create(o).Error
}
