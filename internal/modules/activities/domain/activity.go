package domain

import "time"

type Status string

const (
	StatusDraft     Status = "draft"
	StatusPublished Status = "published"
	StatusRunning   Status = "running"
	StatusClosed    Status = "closed"
)

type Activity struct {
	ID           int64      `gorm:"primaryKey"`
	TenantID     int64      `gorm:"column:tenant_id"`
	DimensionID  int64      `gorm:"column:dimension_id"`
	Title        string     `gorm:"column:title"`
	Status       Status     `gorm:"column:status"`
	Capacity     *int       `gorm:"column:capacity"`
	StartAt      *time.Time `gorm:"column:start_at"`
	EndAt        *time.Time `gorm:"column:end_at"`
	PointsReward int        `gorm:"column:points_reward"`
	CreatedAt    time.Time  `gorm:"column:created_at"`
}

func (Activity) TableName() string { return "activities" }
