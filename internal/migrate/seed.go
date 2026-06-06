package migrate

import (
	"context"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

type Seeder struct {
	DB              *gorm.DB
	DefaultTenantID int64
	// DimensionsFile 价值观维度 YAML 配置文件路径，默认 "./configs/value_dimensions.yaml"
	DimensionsFile string
	// WelcomeBonus 每个 seed 用户的默认积分，写入首维度 customer_first。
	// 0 表示沿用旧 demo 随机分布；>0 则给每人精确这么多分。
	WelcomeBonus int
	// DemoData 为 true 时才生成 50 个演示用户与演示积分（仅本地演示，生产保持 false）。
	DemoData bool
}

func (s *Seeder) Run(ctx context.Context) error {
	if err := s.seedTenant(); err != nil {
		return err
	}
	if err := s.seedDimensions(); err != nil {
		return err
	}
	if err := s.seedDepartments(); err != nil {
		return err
	}
	if s.DemoData {
		if err := s.seedUsers(); err != nil {
			return err
		}
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
	if s.DemoData {
		if err := s.seedDemoPoints(); err != nil {
			return err
		}
	}
	return nil
}

type dimDef struct {
	Code      string  `yaml:"code"`
	Name      string  `yaml:"name"`
	Keywords  string  `yaml:"keywords"`
	Weight    float64 `yaml:"weight"`
	SortOrder int     `yaml:"sort_order"`
}

type dimFile struct {
	Dimensions []dimDef `yaml:"dimensions"`
}

// seedDimensions 从 YAML 读取 6 个默认价值观维度，幂等插入
func (s *Seeder) seedDimensions() error {
	path := s.DimensionsFile
	if path == "" {
		path = "./configs/value_dimensions.yaml"
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	var f dimFile
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	for _, d := range f.Dimensions {
		err := s.DB.Exec(`
			INSERT INTO value_dimensions (tenant_id, code, name, keywords, weight, sort_order, enabled)
			VALUES (?, ?, ?, ?, ?, ?, 1)
			ON DUPLICATE KEY UPDATE
				name = VALUES(name),
				keywords = VALUES(keywords),
				weight = VALUES(weight),
				sort_order = VALUES(sort_order),
				enabled = 1
		`, s.DefaultTenantID, d.Code, d.Name, d.Keywords, d.Weight, d.SortOrder).Error
		if err != nil {
			return fmt.Errorf("upsert dimension %s: %w", d.Code, err)
		}
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
	// 自愈：已是新版里程碑勋章（存在"初来乍到"）则跳过；否则清掉旧勋章后重建，
	// 便于从旧的 24 枚价值观勋章平滑迁移（无需 drop 整库）。
	var milestone int64
	s.DB.Raw(`SELECT COUNT(*) FROM badges WHERE tenant_id = ? AND name = ?`, s.DefaultTenantID, "初来乍到").Scan(&milestone)
	if milestone > 0 {
		return nil
	}
	if err := s.DB.Exec(`DELETE FROM user_badges WHERE badge_id IN (SELECT id FROM badges WHERE tenant_id = ?)`, s.DefaultTenantID).Error; err != nil {
		return err
	}
	if err := s.DB.Exec(`DELETE FROM badges WHERE tenant_id = ?`, s.DefaultTenantID).Error; err != nil {
		return err
	}
	// 10 枚「成长里程碑」勋章：起点 / 赚取线 / 消费线。
	// 均为全局勋章（dimension_id = 0）。icon_url 存 emblem 代码，前端按代码渲染拟物奖牌。
	badges := []struct {
		name   string // 四字成语称号
		desc   string // 解锁条件文案
		rarity string
		rule   string
		emblem string
	}{
		{"初来乍到", "完成第一次活动签到", "common", `{"type":"first_signin"}`, "sprout"},
		{"旗开得胜", "赚到第一笔积分", "common", `{"type":"earned_total","threshold":1}`, "flag"},
		{"积少成多", "累计赚取满 5 分", "common", `{"type":"earned_total","threshold":5}`, "coin_stack"},
		{"聚沙成塔", "累计赚取满 10 分", "rare", `{"type":"earned_total","threshold":10}`, "pagoda"},
		{"厚积薄发", "累计赚取满 20 分", "epic", `{"type":"earned_total","threshold":20}`, "burst"},
		{"富甲一方", "累计赚取满 50 分", "legendary", `{"type":"earned_total","threshold":50}`, "ingot"},
		{"小试牛刀", "累计消费满 5 分", "common", `{"type":"spent_total","threshold":5}`, "cleaver"},
		{"各取所需", "累计消费满 10 分", "rare", `{"type":"spent_total","threshold":10}`, "gift"},
		{"满载而归", "累计消费满 20 分", "epic", `{"type":"spent_total","threshold":20}`, "bag"},
		{"一掷千金", "累计消费满 50 分", "legendary", `{"type":"spent_total","threshold":50}`, "coins_toss"},
	}
	for _, b := range badges {
		err := s.DB.Exec(
			`INSERT INTO badges (tenant_id, dimension_id, name, description, rarity, rule_json, icon_url) VALUES (?, 0, ?, ?, ?, ?, ?)`,
			s.DefaultTenantID, b.name, b.desc, b.rarity, b.rule, b.emblem,
		).Error
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Seeder) seedMallItems() error {
	if err := s.syncDefaultBlindboxCosts(); err != nil {
		return err
	}
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
		{"blindbox", "AI 文化盲盒 · 普通", 5, nil},
		{"blindbox", "AI 文化盲盒 · 闪光", 10, nil},
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

func (s *Seeder) syncDefaultBlindboxCosts() error {
	updates := []struct {
		name string
		cost int
	}{
		{"AI 文化盲盒 · 普通", 5},
		{"AI 文化盲盒 · 闪光", 10},
	}
	for _, u := range updates {
		if err := s.DB.Exec(
			`UPDATE mall_items SET cost = ? WHERE tenant_id = ? AND type = 'blindbox' AND name = ?`,
			u.cost, s.DefaultTenantID, u.name,
		).Error; err != nil {
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
