//go:build integration

package repository_test

import (
	"context"
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
	"github.com/standardsoftware/culture_points_mall/internal/modules/publication/repository"
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

	// 从测试文件目录（internal/modules/publication/repository/）向上 4 层到项目根
	r := &migrate.Runner{DB: testDB, Dir: "../../../../migrations"}
	if err := r.Up(); err != nil {
		log.Fatalf("migrate up: %v", err)
	}

	code := m.Run()
	os.Exit(code)
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

// insertUser 插入一个用户，返回其 ID。
func insertUser(t *testing.T, tenantID int64, name string) int64 {
	t.Helper()
	require.NoError(t, testDB.Exec(
		"INSERT INTO users (tenant_id, name, avatar_url) VALUES (?, ?, ?)", tenantID, name, "https://example.com/avatar.png",
	).Error)
	var id int64
	require.NoError(t, testDB.Raw("SELECT LAST_INSERT_ID()").Scan(&id).Error)
	return id
}

// insertDimension 插入一个启用的价值观维度（含 011 新字段），返回 ID。
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

func TestAggStarWinners(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	const tenantID = int64(1)

	repo := repository.New(testDB)

	// 建两个用户、两个维度
	u1 := insertUser(t, tenantID, "张三")
	u2 := insertUser(t, tenantID, "李四")
	d1 := insertDimension(t, tenantID, "innovation")
	d2 := insertDimension(t, tenantID, "teamwork")

	// 建季次
	require.NoError(t, testDB.Exec(
		"INSERT INTO star_seasons (tenant_id, name, quarter_code, status) VALUES (?, ?, ?, 'judging')",
		tenantID, "2026Q2测试季", "2026Q2",
	).Error)
	var seasonID int64
	require.NoError(t, testDB.Raw("SELECT LAST_INSERT_ID()").Scan(&seasonID).Error)

	// 插入两个获奖记录
	require.NoError(t, testDB.Exec(
		"INSERT INTO star_winners (tenant_id, season_id, user_id, dimension_id, citation) VALUES (?,?,?,?,?),(?,?,?,?,?)",
		tenantID, seasonID, u1, d1, "创新能手",
		tenantID, seasonID, u2, d2, "团队精神",
	).Error)

	rows, err := repo.AggStarWinners(ctx, tenantID, seasonID)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	// 按 w.id ASC 排序，第一行是 u1
	require.Equal(t, u1, rows[0].UserID)
	require.Equal(t, "张三", rows[0].Name)
	require.Equal(t, "innovation-name", rows[0].Dimension)
	require.Equal(t, "创新能手", rows[0].Citation)
	require.NotEmpty(t, rows[0].AvatarURL)

	require.Equal(t, u2, rows[1].UserID)
	require.Equal(t, "李四", rows[1].Name)
	require.Equal(t, "teamwork-name", rows[1].Dimension)
	require.Equal(t, "团队精神", rows[1].Citation)

	// 不同 season 不应返回结果
	rows2, err := repo.AggStarWinners(ctx, tenantID, seasonID+999)
	require.NoError(t, err)
	require.Empty(t, rows2)
}

func TestAggLottery_WindowAndPrizeName(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	const tenantID = int64(1)

	repo := repository.New(testDB)

	u1 := insertUser(t, tenantID, "王五")
	u2 := insertUser(t, tenantID, "赵六")

	// 建盲盒商品和奖池
	require.NoError(t, testDB.Exec(
		"INSERT INTO mall_items (tenant_id, type, name, cost) VALUES (?, 'blindbox', '神秘盲盒', 100)",
		tenantID,
	).Error)
	var boxItemID int64
	require.NoError(t, testDB.Raw("SELECT LAST_INSERT_ID()").Scan(&boxItemID).Error)

	require.NoError(t, testDB.Exec(
		"INSERT INTO mall_blindbox_pool (box_item_id, prize_name, weight) VALUES (?, '一等奖iPhone', 10),(?, '二等奖图书', 20)",
		boxItemID, boxItemID,
	).Error)
	var pool1ID, pool2ID int64
	require.NoError(t, testDB.Raw("SELECT id FROM mall_blindbox_pool ORDER BY id ASC LIMIT 1").Scan(&pool1ID).Error)
	require.NoError(t, testDB.Raw("SELECT id FROM mall_blindbox_pool ORDER BY id DESC LIMIT 1").Scan(&pool2ID).Error)

	// u1 中奖时间：窗口内 2026-05-15
	// u2 中奖时间：窗口外 2026-04-01
	require.NoError(t, testDB.Exec(
		"INSERT INTO mall_orders (tenant_id, user_id, prize_id, cost, created_at) VALUES (?,?,?,100,'2026-05-15 10:00:00'),(?,?,?,100,'2026-04-01 10:00:00')",
		tenantID, u1, pool1ID,
		tenantID, u2, pool2ID,
	).Error)

	start := time.Date(2026, 5, 1, 0, 0, 0, 0, time.Local)
	end := time.Date(2026, 5, 31, 23, 59, 59, 0, time.Local)

	// 窗口内：只返回 u1
	rows, err := repo.AggLottery(ctx, tenantID, &start, &end, 10)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, u1, rows[0].UserID)
	require.Equal(t, "王五", rows[0].Name)
	require.Equal(t, "一等奖iPhone", rows[0].Prize)
	require.Equal(t, "2026-05-15", rows[0].WonAt)

	// 不限窗口：返回 2 行（newest first）
	all, err := repo.AggLottery(ctx, tenantID, nil, nil, 10)
	require.NoError(t, err)
	require.Len(t, all, 2)

	// 验证二等奖名字正确
	var foundPool2 bool
	for _, row := range all {
		if row.UserID == u2 {
			require.Equal(t, "二等奖图书", row.Prize)
			foundPool2 = true
		}
	}
	require.True(t, foundPool2, "应能找到 u2 的二等奖记录")
}

func TestAggLeaderboard_SumQuarter(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	const tenantID = int64(1)

	repo := repository.New(testDB)

	u1 := insertUser(t, tenantID, "高分甲")
	u2 := insertUser(t, tenantID, "高分乙")
	d1 := insertDimension(t, tenantID, "courage")
	d2 := insertDimension(t, tenantID, "integrity")

	// u1: 维度 d1 季度分 60 + 维度 d2 季度分 40 = 100
	// u2: 维度 d1 季度分 30 + 维度 d2 季度分 50 = 80
	require.NoError(t, testDB.Exec(
		"INSERT INTO user_dimension_scores (user_id, tenant_id, dimension_id, total_score, quarter_score) VALUES (?,?,?,60,60),(?,?,?,40,40),(?,?,?,30,30),(?,?,?,50,50)",
		u1, tenantID, d1,
		u1, tenantID, d2,
		u2, tenantID, d1,
		u2, tenantID, d2,
	).Error)

	rows, err := repo.AggLeaderboard(ctx, tenantID, 10)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	// 按 SUM(quarter_score) 倒序：u1=100, u2=80
	require.Equal(t, u1, rows[0].UserID)
	require.Equal(t, "高分甲", rows[0].Name)
	require.Equal(t, 100, rows[0].Score)

	require.Equal(t, u2, rows[1].UserID)
	require.Equal(t, "高分乙", rows[1].Name)
	require.Equal(t, 80, rows[1].Score)

	// limit=1 只返回第一名
	top1, err := repo.AggLeaderboard(ctx, tenantID, 1)
	require.NoError(t, err)
	require.Len(t, top1, 1)
	require.Equal(t, u1, top1[0].UserID)
}
