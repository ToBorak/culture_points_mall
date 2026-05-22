package domain

import "time"

type Transaction struct {
	ID          int64     `gorm:"primaryKey"`
	TenantID    int64     `gorm:"column:tenant_id"`
	UserID      int64     `gorm:"column:user_id"`
	DimensionID int64     `gorm:"column:dimension_id"`
	Amount      int       `gorm:"column:amount"`
	ActivityID  *int64    `gorm:"column:activity_id"`
	Reason      string    `gorm:"column:reason"`
	OperatorID  *int64    `gorm:"column:operator_id"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

func (Transaction) TableName() string { return "point_transactions" }
