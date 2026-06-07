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
	ID              int64        `gorm:"column:id;primaryKey" json:"id"`
	TenantID        int64        `gorm:"column:tenant_id" json:"-"`
	Name            string       `gorm:"column:name" json:"name"`
	QuarterCode     string       `gorm:"column:quarter_code" json:"quarterCode"`
	Status          SeasonStatus `gorm:"column:status" json:"status"`
	NominateStartAt *time.Time   `gorm:"column:nominate_start_at" json:"nominateStartAt,omitempty"`
	NominateEndAt   *time.Time   `gorm:"column:nominate_end_at" json:"nominateEndAt,omitempty"`
	CreatedAt       time.Time    `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt       time.Time    `gorm:"column:updated_at" json:"-"`
}

func (Season) TableName() string { return "star_seasons" }
