package domain

import "time"

type DimensionScore struct {
	UserID       int64     `gorm:"primaryKey;column:user_id"`
	TenantID     int64     `gorm:"column:tenant_id"`
	DimensionID  int64     `gorm:"primaryKey;column:dimension_id"`
	TotalScore   int       `gorm:"column:total_score"`
	QuarterScore int       `gorm:"column:quarter_score"`
	YearScore    int       `gorm:"column:year_score"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

func (DimensionScore) TableName() string { return "user_dimension_scores" }
