//go:build integration

package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/standardsoftware/culture_points_mall/internal/migrate"
	"github.com/standardsoftware/culture_points_mall/internal/modules/publication/domain"
	pubrepo "github.com/standardsoftware/culture_points_mall/internal/modules/publication/repository"
	pubsvc "github.com/standardsoftware/culture_points_mall/internal/modules/publication/service"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("dockertest.NewPool: %v", err)
	}

	// 优先本地已有的 8.4.4，避免网络 pull 失败
	res, err := pool.Run("mysql", "8.4.4", []string{
		"MYSQL_ROOT_PASSWORD=root",
		"MYSQL_DATABASE=cpm_test",
	})
	if err != nil {
		log.Fatalf("pool.Run mysql: %v", err)
	}
	defer func() { _ = pool.Purge(res) }()

	dsn := fmt.Sprintf(
		"root:root@tcp(127.0.0.1:%s)/cpm_test?charset=utf8mb4&parseTime=true&loc=Local",
		res.GetPort("3306/tcp"),
	)

	pool.MaxWait = 120 * time.Second
	if err := pool.Retry(func() error {
		db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err != nil {
			return err
		}
		sqlDB, _ := db.DB()
		if err := sqlDB.Ping(); err != nil {
			return err
		}
		testDB = db
		return nil
	}); err != nil {
		log.Fatalf("connect mysql: %v", err)
	}

	// 从测试文件目录（internal/modules/publication/service/）向上 4 层到项目根
	r := &migrate.Runner{DB: testDB, Dir: "../../../../migrations"}
	if err := r.Up(); err != nil {
		log.Fatalf("migrate up: %v", err)
	}

	code := m.Run()
	os.Exit(code)
}

// newTestSvc 构建真实 Service（不 mock 任何项目自有代码）。llm/ding 传 nil，AI/推送功能不参与集成测试。
func newTestSvc() *pubsvc.Service {
	return pubsvc.New(pubrepo.New(testDB), nil, nil)
}

// truncateAll 隔离每个用例：清空测试相关表。
func truncateAll(t *testing.T) {
	t.Helper()
	tables := []string{
		"publication_snapshots",
		"publication_articles",
		"publication_sections",
		"publications",
		"star_winners",
		"star_nominations",
		"star_seasons",
		"user_badges",
		"badges",
		"mall_orders",
		"mall_blindbox_pool",
		"mall_items",
		"activities",
		"user_dimension_scores",
		"users",
		"value_dimensions",
	}
	for _, tbl := range tables {
		require.NoError(t, testDB.Exec("TRUNCATE "+tbl).Error)
	}
}

// insertUser 插入用户，返回其 ID。
func insertUser(t *testing.T, tenantID int64, name string) int64 {
	t.Helper()
	require.NoError(t, testDB.Exec(
		"INSERT INTO users (tenant_id, name, avatar_url) VALUES (?, ?, ?)", tenantID, name, "https://example.com/avatar.png",
	).Error)
	var id int64
	require.NoError(t, testDB.Raw("SELECT LAST_INSERT_ID()").Scan(&id).Error)
	return id
}

// insertDimension 插入价值观维度（含 011 新字段），返回 ID。
func insertDimension(t *testing.T, tenantID int64, code string) int64 {
	t.Helper()
	require.NoError(t, testDB.Exec(
		"INSERT INTO value_dimensions (tenant_id, code, name, description, icon, color, enabled) VALUES (?, ?, ?, ?, ?, ?, 1)",
		tenantID, code, code+"-name", code+"-desc", "icon-"+code, "#ffffff",
	).Error)
	var id int64
	require.NoError(t, testDB.Raw("SELECT LAST_INSERT_ID()").Scan(&id).Error)
	return id
}

