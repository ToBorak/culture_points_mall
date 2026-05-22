package repository

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

type Session struct {
	ID         int64     `gorm:"primaryKey"`
	TenantID   int64     `gorm:"column:tenant_id"`
	OperatorID int64     `gorm:"column:operator_id"`
	Title      string    `gorm:"column:title"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

func (Session) TableName() string { return "agent_sessions" }

type Message struct {
	ID        int64           `gorm:"primaryKey"`
	SessionID int64           `gorm:"column:session_id"`
	Role      string          `gorm:"column:role"`
	Content   json.RawMessage `gorm:"column:content"`
	CreatedAt time.Time       `gorm:"column:created_at"`
}

func (Message) TableName() string { return "agent_messages" }

type Repo struct{ DB *gorm.DB }

func New(db *gorm.DB) *Repo { return &Repo{DB: db} }

func (r *Repo) CreateSession(ctx context.Context, s *Session) error {
	return r.DB.WithContext(ctx).Create(s).Error
}

func (r *Repo) AppendMessage(ctx context.Context, m *Message) error {
	return r.DB.WithContext(ctx).Create(m).Error
}

func (r *Repo) ListSessions(ctx context.Context, tenantID, operatorID int64, limit int) ([]Session, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var rows []Session
	err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND operator_id = ?", tenantID, operatorID).
		Order("id DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

func (r *Repo) ListMessages(ctx context.Context, sessionID int64) ([]Message, error) {
	var rows []Message
	err := r.DB.WithContext(ctx).Where("session_id = ?", sessionID).Order("id ASC").Find(&rows).Error
	return rows, err
}
