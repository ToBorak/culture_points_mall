package domain

import "encoding/json"

type Rarity string

const (
	RarityCommon    Rarity = "common"
	RarityRare      Rarity = "rare"
	RarityEpic      Rarity = "epic"
	RarityLegendary Rarity = "legendary"
)

type Badge struct {
	ID          int64           `gorm:"primaryKey"`
	TenantID    int64           `gorm:"column:tenant_id"`
	DimensionID int64           `gorm:"column:dimension_id"`
	Name        string          `gorm:"column:name"`
	Rarity      Rarity          `gorm:"column:rarity"`
	RuleJSON    json.RawMessage `gorm:"column:rule_json"`
	IconURL     string          `gorm:"column:icon_url"`
}

func (Badge) TableName() string { return "badges" }

type UserBadge struct {
	UserID   int64  `gorm:"primaryKey;column:user_id"`
	BadgeID  int64  `gorm:"primaryKey;column:badge_id"`
	EarnedAt string `gorm:"column:earned_at"`
}

func (UserBadge) TableName() string { return "user_badges" }

type Rule struct {
	Type      string `json:"type"`
	Dimension string `json:"dimension"`
	Threshold int    `json:"threshold"`
}
