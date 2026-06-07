package domain

import "time"

type Winner struct {
	ID                 int64     `gorm:"column:id;primaryKey"`
	TenantID           int64     `gorm:"column:tenant_id"`
	SeasonID           int64     `gorm:"column:season_id"`
	UserID             int64     `gorm:"column:user_id"`
	DimensionID        int64     `gorm:"column:dimension_id"`
	Citation           *string   `gorm:"column:citation"`
	SourceNominationID *int64    `gorm:"column:source_nomination_id"`
	CreatedAt          time.Time `gorm:"column:created_at"`
}

func (Winner) TableName() string { return "star_winners" }
