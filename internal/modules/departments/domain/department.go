package domain

import "context"

type Department struct {
	ID       int64  `gorm:"primaryKey"`
	TenantID int64  `gorm:"column:tenant_id"`
	Name     string `gorm:"column:name"`
}

func (Department) TableName() string { return "departments" }

type Repository interface {
	List(ctx context.Context, tenantID int64) ([]Department, error)
	GetByID(ctx context.Context, tenantID, id int64) (*Department, error)
}
