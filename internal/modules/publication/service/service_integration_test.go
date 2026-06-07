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

// newTestSvc 构建真实 Service（不 mock 任何项目自有代码）。
func newTestSvc() *pubsvc.Service {
	return pubsvc.New(pubrepo.New(testDB))
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
