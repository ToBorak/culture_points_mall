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

// PrizeView 奖池展示/抽奖视图：关联「积分好物」取实时名称/图片/价值；
// ItemID 为 NULL 即「无奖品」行（名称取奖池快照 prize_name，如「谢谢参与」）。
type PrizeView struct {
	ID         int64  `json:"id"`
	ItemID     *int64 `json:"itemId"`
	PrizeName  string `json:"prizeName"`
	PrizeImage string `json:"prizeImage"`
	Weight     int    `json:"weight"`
	Stock      *int   `json:"stock"`
	Cost       int    `json:"cost"` // 关联好物的兑换积分（无奖品行为 0），仅供展示
}

// ListPrizeViews 列出某盲盒的奖池（含关联好物的实时名称/图片）。好物在前、无奖品在后。
func (r *GormRepo) ListPrizeViews(ctx context.Context, boxID int64) ([]PrizeView, error) {
	var rows []PrizeView
	err := r.DB.WithContext(ctx).Raw(`
		SELECT p.id,
		       p.item_id AS item_id,
		       COALESCE(i.name, p.prize_name) AS prize_name,
		       COALESCE(NULLIF(i.image_url, ''), p.prize_image) AS prize_image,
		       p.weight,
		       p.stock,
		       COALESCE(i.cost, 0) AS cost
		FROM mall_blindbox_pool p
		LEFT JOIN mall_items i ON i.id = p.item_id
		WHERE p.box_item_id = ?
		ORDER BY (p.item_id IS NULL), p.id
	`, boxID).Scan(&rows).Error
	return rows, err
}

// DecrementPrizeStock 奖项份数非空时减 1（份数为 NULL 表示不限量）。中奖好物时调用。
func (r *GormRepo) DecrementPrizeStock(ctx context.Context, prizeID int64) error {
	return r.DB.WithContext(ctx).Exec(
		`UPDATE mall_blindbox_pool SET stock = stock - 1 WHERE id = ? AND stock IS NOT NULL AND stock > 0`, prizeID,
	).Error
}

// DeleteItem 删除商品/盲盒：连带删除其作为盲盒的奖池行（box_item_id）以及作为奖品被引用的行（item_id）。
func (r *GormRepo) DeleteItem(ctx context.Context, tenantID, id int64) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`DELETE FROM mall_blindbox_pool WHERE box_item_id = ? OR item_id = ?`, id, id).Error; err != nil {
			return err
		}
		return tx.Where("tenant_id = ? AND id = ?", tenantID, id).Delete(&domain.Item{}).Error
	})
}

// PrizeInput 整存盲盒奖池时的单条奖项配置。ItemID 为 nil 即「无奖品」行。
type PrizeInput struct {
	ItemID     *int64
	PrizeName  string
	PrizeImage string
	Weight     int
	Stock      *int
}

// ReplaceBoxConfig 整存某盲盒配置：更新 charge_on_miss、清空并重建奖池。
func (r *GormRepo) ReplaceBoxConfig(ctx context.Context, boxID int64, chargeOnMiss bool, prizes []PrizeInput) error {
	charge := 0
	if chargeOnMiss {
		charge = 1
	}
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`UPDATE mall_items SET charge_on_miss = ? WHERE id = ? AND type = 'blindbox'`, charge, boxID).Error; err != nil {
			return err
		}
		if err := tx.Exec(`DELETE FROM mall_blindbox_pool WHERE box_item_id = ?`, boxID).Error; err != nil {
			return err
		}
		for _, p := range prizes {
			if err := tx.Exec(
				`INSERT INTO mall_blindbox_pool (box_item_id, item_id, prize_name, prize_image, weight, stock) VALUES (?, ?, ?, ?, ?, ?)`,
				boxID, p.ItemID, p.PrizeName, p.PrizeImage, p.Weight, p.Stock,
			).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
