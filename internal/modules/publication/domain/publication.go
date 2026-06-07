package domain

import "time"

type PublicationStatus string

const (
	PubDraft     PublicationStatus = "draft"
	PubPublished PublicationStatus = "published"
	PubArchived  PublicationStatus = "archived"
)

type Publication struct {
	ID            int64             `gorm:"column:id;primaryKey" json:"id"`
	TenantID      int64             `gorm:"column:tenant_id" json:"-"`
	SeasonID      *int64            `gorm:"column:season_id" json:"seasonId,omitempty"`
	Title         string            `gorm:"column:title" json:"title"`
	PeriodCode    string            `gorm:"column:period_code" json:"periodCode"`
	CoverImageURL *string           `gorm:"column:cover_image_url" json:"coverImageUrl,omitempty"`
	IntroText     *string           `gorm:"column:intro_text" json:"introText,omitempty"`
	PeriodStart   *time.Time        `gorm:"column:period_start" json:"periodStart,omitempty"`
	PeriodEnd     *time.Time        `gorm:"column:period_end" json:"periodEnd,omitempty"`
	Status        PublicationStatus `gorm:"column:status" json:"status"`
	PublishedAt   *time.Time        `gorm:"column:published_at" json:"publishedAt,omitempty"`
	CreatedAt     time.Time         `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt     time.Time         `gorm:"column:updated_at" json:"-"`
}

func (Publication) TableName() string { return "publications" }
