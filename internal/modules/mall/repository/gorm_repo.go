package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/standardsoftware/culture_points_mall/internal/modules/mall/domain"
)

type GormRepo struct{ DB *gorm.DB }

func New(db *gorm.DB) *GormRepo { return &GormRepo{DB: db} }

func (r *GormRepo) ListItems(ctx context.Context, tenantID int64, typ string, onlyActive bool) ([]domain.Item, error) {
	q := r.DB.WithContext(ctx).Where("tenant_id = ?", tenantID)
	if typ != "" {
		q = q.Where("type = ?", typ)
	}
	if onlyActive {
		q = q.Where("status = ?", domain.StatusOnShelf)
	}
	var rows []domain.Item
	err := q.Order("id DESC").Find(&rows).Error
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

func (r *GormRepo) CreateItem(ctx context.Context, it *domain.Item) error {
	return r.DB.WithContext(ctx).Create(it).Error
}

// UpdateItemFields 按 map 局部更新商品字段（name/cost/stock/image_url/status），便于改库存/改名/改积分/上下架。
func (r *GormRepo) UpdateItemFields(ctx context.Context, tenantID, id int64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	return r.DB.WithContext(ctx).Model(&domain.Item{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).Updates(fields).Error
}

// DeleteItem 硬删除商品（用于「撤销新增商品」的回撤）。
func (r *GormRepo) DeleteItem(ctx context.Context, tenantID, id int64) error {
	return r.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).Delete(&domain.Item{}).Error
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

// OrderView 我的订单展示视图（关联商品名/奖品名）
type OrderView struct {
	ID        int64  `json:"id"`
	ItemID    *int64 `json:"itemId"`
	ItemName  string `json:"itemName"`
	PrizeID   *int64 `json:"prizeId"`
	PrizeName string `json:"prizeName"`
	Cost      int    `json:"cost"`
	Status    string `json:"status"`
}

func (r *GormRepo) ListOrdersByUser(ctx context.Context, tenantID, userID int64) ([]OrderView, error) {
	var rows []OrderView
	err := r.DB.WithContext(ctx).Raw(`
		SELECT o.id, o.item_id, COALESCE(i.name,'') AS item_name,
		       o.prize_id, COALESCE(p.prize_name,'') AS prize_name,
		       o.cost, o.status
		FROM mall_orders o
		LEFT JOIN mall_items i ON i.id = o.item_id
		LEFT JOIN mall_blindbox_pool p ON p.id = o.prize_id
		WHERE o.tenant_id = ? AND o.user_id = ?
		ORDER BY o.id DESC
		LIMIT 100
	`, tenantID, userID).Scan(&rows).Error
	return rows, err
}

// DecrementStock 库存非空时减 1（库存为 NULL 表示不限量）
func (r *GormRepo) DecrementStock(ctx context.Context, itemID int64) error {
	return r.DB.WithContext(ctx).Exec(
		`UPDATE mall_items SET stock = stock - 1 WHERE id = ? AND stock IS NOT NULL AND stock > 0`, itemID,
	).Error
}
