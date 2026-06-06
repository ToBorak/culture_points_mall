package domain

import "time"

type Status string

const (
	StatusPublished Status = "published"
	StatusPartial   Status = "partial"
	StatusFailed    Status = "failed"
)

type Schedule struct {
	ID              int64     `gorm:"primaryKey"`
	TenantID        int64     `gorm:"column:tenant_id"`
	Title           string    `gorm:"column:title"`
	StartAt         time.Time `gorm:"column:start_at"`
	EndAt           time.Time `gorm:"column:end_at"`
	Location        string    `gorm:"column:location"`
	Detail          string    `gorm:"column:detail"`
	AttendeeUserIDs []string  `gorm:"column:attendee_user_ids;serializer:json"`
	GroupIDs        []string  `gorm:"column:group_ids;serializer:json"`
	PushCalendar    bool      `gorm:"column:push_calendar"`
	PushGroup       bool      `gorm:"column:push_group"`
	Status          Status    `gorm:"column:status"`
	CalendarEventID string    `gorm:"column:calendar_event_id"`
	ResultNote      string    `gorm:"column:result_note"`
	CreatedBy       int64     `gorm:"column:created_by"`
	CreatedAt       time.Time `gorm:"column:created_at"`
}

func (Schedule) TableName() string { return "schedules" }
