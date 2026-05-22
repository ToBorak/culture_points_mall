package migrate

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

type Seeder struct {
	DB              *gorm.DB
	DefaultTenantID int64
	DimensionsFile  string
}

func (s *Seeder) Run(ctx context.Context) error {
	if err := s.seedTenant(); err != nil {
		return err
	}
	if err := s.seedDepartments(); err != nil {
		return err
	}
	if err := s.seedUsers(); err != nil {
		return err
	}
	if err := s.seedBadges(); err != nil {
		return err
	}
	if err := s.seedMallItems(); err != nil {
		return err
	}
	if err := s.seedBlindboxPool(); err != nil {
		return err
	}
	return nil
}

func (s *Seeder) seedTenant() error {
	return s.DB.Exec(`INSERT IGNORE INTO tenants (id, name) VALUES (?, ?)`, s.DefaultTenantID, "示范企业").Error
}

func (s *Seeder) seedDepartments() error {
	if cnt := s.count("departments", s.DefaultTenantID); cnt >= 3 {
		return nil
	}
	names := []string{"销售部", "研发部", "客服部"}
	for _, n := range names {
		if err := s.DB.Exec(`INSERT INTO departments (tenant_id, name) VALUES (?, ?)`, s.DefaultTenantID, n).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Seeder) seedUsers() error {
	if cnt := s.count("users", s.DefaultTenantID); cnt >= 50 {
		return nil
	}
	for i := 1; i <= 50; i++ {
		dept := ((i - 1) % 3) + 1
		err := s.DB.Exec(
			`INSERT INTO users (tenant_id, ding_user_id, name, avatar_url, dept_id) VALUES (?, ?, ?, ?, ?)`,
			s.DefaultTenantID,
			fmt.Sprintf("u%03d", i),
			fmt.Sprintf("员工%02d", i),
			fmt.Sprintf("https://api.dicebear.com/9.x/notionists/svg?seed=user-%03d", i),
			dept,
		).Error
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Seeder) seedBadges() error {
	if cnt := s.count("badges", s.DefaultTenantID); cnt > 0 {
		return nil
	}
	codes := []string{"customer_first", "team_collab", "innovation", "integrity", "craftsmanship", "growth"}
	tiers := []struct {
		name   string
		rarity string
		thresh int
	}{
		{"微光", "common", 10},
		{"进阶", "rare", 50},
		{"卓越", "epic", 150},
		{"传奇", "legendary", 300},
	}
	for _, code := range codes {
		var dimID int64
		s.DB.Raw(`SELECT id FROM value_dimensions WHERE tenant_id = ? AND code = ?`, s.DefaultTenantID, code).Scan(&dimID)
		if dimID == 0 {
			continue
		}
		for _, t := range tiers {
			rule := fmt.Sprintf(`{"type":"accumulated","dimension":"%s","threshold":%d}`, code, t.thresh)
			err := s.DB.Exec(
				`INSERT INTO badges (tenant_id, dimension_id, name, rarity, rule_json, icon_url) VALUES (?, ?, ?, ?, ?, ?)`,
				s.DefaultTenantID, dimID,
				fmt.Sprintf("%s · %s", code, t.name),
				t.rarity, rule,
				fmt.Sprintf("https://api.dicebear.com/9.x/shapes/svg?seed=badge-%s-%s", code, t.rarity),
			).Error
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Seeder) seedMallItems() error {
	if cnt := s.count("mall_items", s.DefaultTenantID); cnt > 0 {
		return nil
	}
	rows := []struct {
		typ   string
		name  string
		cost  int
		stock *int
	}{
		{"item", "周边帆布袋", 50, intPtr(100)},
		{"item", "公司定制 T 恤", 120, intPtr(50)},
		{"item", "咖啡券", 30, intPtr(200)},
		{"blindbox", "AI 文化盲盒 · 普通", 80, nil},
		{"blindbox", "AI 文化盲盒 · 闪光", 200, nil},
	}
	for _, r := range rows {
		var stock any = nil
		if r.stock != nil {
			stock = *r.stock
		}
		err := s.DB.Exec(
			`INSERT INTO mall_items (tenant_id, type, name, cost, stock, image_url) VALUES (?, ?, ?, ?, ?, ?)`,
			s.DefaultTenantID, r.typ, r.name, r.cost, stock,
			fmt.Sprintf("https://api.dicebear.com/9.x/shapes/svg?seed=item-%s", r.name),
		).Error
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Seeder) seedBlindboxPool() error {
	var boxIDs []int64
	s.DB.Raw(`SELECT id FROM mall_items WHERE tenant_id = ? AND type = 'blindbox'`, s.DefaultTenantID).Scan(&boxIDs)
	for _, boxID := range boxIDs {
		var cnt int64
		s.DB.Raw(`SELECT COUNT(*) FROM mall_blindbox_pool WHERE box_item_id = ?`, boxID).Scan(&cnt)
		if cnt > 0 {
			continue
		}
		prizes := []struct {
			name   string
			weight int
		}{
			{"无中奖（鼓励气泡）", 60},
			{"咖啡券", 25},
			{"帆布袋", 10},
			{"公司定制 T 恤", 5},
		}
		for _, p := range prizes {
			err := s.DB.Exec(
				`INSERT INTO mall_blindbox_pool (box_item_id, prize_name, prize_image, weight) VALUES (?, ?, ?, ?)`,
				boxID, p.name, fmt.Sprintf("https://api.dicebear.com/9.x/shapes/svg?seed=prize-%s", p.name), p.weight,
			).Error
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Seeder) count(table string, tenantID int64) int64 {
	var c int64
	s.DB.Raw(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE tenant_id = ?", table), tenantID).Scan(&c)
	return c
}

func intPtr(v int) *int { return &v }
