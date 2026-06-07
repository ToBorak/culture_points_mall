package migrate

import (
	"context"
	"encoding/json"
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
	if err := s.seedCultureDemo(); err != nil {
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

func nullStr(v string) any {
	if v == "" {
		return nil
	}
	return v
}

// seedCultureDemo 灌入「文化刊」演示数据，让团队 migrate seed 后 H5/后台即有完整效果：
//   - 12 个演示用户、四大价值观补全(描述/图标/颜色)
//   - Q1(已公示,含 3 位星标得主)+Q2(提报中,供报名) 两个季次
//   - 已发布的《2026 Q1 文化刊》：star/values/honors/lottery 四栏目 + 快照(baked JSON) + 价值观践行案例
//
// 自包含、幂等(已有 2026Q1 季次则跳过)、不依赖 AI 与源数据。
func (s *Seeder) seedCultureDemo() error {
	t := s.DefaultTenantID
	var exists int64
	s.DB.Raw(`SELECT COUNT(*) FROM star_seasons WHERE tenant_id=? AND quarter_code='2026Q1'`, t).Scan(&exists)
	if exists > 0 {
		return nil
	}

	var firstErr error
	do := func(q string, a ...any) {
		if firstErr == nil {
			firstErr = s.DB.Exec(q, a...).Error
		}
	}
	id := func(q string, a ...any) int64 {
		var v int64
		s.DB.Raw(q, a...).Scan(&v)
		return v
	}

	// 1) 演示用户 员工01..员工12
	for i := 1; i <= 12; i++ {
		do(`INSERT IGNORE INTO users (tenant_id, ding_user_id, name, avatar_url) VALUES (?, ?, ?, '')`,
			t, fmt.Sprintf("demo-%03d", i), fmt.Sprintf("员工%02d", i))
	}
	uid := func(name string) int64 {
		return id(`SELECT id FROM users WHERE tenant_id=? AND name=? ORDER BY id LIMIT 1`, t, name)
	}

	// 2) 四大价值观补全(upsert，确保存在 + 补 description/icon/color)
	dims := []struct{ code, name, kw, icon, color, desc string }{
		{"customer_first", "客户第一", "客户,响应,体验", "🎯", "#f97316", "以客户成功为先，主动响应、超出预期"},
		{"candor", "坦诚沟通", "坦诚,反馈,信任", "💬", "#0ea5e9", "开诚布公、对事不对人、敢说真话"},
		{"ownership", "一号位", "担当,主人翁,推进", "🚀", "#10b981", "以主人翁心态推进，敢担当、不推诿"},
		{"innovation", "敢于创新", "创新,突破,变化", "💡", "#ec4899", "勇于突破、拥抱变化、持续创新"},
	}
	for i, d := range dims {
		do(`INSERT INTO value_dimensions (tenant_id, code, name, keywords, weight, sort_order, enabled, description, icon, color)
			VALUES (?, ?, ?, ?, 1.0, ?, 1, ?, ?, ?)
			ON DUPLICATE KEY UPDATE description=VALUES(description), icon=VALUES(icon), color=VALUES(color)`,
			t, d.code, d.name, d.kw, i, d.desc, d.icon, d.color)
	}
	dimID := func(code string) int64 {
		return id(`SELECT id FROM value_dimensions WHERE tenant_id=? AND code=? LIMIT 1`, t, code)
	}
	dC, dK, dO, dN := dimID("customer_first"), dimID("candor"), dimID("ownership"), dimID("innovation")

	// 3) 季次：Q1 已公示 / Q2 提报中
	do(`INSERT INTO star_seasons (tenant_id,name,quarter_code,status,nominate_start_at,nominate_end_at)
		VALUES (?, '2026 Q1 文化星标','2026Q1','published','2026-01-01 00:00:00','2026-03-31 23:59:59')`, t)
	do(`INSERT INTO star_seasons (tenant_id,name,quarter_code,status,nominate_start_at,nominate_end_at)
		VALUES (?, '2026 Q2 文化星标','2026Q2','nominating', NOW() - INTERVAL 5 DAY, NOW() + INTERVAL 30 DAY)`, t)
	q1 := id(`SELECT id FROM star_seasons WHERE tenant_id=? AND quarter_code='2026Q1' LIMIT 1`, t)
	q2 := id(`SELECT id FROM star_seasons WHERE tenant_id=? AND quarter_code='2026Q2' LIMIT 1`, t)

	// 4) Q1 提报 + 得主
	noms := []struct {
		nominator, nominee, dim int64
		text, refined, status   string
		score                   any
	}{
		{uid("员工02"), uid("员工01"), dN, "重构盲盒抽奖积分冻结逻辑，把超时回滚从人工改成自动，月省两小时核对。", "员工01 主动重构盲盒抽奖的积分冻结链路，将超时回滚由人工改为自动，每月省下约两小时核对。", "selected", 9.2},
		{uid("员工03"), uid("员工05"), dC, "客户半夜反馈支付异常，第一时间响应陪查到凌晨，次日给出根因与补偿。", "员工05 在客户深夜报障时第一时间响应，陪查至凌晨并次日给出补偿方案。", "selected", 9.0},
		{uid("员工04"), uid("员工08"), dK, "复盘会直言项目排期风险，推动团队提前两周调整，避免延期。", "员工08 在复盘会坦率指出排期风险，推动提前两周调整，避免延期。", "selected", 8.8},
		{uid("员工05"), uid("员工06"), dO, "无人认领的线上事故主动接手，牵头复盘并落地三条改进。", "", "shortlisted", 7.5},
		{uid("员工06"), uid("员工07"), dN, "用 AI 自动汇总周报，团队周报效率翻倍。", "", "submitted", nil},
		{uid("员工07"), uid("员工02"), dC, "为新客户做超出合同范围的上手培训，促成续约。", "", "submitted", nil},
	}
	for _, n := range noms {
		do(`INSERT INTO star_nominations (tenant_id,season_id,nominator_id,nominee_id,dimension_id,case_text,case_refined,status,score)
			VALUES (?,?,?,?,?,?,?,?,?)`, t, q1, n.nominator, n.nominee, n.dim, n.text, nullStr(n.refined), n.status, n.score)
	}
	do(`INSERT INTO star_nominations (tenant_id,season_id,nominator_id,nominee_id,dimension_id,case_text,status)
		VALUES (?,?,?,?,?, '主动牵头跨部门协作，推动季度目标对齐。', 'submitted')`, t, q2, uid("员工01"), uid("员工03"), dO)

	winners := []struct {
		user, dim int64
		cit       string
	}{
		{uid("员工01"), dN, "以自动化重构守护系统稳健，敢创新、能落地——2026 Q1「敢于创新」星标。"},
		{uid("员工05"), dC, "把客户的急难当成自己的事，深夜驰援——2026 Q1「客户第一」星标。"},
		{uid("员工08"), dK, "敢说真话、对事不对人，让风险提前暴露——2026 Q1「坦诚沟通」星标。"},
	}
	for _, w := range winners {
		do(`INSERT INTO star_winners (tenant_id,season_id,user_id,dimension_id,citation) VALUES (?,?,?,?,?)`, t, q1, w.user, w.dim, w.cit)
	}

	// 5) 文化刊 + 栏目 + 案例文章
	do(`INSERT INTO publications (tenant_id,season_id,title,period_code,intro_text,status,published_at)
		VALUES (?,?, '2026 Q1 文化刊','2026Q1', ?, 'published', NOW())`,
		t, q1, "这一季，我们一起把价值观活成了具体的样子。3 位同事当选文化星标，更多努力被看见——翻开这期，看看那些被记住的微光。")
	pub := id(`SELECT id FROM publications WHERE tenant_id=? AND period_code='2026Q1' LIMIT 1`, t)

	secs := []struct{ typ, title, copy string }{
		{"star", "本期文化星标", "三位同事，三种坚持。"},
		{"values", "价值观专区", "四个关键词，定义我们是谁。"},
		{"honors", "获奖公示", "里程碑达成，实至名归。"},
		{"lottery", "幸运转盘中奖", "恭喜以下同事抽中好礼。"},
	}
	for i, sec := range secs {
		do(`INSERT INTO publication_sections (publication_id,type,title,sort_order,visible,ai_copy) VALUES (?,?,?,?,1,?)`,
			pub, sec.typ, sec.title, i, sec.copy)
	}
	secID := func(typ string) int64 {
		return id(`SELECT id FROM publication_sections WHERE publication_id=? AND type=? LIMIT 1`, pub, typ)
	}
	valSec := secID("values")
	arts := []struct {
		title, body string
		dim         int64
	}{
		{"员工01 · 敢于创新", "员工01 主动重构盲盒抽奖的积分冻结链路，把超时回滚由人工改为自动，每月为团队省下约两小时核对，体现对系统稳健性的极致追求。", dN},
		{"员工05 · 客户第一", "客户深夜报障，员工05 没有等到上班，而是立刻上线陪排查至凌晨，次日给出根因与补偿——把『明天再说』变成『现在就办』。", dC},
		{"员工08 · 坦诚沟通", "员工08 在复盘会坦率指出排期风险，推动团队提前两周调整，避免项目延期——敢说真话，是最朴素的坦诚。", dK},
	}
	for _, a := range arts {
		do(`INSERT INTO publication_articles (tenant_id,publication_id,section_id,title,content_html,source_type,value_dimension_id,status)
			VALUES (?,?,?,?,?, 'manual', ?, 'published')`, t, pub, valSec, a.title, a.body, a.dim)
	}

	// 6) 快照(baked JSON，字段与前端 Row 类型对齐)
	snap := func(typ string, rows []map[string]any) {
		raw, _ := json.Marshal(rows)
		do(`INSERT INTO publication_snapshots (publication_id, section_id, data_json) VALUES (?, ?, ?)`, pub, secID(typ), string(raw))
	}
	snap("star", []map[string]any{
		{"userId": uid("员工01"), "name": "员工01", "avatarUrl": "", "dimension": "敢于创新", "citation": winners[0].cit},
		{"userId": uid("员工05"), "name": "员工05", "avatarUrl": "", "dimension": "客户第一", "citation": winners[1].cit},
		{"userId": uid("员工08"), "name": "员工08", "avatarUrl": "", "dimension": "坦诚沟通", "citation": winners[2].cit},
	})
	snap("values", []map[string]any{
		{"dimensionId": dC, "name": "客户第一", "description": dims[0].desc, "icon": "🎯", "color": "#f97316", "nominationCount": 2},
		{"dimensionId": dK, "name": "坦诚沟通", "description": dims[1].desc, "icon": "💬", "color": "#0ea5e9", "nominationCount": 1},
		{"dimensionId": dO, "name": "一号位", "description": dims[2].desc, "icon": "🚀", "color": "#10b981", "nominationCount": 1},
		{"dimensionId": dN, "name": "敢于创新", "description": dims[3].desc, "icon": "💡", "color": "#ec4899", "nominationCount": 2},
	})
	snap("honors", []map[string]any{
		{"userId": uid("员工01"), "name": "员工01", "badge": "富甲一方", "rarity": "legendary", "iconUrl": "ingot", "earnedAt": "2026-03-20"},
		{"userId": uid("员工05"), "name": "员工05", "badge": "持之以恒", "rarity": "epic", "iconUrl": "flame", "earnedAt": "2026-03-18"},
		{"userId": uid("员工08"), "name": "员工08", "badge": "厚积薄发", "rarity": "epic", "iconUrl": "burst", "earnedAt": "2026-03-15"},
		{"userId": uid("员工03"), "name": "员工03", "badge": "渐入佳境", "rarity": "rare", "iconUrl": "calendar_check", "earnedAt": "2026-03-10"},
		{"userId": uid("员工07"), "name": "员工07", "badge": "初来乍到", "rarity": "common", "iconUrl": "sprout", "earnedAt": "2026-03-05"},
	})
	snap("lottery", []map[string]any{
		{"userId": uid("员工01"), "name": "员工01", "prize": "定制背包", "wonAt": "2026-03-28"},
		{"userId": uid("员工05"), "name": "员工05", "prize": "100元喜茶卡", "wonAt": "2026-03-28"},
		{"userId": uid("员工08"), "name": "员工08", "prize": "定制水杯", "wonAt": "2026-03-28"},
		{"userId": uid("员工02"), "name": "员工02", "prize": "帽子", "wonAt": "2026-03-28"},
		{"userId": uid("员工04"), "name": "员工04", "prize": "定制帆布袋", "wonAt": "2026-03-28"},
	})

	return firstErr
}
