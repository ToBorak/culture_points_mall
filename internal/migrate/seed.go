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
	// WelcomeBonus 每个 seed 用户的默认积分，写入首维度 customer_first。
	// 0 表示沿用旧 demo 随机分布；>0 则给每人精确这么多分。
	WelcomeBonus int
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
	if err := s.seedDemoPoints(); err != nil {
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

func (s *Seeder) seedDemoPoints() error {
	var existing int64
	s.DB.Raw("SELECT COUNT(*) FROM point_transactions WHERE tenant_id = ?", s.DefaultTenantID).Scan(&existing)
	if existing > 0 {
		return nil
	}

	bonus := s.WelcomeBonus
	if bonus <= 0 {
		bonus = 100000 // 默认每个 seed 用户起始 100,000 分
	}

	// 第一步：给每个用户发放欢迎积分（统一到 customer_first 维度 = id 1，与运行时 welcomeGranter 一致）
	for uid := int64(1); uid <= 50; uid++ {
		if err := s.grantWelcome(uid, bonus); err != nil {
			return err
		}
	}

	// 第二步：在其它维度撒一些演示分数，让排行榜/雷达有变化
	for uid := int64(1); uid <= 50; uid++ {
		n := 3 + (uid % 6)
		for i := int64(0); i < n; i++ {
			dimID := (i % 6) + 1
			if dimID == 1 {
				continue // customer_first 已经发过欢迎积分
			}
			amt := 10 + int(uid+i)*3
			_ = s.DB.Exec(
				`INSERT INTO point_transactions (tenant_id, user_id, dimension_id, amount, reason) VALUES (?, ?, ?, ?, ?)`,
				s.DefaultTenantID, uid, dimID, amt, "演示加分",
			).Error
			_ = s.DB.Exec(`
				INSERT INTO user_dimension_scores (user_id, tenant_id, dimension_id, total_score, quarter_score, year_score)
				VALUES (?, ?, ?, ?, ?, ?)
				ON DUPLICATE KEY UPDATE
					total_score = total_score + VALUES(total_score),
					quarter_score = quarter_score + VALUES(quarter_score),
					year_score = year_score + VALUES(year_score)
			`, uid, s.DefaultTenantID, dimID, amt, amt, amt).Error
		}
	}
	return nil
}

// grantWelcome 写一条 100,000 欢迎积分流水 + 维度汇总，全部计入 customer_first（dim_id=1）
func (s *Seeder) grantWelcome(uid int64, amount int) error {
	if err := s.DB.Exec(
		`INSERT INTO point_transactions (tenant_id, user_id, dimension_id, amount, reason) VALUES (?, ?, 1, ?, '新员工欢迎积分')`,
		s.DefaultTenantID, uid, amount,
	).Error; err != nil {
		return err
	}
	return s.DB.Exec(`
		INSERT INTO user_dimension_scores (user_id, tenant_id, dimension_id, total_score, quarter_score, year_score)
		VALUES (?, ?, 1, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			total_score = total_score + VALUES(total_score),
			quarter_score = quarter_score + VALUES(quarter_score),
			year_score = year_score + VALUES(year_score)
	`, uid, s.DefaultTenantID, amount, amount, amount).Error
}

func (s *Seeder) count(table string, tenantID int64) int64 {
	var c int64
	s.DB.Raw(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE tenant_id = ?", table), tenantID).Scan(&c)
	return c
}

func intPtr(v int) *int { return &v }
