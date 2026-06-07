package domain

import "time"

type SigninCode struct {
	ID         int64     `gorm:"primaryKey"`
	ActivityID int64     `gorm:"column:activity_id"`
	Code       string    `gorm:"column:code"`
	IssuedAt   time.Time `gorm:"column:issued_at"`
	ExpiresAt  time.Time `gorm:"column:expires_at"`
}

func (SigninCode) TableName() string { return "signin_codes" }

type SigninRecord struct {
	ID         int64     `gorm:"primaryKey"`
	ActivityID int64     `gorm:"column:activity_id"`
	UserID     int64     `gorm:"column:user_id"`
	Result     string    `gorm:"column:result"`
	Reason     string    `gorm:"column:reason"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

func (SigninRecord) TableName() string { return "signin_records" }
