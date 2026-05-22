package domain

import (
	"context"
	"time"
)

type User struct {
	ID         int64     `gorm:"primaryKey"`
	TenantID   int64     `gorm:"column:tenant_id"`
	DingUserID string    `gorm:"column:ding_user_id"`
	Name       string    `gorm:"column:name"`
	AvatarURL  string    `gorm:"column:avatar_url"`
	DeptID     *int64    `gorm:"column:dept_id"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

func (User) TableName() string { return "users" }

type Repository interface {
	GetByID(ctx context.Context, tenantID, id int64) (*User, error)
	ListByDept(ctx context.Context, tenantID, deptID int64) ([]User, error)
}
