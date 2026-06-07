package domain

import "time"

type SeasonStatus string

const (
	SeasonNominating SeasonStatus = "nominating"
	SeasonJudging    SeasonStatus = "judging"
	SeasonPublished  SeasonStatus = "published"
	SeasonClosed     SeasonStatus = "closed"
)

type Season struct {
	ID              int64        `gorm:"column:id;primaryKey"`
	TenantID        int64        `gorm:"column:tenant_id"`
	Name            string       `gorm:"column:name"`
	QuarterCode     string       `gorm:"column:quarter_code"`
	Status          SeasonStatus `gorm:"column:status"`
	NominateStartAt *time.Time   `gorm:"column:nominate_start_at"`
	NominateEndAt   *time.Time   `gorm:"column:nominate_end_at"`
	CreatedAt       time.Time    `gorm:"column:created_at"`
	UpdatedAt       time.Time    `gorm:"column:updated_at"`
}

func (Season) TableName() string { return "star_seasons" }
