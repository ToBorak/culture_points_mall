package domain

import "time"

type ArticleSource string

const (
	ArticleManual         ArticleSource = "manual"
	ArticleFromNomination ArticleSource = "from_nomination"
)

type ArticleStatus string

const (
	ArticleDraft     ArticleStatus = "draft"
	ArticlePublished ArticleStatus = "published"
)

type Article struct {
	ID               int64         `gorm:"column:id;primaryKey" json:"id"`
	TenantID         int64         `gorm:"column:tenant_id" json:"-"`
	PublicationID    *int64        `gorm:"column:publication_id" json:"publicationId,omitempty"`
	SectionID        *int64        `gorm:"column:section_id" json:"sectionId,omitempty"`
	Title            string        `gorm:"column:title" json:"title"`
	Summary          *string       `gorm:"column:summary" json:"summary,omitempty"`
	ContentHTML      string        `gorm:"column:content_html" json:"contentHtml"`
	CoverImageURL    *string       `gorm:"column:cover_image_url" json:"coverImageUrl,omitempty"`
	SourceType       ArticleSource `gorm:"column:source_type" json:"sourceType"`
	SourceID         *int64        `gorm:"column:source_id" json:"sourceId,omitempty"`
	ValueDimensionID *int64        `gorm:"column:value_dimension_id" json:"valueDimensionId,omitempty"`
	AuthorID         *int64        `gorm:"column:author_id" json:"authorId,omitempty"`
	Status           ArticleStatus `gorm:"column:status" json:"status"`
	PublishedAt      *time.Time    `gorm:"column:published_at" json:"publishedAt,omitempty"`
	CreatedAt        time.Time     `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt        time.Time     `gorm:"column:updated_at" json:"-"`
}

func (Article) TableName() string { return "publication_articles" }
