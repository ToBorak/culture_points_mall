package domain

import "time"

type SectionType string

const (
	SecEditorial   SectionType = "editorial"
	SecStar        SectionType = "star"
	SecValues      SectionType = "values"
	SecHonors      SectionType = "honors"
	SecLottery     SectionType = "lottery"
	SecActivity    SectionType = "activity"
	SecLeaderboard SectionType = "leaderboard"
	SecInnovation  SectionType = "innovation"
	SecCustom      SectionType = "custom"
)

// SnapshotBacked 标记需要聚合快照的栏目类型（其余为成稿/文章类）。
func (t SectionType) SnapshotBacked() bool {
	switch t {
	case SecStar, SecValues, SecHonors, SecLottery, SecActivity, SecLeaderboard:
		return true
	}
	return false
}

type Section struct {
	ID            int64       `gorm:"column:id;primaryKey" json:"id"`
	PublicationID int64       `gorm:"column:publication_id" json:"publicationId"`
	Type          SectionType `gorm:"column:type" json:"type"`
	Title         string      `gorm:"column:title" json:"title"`
	SortOrder     int         `gorm:"column:sort_order" json:"sortOrder"`
	Visible       bool        `gorm:"column:visible" json:"visible"`
	AICopy        *string     `gorm:"column:ai_copy" json:"aiCopy,omitempty"`
	ConfigJSON    *string     `gorm:"column:config_json" json:"-"`
	CreatedAt     time.Time   `gorm:"column:created_at" json:"-"`
}

func (Section) TableName() string { return "publication_sections" }
