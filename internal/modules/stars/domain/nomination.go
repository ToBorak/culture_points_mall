package domain

import "time"

type NominationStatus string

const (
	NominationSubmitted   NominationStatus = "submitted"
	NominationDuplicate   NominationStatus = "duplicate"
	NominationShortlisted NominationStatus = "shortlisted"
	NominationSelected    NominationStatus = "selected"
	NominationRejected    NominationStatus = "rejected"
)

type Nomination struct {
	ID          int64            `gorm:"column:id;primaryKey"`
	TenantID    int64            `gorm:"column:tenant_id"`
	SeasonID    int64            `gorm:"column:season_id"`
	NominatorID int64            `gorm:"column:nominator_id"`
	NomineeID   int64            `gorm:"column:nominee_id"`
	DimensionID int64            `gorm:"column:dimension_id"`
	CaseText    string           `gorm:"column:case_text"`
	CaseRefined *string          `gorm:"column:case_refined"`
	AITags      *string          `gorm:"column:ai_tags"`
	Status      NominationStatus `gorm:"column:status"`
	Score       *float64         `gorm:"column:score"`
	CreatedAt   time.Time        `gorm:"column:created_at"`
	UpdatedAt   time.Time        `gorm:"column:updated_at"`
}

func (Nomination) TableName() string { return "star_nominations" }
