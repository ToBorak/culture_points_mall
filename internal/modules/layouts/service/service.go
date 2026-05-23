package service

import (
	"context"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
)

// Module H5 首页可编排模块
type Module struct {
	ID      string `json:"id"`      // 标识，前端按 id 渲染对应组件
	Visible bool   `json:"visible"` // 是否显示
}

// Layout 单租户的 H5 首页布局配置
type Layout struct {
	Modules []Module `json:"modules"`
}

// DefaultLayout 出厂默认布局（顺序 = 前端 HomePage 默认顺序）
func DefaultLayout() Layout {
	return Layout{Modules: []Module{
		{ID: "hero", Visible: true},
		{ID: "icons", Visible: true},
		{ID: "challenge", Visible: true},
		{ID: "coach", Visible: true},
		{ID: "dna_entry", Visible: true},
		{ID: "promo_blindbox", Visible: true},
		{ID: "bento", Visible: true},
	}}
}

// AvailableModules 模块元信息（给 admin 编排面板用）
type ModuleMeta struct {
	ID          string `json:"id"`
	Name        string `json:"name"`        // 中文标题
	Description string `json:"description"` // 描述
	Icon        string `json:"icon"`        // emoji/字符 图标
	Tint        string `json:"tint"`        // 主题色（编排卡片用）
	PreviewKind string `json:"previewKind"` // hero / list / banner / grid / chip
}

func AvailableModules() []ModuleMeta {
	return []ModuleMeta{
		{ID: "hero", Name: "个人 Hero 卡", Description: "头像 / 等级 / 总积分 / 6 维度色条", Icon: "✦", Tint: "#7c3aed", PreviewKind: "hero"},
		{ID: "icons", Name: "图标入口网格", Description: "3×2 文化护照 / 排行榜 / 商城等", Icon: "⊞", Tint: "#0891b2", PreviewKind: "grid"},
		{ID: "challenge", Name: "今日 AI 挑战", Description: "AI 生成的 5 分钟文化任务，员工提交后自动加分", Icon: "◇", Tint: "#10b981", PreviewKind: "list"},
		{ID: "coach", Name: "AI 成长教练", Description: "弱势维度聚焦 + 3 个 action items + 预期收益", Icon: "⚡", Tint: "#f59e0b", PreviewKind: "list"},
		{ID: "dna_entry", Name: "文化 DNA 入口", Description: "深紫粉渐变 banner，点击进入沉浸式季报", Icon: "🧬", Tint: "#ec4899", PreviewKind: "banner"},
		{ID: "promo_blindbox", Name: "盲盒推荐 banner", Description: "周末特惠 / 限时盲盒卡", Icon: "◈", Tint: "#e11d48", PreviewKind: "banner"},
		{ID: "bento", Name: "数据 Bento", Description: "我的排名 + 下场活动两列", Icon: "▣", Tint: "#0f172a", PreviewKind: "grid"},
	}
}

type Service struct {
	DB *gorm.DB
}

func New(db *gorm.DB) *Service { return &Service{DB: db} }

// Get 读取某租户的布局（兜底返回默认布局）
func (s *Service) Get(ctx context.Context, tenantID int64) (Layout, error) {
	var raw []byte
	row := s.DB.WithContext(ctx).Raw(
		`SELECT config_json FROM tenants WHERE id = ?`, tenantID,
	).Row()
	if err := row.Scan(&raw); err != nil {
		// 找不到租户也返回默认布局，避免 H5 直接报错
		return DefaultLayout(), nil
	}
	if len(raw) == 0 {
		return DefaultLayout(), nil
	}
	var cfg struct {
		Layout *Layout `json:"layout,omitempty"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return DefaultLayout(), nil
	}
	if cfg.Layout == nil || len(cfg.Layout.Modules) == 0 {
		return DefaultLayout(), nil
	}
	// 用持久化的顺序 + 默认补全（新增模块出现时也能展示）
	merged := mergeWithDefault(*cfg.Layout)
	return merged, nil
}

// Save 写入布局
func (s *Service) Save(ctx context.Context, tenantID int64, layout Layout) error {
	if err := validate(layout); err != nil {
		return err
	}
	// 用 JSON_SET 局部更新 config_json.layout，保留其它字段
	payload, _ := json.Marshal(layout)
	res := s.DB.WithContext(ctx).Exec(
		`UPDATE tenants SET config_json = JSON_SET(COALESCE(config_json, JSON_OBJECT()), '$.layout', CAST(? AS JSON)) WHERE id = ?`,
		string(payload), tenantID,
	)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("tenant %d not found", tenantID)
	}
	return nil
}

// mergeWithDefault 用配置的顺序 + 把默认存在但配置缺失的模块附加到末尾（保留 visible=true）
func mergeWithDefault(cfg Layout) Layout {
	def := DefaultLayout()
	configured := map[string]bool{}
	for _, m := range cfg.Modules {
		configured[m.ID] = true
	}
	out := Layout{Modules: append([]Module{}, cfg.Modules...)}
	for _, m := range def.Modules {
		if !configured[m.ID] {
			out.Modules = append(out.Modules, m)
		}
	}
	return out
}

func validate(layout Layout) error {
	if len(layout.Modules) == 0 {
		return fmt.Errorf("layout.modules cannot be empty")
	}
	seen := map[string]bool{}
	allowed := map[string]bool{}
	for _, m := range AvailableModules() {
		allowed[m.ID] = true
	}
	for _, m := range layout.Modules {
		if !allowed[m.ID] {
			return fmt.Errorf("unknown module id: %s", m.ID)
		}
		if seen[m.ID] {
			return fmt.Errorf("duplicate module id: %s", m.ID)
		}
		seen[m.ID] = true
	}
	return nil
}
