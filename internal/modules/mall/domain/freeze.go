package domain

import "time"

type FreezeStatus string

const (
	FreezeTry       FreezeStatus = "try"
	FreezeConfirmed FreezeStatus = "confirmed"
	FreezeCancelled FreezeStatus = "cancelled"
)

type Freeze struct {
	ID        int64        `gorm:"primaryKey"`
	TxID      string       `gorm:"column:tx_id"`
	UserID    int64        `gorm:"column:user_id"`
	BoxItemID int64        `gorm:"column:box_item_id"`
	Amount    int          `gorm:"column:amount"`
	Status    FreezeStatus `gorm:"column:status"`
	ExpiresAt time.Time    `gorm:"column:expires_at"`
	CreatedAt time.Time    `gorm:"column:created_at"`
}

func (Freeze) TableName() string { return "mall_blindbox_freeze" }

type Order struct {
	ID       int64  `gorm:"primaryKey"`
	TenantID int64  `gorm:"column:tenant_id"`
	UserID   int64  `gorm:"column:user_id"`
	ItemID   *int64 `gorm:"column:item_id"`
	PrizeID  *int64 `gorm:"column:prize_id"`
	Cost     int    `gorm:"column:cost"`
	Status   string `gorm:"column:status"`
}

func (Order) TableName() string { return "mall_orders" }
