package domain

import "time"

type Snapshot struct {
	ID            int64     `gorm:"column:id;primaryKey" json:"-"`
	PublicationID int64     `gorm:"column:publication_id" json:"-"`
	SectionID     int64     `gorm:"column:section_id" json:"sectionId"`
	DataJSON      string    `gorm:"column:data_json" json:"-"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"-"`
}

func (Snapshot) TableName() string { return "publication_snapshots" }
