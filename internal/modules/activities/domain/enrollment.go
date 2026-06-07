package domain

import "time"

type EnrollStatus string

const (
	EnrollEnrolled  EnrollStatus = "enrolled"
	EnrollCheckedIn EnrollStatus = "checked_in"
	EnrollAbsent    EnrollStatus = "absent"
)

// Enrollment 复用 001 迁移已建的 activity_enrollments 表（用户报名 / 签到关系）。
type Enrollment struct {
	ID         int64        `gorm:"primaryKey"`
	ActivityID int64        `gorm:"column:activity_id"`
	UserID     int64        `gorm:"column:user_id"`
	Status     EnrollStatus `gorm:"column:status"`
	CreatedAt  time.Time    `gorm:"column:created_at"`
}

func (Enrollment) TableName() string { return "activity_enrollments" }
