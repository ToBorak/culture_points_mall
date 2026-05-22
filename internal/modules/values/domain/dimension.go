package domain

import "time"

type Dimension struct {
	ID        int64     `gorm:"primaryKey"`
	TenantID  int64     `gorm:"column:tenant_id"`
	Code      string    `gorm:"column:code"`
	Name      string    `gorm:"column:name"`
	Keywords  string    `gorm:"column:keywords"`
	Weight    float64   `gorm:"column:weight"`
	SortOrder int       `gorm:"column:sort_order"`
	Enabled   bool      `gorm:"column:enabled"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (Dimension) TableName() string { return "value_dimensions" }
