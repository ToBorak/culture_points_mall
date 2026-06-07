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
	Description string          `gorm:"column:description"`
	Rarity      Rarity          `gorm:"column:rarity"`
	RuleJSON    json.RawMessage `gorm:"column:rule_json"`
	IconURL     string          `gorm:"column:icon_url"` // 里程碑勋章中存 emblem 代码（如 "sprout"），非图片 URL
}

func (Badge) TableName() string { return "badges" }

type UserBadge struct {
	UserID     int64  `gorm:"primaryKey;column:user_id"`
	BadgeID    int64  `gorm:"primaryKey;column:badge_id"`
	Celebrated bool   `gorm:"column:celebrated"`
	EarnedAt   string `gorm:"column:earned_at"`
}

func (UserBadge) TableName() string { return "user_badges" }

// Rule 勋章解锁规则。Type 取值：
//
//	accumulated  价值观维度累计积分 ≥ Threshold（Dimension 为维度 code，旧体系保留）
//	first_signin 完成过第一次活动签到
//	earned_total 累计赚取（排除欢迎积分）≥ Threshold
//	spent_total  累计消费 ≥ Threshold
type Rule struct {
	Type      string `json:"type"`
	Dimension string `json:"dimension"`
	Threshold int    `json:"threshold"`
}