// TestAggregate_WritesSnapshots 验证：
//   - 建 draft 期（带 season + period）
//   - 配 star + lottery + leaderboard 三个可见 SnapshotBacked 栏目
//   - 造源数据（star_winners / mall_orders+pool / user_dimension_scores）
//   - Aggregate → publication_snapshots 3 行，每行 data_json 可反序列化出预期行数
//   - 再调一次 Aggregate（幂等）→ 仍 3 行
func TestAggregate_WritesSnapshots(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	svc := newTestSvc()
	const tenantID = int64(1)

	// ── 建季次 ──────────────────────────────────────────────────────────────
	require.NoError(t, testDB.Exec(
		"INSERT INTO star_seasons (tenant_id, name, quarter_code, status) VALUES (?, ?, ?, 'judging')",
		tenantID, "2026Q2测试季", "2026Q2",
	).Error)
	var seasonID int64
	require.NoError(t, testDB.Raw("SELECT LAST_INSERT_ID()").Scan(&seasonID).Error)

	// ── 建期刊（draft，带 season + period）──────────────────────────────────
	periodStart := time.Date(2026, 5, 1, 0, 0, 0, 0, time.Local)
	periodEnd := time.Date(2026, 5, 31, 23, 59, 59, 0, time.Local)
	pub, err := svc.CreateIssue(ctx, pubsvc.CreateIssueCmd{
		TenantID:    tenantID,
		SeasonID:    &seasonID,
		Title:       "2026年5月刊",
		PeriodCode:  "2026-05",
		PeriodStart: &periodStart,
		PeriodEnd:   &periodEnd,
	})
	require.NoError(t, err)
	require.NotZero(t, pub.ID)

	// ── 配三个可见 SnapshotBacked 栏目（star / lottery / leaderboard）──────
	err = svc.ConfigureSections(ctx, tenantID, pub.ID, []domain.Section{
		{Type: domain.SecStar, Title: "明星员工", SortOrder: 1, Visible: true},
		{Type: domain.SecLottery, Title: "幸运抽奖", SortOrder: 2, Visible: true},
		{Type: domain.SecLeaderboard, Title: "积分榜", SortOrder: 3, Visible: true},
	})
	require.NoError(t, err)

	// ── 造源数据 ─────────────────────────────────────────────────────────────

	// star_winners：2 个获奖者
	u1 := insertUser(t, tenantID, "张三")
	u2 := insertUser(t, tenantID, "李四")
	u3 := insertUser(t, tenantID, "王五")
	d1 := insertDimension(t, tenantID, "innovation")
	d2 := insertDimension(t, tenantID, "teamwork")

	require.NoError(t, testDB.Exec(
		"INSERT INTO star_winners (tenant_id, season_id, user_id, dimension_id, citation) VALUES (?,?,?,?,?),(?,?,?,?,?)",
		tenantID, seasonID, u1, d1, "创新先锋",
		tenantID, seasonID, u2, d2, "协作标杆",
	).Error)

	// mall_orders + pool：1 条中奖记录（在 period 窗口内）
	require.NoError(t, testDB.Exec(
		"INSERT INTO mall_items (tenant_id, type, name, cost) VALUES (?, 'blindbox', '神秘盲盒', 100)", tenantID,
	).Error)
	var boxItemID int64
	require.NoError(t, testDB.Raw("SELECT LAST_INSERT_ID()").Scan(&boxItemID).Error)

	require.NoError(t, testDB.Exec(
		"INSERT INTO mall_blindbox_pool (box_item_id, prize_name, weight) VALUES (?, '一等奖', 10)", boxItemID,
	).Error)
	var poolID int64
	require.NoError(t, testDB.Raw("SELECT LAST_INSERT_ID()").Scan(&poolID).Error)

	require.NoError(t, testDB.Exec(
		"INSERT INTO mall_orders (tenant_id, user_id, prize_id, cost, created_at) VALUES (?, ?, ?, 100, '2026-05-15 10:00:00')",
		tenantID, u3, poolID,
	).Error)

	// user_dimension_scores：u1/u2 各有积分
	require.NoError(t, testDB.Exec(
		"INSERT INTO user_dimension_scores (user_id, tenant_id, dimension_id, total_score, quarter_score) VALUES (?,?,?,80,80),(?,?,?,60,60)",
		u1, tenantID, d1,
		u2, tenantID, d2,
	).Error)

	// ── 第一次 Aggregate ──────────────────────────────────────────────────────
	require.NoError(t, svc.Aggregate(ctx, tenantID, pub.ID))

	// 断言：publication_snapshots 应有 3 行
	var snapCount int64
	require.NoError(t, testDB.Raw("SELECT COUNT(*) FROM publication_snapshots WHERE publication_id = ?", pub.ID).Scan(&snapCount).Error)
	require.Equal(t, int64(3), snapCount, "应写入 3 个快照")

	// 断言：star 快照可反序列化出 2 行
	var starSnap struct{ DataJSON string }
	require.NoError(t, testDB.Raw(
		"SELECT ps.data_json FROM publication_snapshots ps JOIN publication_sections sec ON sec.id = ps.section_id WHERE ps.publication_id = ? AND sec.type = ?",
		pub.ID, domain.SecStar,
	).Scan(&starSnap).Error)
	var starRows []domain.StarWinnerRow
	require.NoError(t, json.Unmarshal([]byte(starSnap.DataJSON), &starRows))
	require.Len(t, starRows, 2, "star 快照应有 2 行")

	// 断言：lottery 快照可反序列化出 1 行（窗口内）
	var lotterySnap struct{ DataJSON string }
	require.NoError(t, testDB.Raw(
		"SELECT ps.data_json FROM publication_snapshots ps JOIN publication_sections sec ON sec.id = ps.section_id WHERE ps.publication_id = ? AND sec.type = ?",
		pub.ID, domain.SecLottery,
	).Scan(&lotterySnap).Error)
	var lotteryRows []domain.LotteryRow
	require.NoError(t, json.Unmarshal([]byte(lotterySnap.DataJSON), &lotteryRows))
	require.Len(t, lotteryRows, 1, "lottery 快照应有 1 行")

	// 断言：leaderboard 快照可反序列化出 2 行
	var lbSnap struct{ DataJSON string }
	require.NoError(t, testDB.Raw(
		"SELECT ps.data_json FROM publication_snapshots ps JOIN publication_sections sec ON sec.id = ps.section_id WHERE ps.publication_id = ? AND sec.type = ?",
		pub.ID, domain.SecLeaderboard,
	).Scan(&lbSnap).Error)
	var lbRows []domain.LeaderRow
	require.NoError(t, json.Unmarshal([]byte(lbSnap.DataJSON), &lbRows))
	require.Len(t, lbRows, 2, "leaderboard 快照应有 2 行")

	// ── 第二次 Aggregate（幂等验证）──────────────────────────────────────────
	require.NoError(t, svc.Aggregate(ctx, tenantID, pub.ID))

	var snapCount2 int64
	require.NoError(t, testDB.Raw("SELECT COUNT(*) FROM publication_snapshots WHERE publication_id = ?", pub.ID).Scan(&snapCount2).Error)
	require.Equal(t, int64(3), snapCount2, "第二次 Aggregate 后仍应为 3 行（幂等）")
}

