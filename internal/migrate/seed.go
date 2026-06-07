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
	// 12 枚「成长里程碑」勋章：起点 / 签到线 / 赚取线 / 消费线。
	// 均为全局勋章（dimension_id = 0）。icon_url 存 emblem 代码，前端按代码渲染拟物奖牌。
	badges := []struct {
		name   string // 四字成语称号
		desc   string // 解锁条件文案
		rarity string
		rule   string
		emblem string
	}{
		{"初来乍到", "完成第一次活动签到", "common", `{"type":"first_signin"}`, "sprout"},
		{"渐入佳境", "完成 5 次活动签到", "rare", `{"type":"signin_count","threshold":5}`, "calendar_check"},
		{"持之以恒", "完成 10 次活动签到", "epic", `{"type":"signin_count","threshold":10}`, "flame"},
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
	if cnt := s.count("mall_items", s.DefaultTenantID); cnt > 0 {
		return nil
	}
	// 积分好物（type=item）。图片走后端静态托管 /api/uploads/seed/*（种子图随仓库提交）。
	goods := []struct {
		name  string
		cost  int
		stock *int
		image string
	}{
		{"定制鼠标垫", 8, nil, "/api/uploads/seed/mousepad.jpeg"},
		{"帽子", 30, nil, "/api/uploads/seed/hat.jpeg"},
		{"定制帆布袋", 40, nil, "/api/uploads/seed/canvas-bag.jpeg"},
		{"定制文化衫（白色-纯logo）", 55, nil, "/api/uploads/seed/tshirt-white.jpeg"},
		{"定制文化衫（黑色-蛇年主题）", 55, nil, "/api/uploads/seed/tshirt-black.png"},
		{"定制水杯", 80, nil, "/api/uploads/seed/cup.jpeg"},
		{"100元喜茶卡", 100, nil, "/api/uploads/seed/heytea-card.jpeg"},
		// 实时上新占位：cost=0 → 前台展示「实时上新」不可兑换，也不进盲盒奖池
		{"其他定制节假日礼品", 0, nil, ""},
	}
	for _, g := range goods {
		var stock any = nil
		if g.stock != nil {
			stock = *g.stock
		}
		if err := s.DB.Exec(
			`INSERT INTO mall_items (tenant_id, type, name, cost, stock, image_url) VALUES (?, 'item', ?, ?, ?, ?)`,
			s.DefaultTenantID, g.name, g.cost, stock, g.image,
		).Error; err != nil {
			return err
		}
	}
	// 惊喜盲盒：5 分/次，默认「未中奖也扣分」(charge_on_miss=1，后台可改)
	if err := s.DB.Exec(
		`INSERT INTO mall_items (tenant_id, type, name, cost, stock, image_url, charge_on_miss) VALUES (?, 'blindbox', ?, ?, NULL, ?, 1)`,
		s.DefaultTenantID, "惊喜盲盒", 5, "",
	).Error; err != nil {
		return err
	}
	return nil
}

// seedBlindboxPool 为「惊喜盲盒」灌入默认奖池：7 件积分好物（关联 item_id）+ 一行「无奖品」。
// 默认按价梯度给权重，喜茶卡限 1 份；后台可随时在「奖池配置」里调整。
func (s *Seeder) seedBlindboxPool() error {
	var boxID int64
	s.DB.Raw(`SELECT id FROM mall_items WHERE tenant_id = ? AND type = 'blindbox' AND name = ? LIMIT 1`,
		s.DefaultTenantID, "惊喜盲盒").Scan(&boxID)
	if boxID == 0 {
		return nil
	}
	var cnt int64
	s.DB.Raw(`SELECT COUNT(*) FROM mall_blindbox_pool WHERE box_item_id = ?`, boxID).Scan(&cnt)
	if cnt > 0 {
		return nil
	}
	prizes := []struct {
		name   string
		weight int
		stock  *int
	}{
		{"定制鼠标垫", 30, nil},
		{"帽子", 20, nil},
		{"定制帆布袋", 12, nil},
		{"定制文化衫（白色-纯logo）", 6, nil},
		{"定制文化衫（黑色-蛇年主题）", 6, nil},
		{"定制水杯", 4, nil},
		{"100元喜茶卡", 2, intPtr(1)},
	}
	for _, p := range prizes {
		var g struct {
			ID       int64
			ImageURL string
		}
		s.DB.Raw(`SELECT id, image_url FROM mall_items WHERE tenant_id = ? AND type = 'item' AND name = ? LIMIT 1`,
			s.DefaultTenantID, p.name).Scan(&g)
		if g.ID == 0 {
			continue
		}
		var stock any = nil
		if p.stock != nil {
			stock = *p.stock
		}
		if err := s.DB.Exec(
			`INSERT INTO mall_blindbox_pool (box_item_id, item_id, prize_name, prize_image, weight, stock) VALUES (?, ?, ?, ?, ?, ?)`,
			boxID, g.ID, p.name, g.ImageURL, p.weight, stock,
		).Error; err != nil {
			return err
		}
	}
	// 无奖品行（item_id=NULL）
	if err := s.DB.Exec(
		`INSERT INTO mall_blindbox_pool (box_item_id, item_id, prize_name, prize_image, weight, stock) VALUES (?, NULL, ?, '', ?, NULL)`,
		boxID, "谢谢参与", 80,
	).Error; err != nil {
		return err
	}
	return nil
}

func (s *Seeder) count(table string, tenantID int64) int64 {
	var c int64
	s.DB.Raw(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE tenant_id = ?", table), tenantID).Scan(&c)
	return c
}

func intPtr(v int) *int { return &v }
