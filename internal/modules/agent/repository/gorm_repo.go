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
	Summary    string    `gorm:"column:summary"` // 结束会话时由 AI 提炼的核心摘要
	Ended      bool      `gorm:"column:ended"`   // 已结束=从历史列表移除（归档），但摘要仍用于记忆
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
		Where("tenant_id = ? AND operator_id = ? AND ended = 0", tenantID, operatorID).
		Order("id DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

// MarkEnded 把会话标记为已结束（从历史列表归档移除）。
func (r *Repo) MarkEnded(ctx context.Context, id int64) error {
	return r.DB.WithContext(ctx).Model(&Session{}).Where("id = ?", id).Update("ended", 1).Error
}

// ListMemories 取该 HR 最近、有摘要的会话（含已结束的），用于跨会话记忆注入。
func (r *Repo) ListMemories(ctx context.Context, tenantID, operatorID int64, limit int) ([]Session, error) {
	if limit <= 0 {
		limit = 20
	}
	var rows []Session
	err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND operator_id = ? AND summary <> ''", tenantID, operatorID).
		Order("id DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

// SearchSessions 按标题/摘要模糊匹配会话（含已结束的），供「AI 智能搜索」跳到具体历史会话。
func (r *Repo) SearchSessions(ctx context.Context, tenantID, operatorID int64, q string, limit int) ([]Session, error) {
	if limit <= 0 {
		limit = 5
	}
	like := "%" + q + "%"
	var rows []Session
	err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND operator_id = ? AND (title LIKE ? OR summary LIKE ?)", tenantID, operatorID, like, like).
		Order("id DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

func (r *Repo) ListMessages(ctx context.Context, sessionID int64) ([]Message, error) {
	var rows []Message
	err := r.DB.WithContext(ctx).Where("session_id = ?", sessionID).Order("id ASC").Find(&rows).Error
	return rows, err
}

// OperatorAssistantMessages 取该 operator 名下各会话的 assistant 消息（用于统计高频操作做个性化推荐）。
func (r *Repo) OperatorAssistantMessages(ctx context.Context, tenantID, operatorID int64, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 400
	}
	var rows []Message
	err := r.DB.WithContext(ctx).
		Where("role = ? AND session_id IN (SELECT id FROM agent_sessions WHERE tenant_id = ? AND operator_id = ?)", "assistant", tenantID, operatorID).
		Order("id DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

func (r *Repo) GetSession(ctx context.Context, id int64) (*Session, error) {
	var s Session
	if err := r.DB.WithContext(ctx).Where("id = ?", id).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *Repo) UpdateSummary(ctx context.Context, id int64, summary string) error {
	return r.DB.WithContext(ctx).Model(&Session{}).Where("id = ?", id).Update("summary", summary).Error
}

func (r *Repo) UpdateTitle(ctx context.Context, id int64, title string) error {
	return r.DB.WithContext(ctx).Model(&Session{}).Where("id = ?", id).Update("title", title).Error
}