// TestGetCurrent_AssemblesView 验证 GetCurrent 读组装：
//   - 快照类栏目 (star, visible) 的 Snapshot 非空且可反序列化出 winner 行
//   - 成稿类栏目 (custom, visible) 的 Articles 含预期文章
//   - invisible 栏目被排除
//   - Sections 按 sort_order 有序
func TestGetCurrent_AssemblesView(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	svc := newTestSvc()
	const tenantID = int64(1)

	// ── 建季次 ──────────────────────────────────────────────────────────────
	require.NoError(t, testDB.Exec(
		"INSERT INTO star_seasons (tenant_id, name, quarter_code, status) VALUES (?, ?, ?, 'judging')",
		tenantID, "2026Q2测试季", "2026Q2",
	).Error)
	var seasonID int64
	require.NoError(t, testDB.Raw("SELECT LAST_INSERT_ID()").Scan(&seasonID).Error)

	// ── 建期刊（draft，带 season + period）──────────────────────────────────
	periodStart := time.Date(2026, 5, 1, 0, 0, 0, 0, time.Local)
	periodEnd := time.Date(2026, 5, 31, 23, 59, 59, 0, time.Local)
	pub, err := svc.CreateIssue(ctx, pubsvc.CreateIssueCmd{
		TenantID:    tenantID,
		SeasonID:    &seasonID,
		Title:       "2026年5月读组装测试刊",
		PeriodCode:  "2026-05-assemble",
		PeriodStart: &periodStart,
		PeriodEnd:   &periodEnd,
	})
	require.NoError(t, err)
	require.NotZero(t, pub.ID)

	// ── 配 3 个栏目：star(可见,sort=1) + custom(可见,sort=2) + editorial(不可见,sort=3) ──
	err = svc.ConfigureSections(ctx, tenantID, pub.ID, []domain.Section{
		{Type: domain.SecStar, Title: "明星员工", SortOrder: 1, Visible: true},
		{Type: domain.SecCustom, Title: "自定义文章", SortOrder: 2, Visible: true},
		{Type: domain.SecEditorial, Title: "卷首语(不可见)", SortOrder: 3, Visible: false},
	})
	require.NoError(t, err)

	// 查询刚写入的 section ID，供后续使用
	var sections []domain.Section
	require.NoError(t, testDB.
		Where("publication_id = ?", pub.ID).
		Order("sort_order ASC").
		Find(&sections).Error)
	require.Len(t, sections, 3)
	starSectionID := sections[0].ID   // sort_order=1, star
	customSectionID := sections[1].ID // sort_order=2, custom
	// sections[2] is editorial (invisible)

	// ── 造 star 源数据（2 个获奖者）──────────────────────────────────────────
	u1 := insertUser(t, tenantID, "张三-assemble")
	u2 := insertUser(t, tenantID, "李四-assemble")
	d1 := insertDimension(t, tenantID, "innovation-asm")
	d2 := insertDimension(t, tenantID, "teamwork-asm")

	require.NoError(t, testDB.Exec(
		"INSERT INTO star_winners (tenant_id, season_id, user_id, dimension_id, citation) VALUES (?,?,?,?,?),(?,?,?,?,?)",
		tenantID, seasonID, u1, d1, "创新先锋-assemble",
		tenantID, seasonID, u2, d2, "协作标杆-assemble",
	).Error)

	// ── Aggregate（写入 star 快照）────────────────────────────────────────────
	require.NoError(t, svc.Aggregate(ctx, tenantID, pub.ID))

	// ── 建一篇 custom 文章，SectionID 指向 custom 栏目 ────────────────────────
	article := &domain.Article{
		TenantID:      tenantID,
		PublicationID: &pub.ID,
		SectionID:     &customSectionID,
		Title:         "自定义文章标题",
		ContentHTML:   "<p>正文内容</p>",
	}
	require.NoError(t, svc.UpsertArticle(ctx, tenantID, article))
	require.NotZero(t, article.ID)

	// ── Publish ────────────────────────────────────────────────────────────────
	require.NoError(t, svc.Publish(ctx, tenantID, pub.ID))

	// ── 调 GetCurrent 断言组装结果 ────────────────────────────────────────────
	v, err := svc.GetCurrent(ctx, tenantID)
	require.NoError(t, err)

	// Publication 非 nil 且 Status = published
	require.NotNil(t, v.Publication)
	require.Equal(t, domain.PubPublished, v.Publication.Status)

	// Sections 只含 visible 栏目（star + custom），editorial 被排除
	require.Len(t, v.Sections, 2, "invisible 栏目应被排除，只剩 2 个可见栏目")

	// 按 sort_order 有序：star(1) → custom(2)
	require.Equal(t, domain.SecStar, v.Sections[0].Section.Type, "第一个栏目应为 star")
	require.Equal(t, domain.SecCustom, v.Sections[1].Section.Type, "第二个栏目应为 custom")

	// star 栏目的 Snapshot 非空，可反序列化出 2 条 winner 行
	require.NotEmpty(t, v.Sections[0].Snapshot, "star 栏目的 Snapshot 不应为空")
	var starRows []domain.StarWinnerRow
	require.NoError(t, json.Unmarshal(v.Sections[0].Snapshot, &starRows), "star Snapshot 应可反序列化为 StarWinnerRow 切片")
	require.Len(t, starRows, 2, "star 快照应含 2 条 winner 行")

	// custom 栏目的 Articles 含那篇文章
	require.Len(t, v.Sections[1].Articles, 1, "custom 栏目应含 1 篇文章")
	require.Equal(t, "自定义文章标题", v.Sections[1].Articles[0].Title)

	// ── （可选）调 GetDetail 断言同样组装 ──────────────────────────────────────
	vd, err := svc.GetDetail(ctx, tenantID, pub.ID)
	require.NoError(t, err)
	require.Equal(t, v.Publication.ID, vd.Publication.ID)
	require.Len(t, vd.Sections, 2, "GetDetail 也应排除 invisible 栏目")

	// 验证 star 快照在 GetDetail 中也可用
	var starRowsD []domain.StarWinnerRow
	require.NotEmpty(t, vd.Sections[0].Snapshot)
	require.NoError(t, json.Unmarshal(vd.Sections[0].Snapshot, &starRowsD))
	require.Len(t, starRowsD, 2)

	// 验证 custom 文章 ID 与 section 映射正确（防止 artBySection 分组错误）
	require.Equal(t, starSectionID, vd.Sections[0].Section.ID, "star 栏目 ID 应匹配")
	require.Equal(t, customSectionID, vd.Sections[1].Section.ID, "custom 栏目 ID 应匹配")
	require.Equal(t, article.ID, vd.Sections[1].Articles[0].ID, "custom 文章 ID 应匹配")
}
