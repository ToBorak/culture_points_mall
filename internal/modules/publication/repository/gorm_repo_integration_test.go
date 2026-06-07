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

func TestAggValues(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	const tenantID = int64(1)

	repo := repository.New(testDB)

	// 建三个价值观维度，sort_order 显式设置确保排序可断言
	require.NoError(t, testDB.Exec(
		"INSERT INTO value_dimensions (tenant_id, code, name, description, icon, color, sort_order, enabled) VALUES (?,?,?,?,?,?,?,1)",
		tenantID, "collab", "协作", "协作精神", "icon-collab", "#ff0000", 1,
	).Error)
	var d1 int64
	require.NoError(t, testDB.Raw("SELECT LAST_INSERT_ID()").Scan(&d1).Error)

	require.NoError(t, testDB.Exec(
		"INSERT INTO value_dimensions (tenant_id, code, name, description, icon, color, sort_order, enabled) VALUES (?,?,?,?,?,?,?,1)",
		tenantID, "inno", "创新", "创新精神", "icon-inno", "#00ff00", 2,
	).Error)
	var d2 int64
	require.NoError(t, testDB.Raw("SELECT LAST_INSERT_ID()").Scan(&d2).Error)

	// 第三个维度 enabled=0，不应出现在结果里
	require.NoError(t, testDB.Exec(
		"INSERT INTO value_dimensions (tenant_id, code, name, description, icon, color, sort_order, enabled) VALUES (?,?,?,?,?,?,?,0)",
		tenantID, "legacy", "废弃维度", "已禁用", "icon-legacy", "#000000", 3,
	).Error)

	// 建季次
	require.NoError(t, testDB.Exec(
		"INSERT INTO star_seasons (tenant_id, name, quarter_code, status) VALUES (?, ?, ?, 'nominating')",
		tenantID, "2026Q3测试季", "2026Q3",
	).Error)
	var seasonID int64
	require.NoError(t, testDB.Raw("SELECT LAST_INSERT_ID()").Scan(&seasonID).Error)

	// 建两个用户作为提名人和被提名人
	u1 := insertUser(t, tenantID, "提名人A")
	u2 := insertUser(t, tenantID, "被提名人B")

	// d1 收到 2 条提名，d2 收到 1 条提名
	require.NoError(t, testDB.Exec(
		"INSERT INTO star_nominations (tenant_id, season_id, nominator_id, nominee_id, dimension_id, case_text) VALUES (?,?,?,?,?,?)",
		tenantID, seasonID, u1, u2, d1, "第一条提名 d1",
	).Error)
	// 第二条 nominator/nominee 互换以绕过 UNIQUE KEY uk_dedup(season_id, nominator_id, nominee_id, dimension_id)
	require.NoError(t, testDB.Exec(
		"INSERT INTO star_nominations (tenant_id, season_id, nominator_id, nominee_id, dimension_id, case_text) VALUES (?,?,?,?,?,?)",
		tenantID, seasonID, u2, u1, d1, "第二条提名 d1",
	).Error)
	require.NoError(t, testDB.Exec(
		"INSERT INTO star_nominations (tenant_id, season_id, nominator_id, nominee_id, dimension_id, case_text) VALUES (?,?,?,?,?,?)",
		tenantID, seasonID, u1, u2, d2, "第一条提名 d2",
	).Error)

	rows, err := repo.AggValues(ctx, tenantID, seasonID)
	require.NoError(t, err)

	// 只返回 enabled=1 的两个维度
	require.Len(t, rows, 2)

	// 按 sort_order ASC：d1(sort_order=1) 先，d2(sort_order=2) 后
	require.Equal(t, d1, rows[0].DimensionID)
	require.Equal(t, "协作", rows[0].Name)
	require.Equal(t, "协作精神", rows[0].Description)
	require.Equal(t, "icon-collab", rows[0].Icon)
	require.Equal(t, "#ff0000", rows[0].Color)
	require.Equal(t, 2, rows[0].NominationCount) // d1 有 2 条提名

	require.Equal(t, d2, rows[1].DimensionID)
	require.Equal(t, "创新", rows[1].Name)
	require.Equal(t, 1, rows[1].NominationCount) // d2 有 1 条提名

	// 不同 season 的提名不计入
	rows2, err := repo.AggValues(ctx, tenantID, seasonID+999)
	require.NoError(t, err)
	require.Len(t, rows2, 2)
	for _, r := range rows2 {
		require.Equal(t, 0, r.NominationCount)
	}
}

func TestAggHonors(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	const tenantID = int64(1)

	repo := repository.New(testDB)

	u1 := insertUser(t, tenantID, "勇气勋章主")
	u2 := insertUser(t, tenantID, "窗口外用户")
	d1 := insertDimension(t, tenantID, "valor")

	// 建两个勋章（tenant_id / dimension_id / rarity / icon_url）
	require.NoError(t, testDB.Exec(
		"INSERT INTO badges (tenant_id, dimension_id, name, description, rarity, icon_url) VALUES (?,?,?,?,?,?)",
		tenantID, d1, "勇气之星", "勇气相关勋章", "epic", "https://example.com/valor.png",
	).Error)
	var badge1ID int64
	require.NoError(t, testDB.Raw("SELECT LAST_INSERT_ID()").Scan(&badge1ID).Error)

	require.NoError(t, testDB.Exec(
		"INSERT INTO badges (tenant_id, dimension_id, name, description, rarity, icon_url) VALUES (?,?,?,?,?,?)",
		tenantID, d1, "传奇先锋", "传奇级别勋章", "legendary", "https://example.com/pioneer.png",
	).Error)
	var badge2ID int64
	require.NoError(t, testDB.Raw("SELECT LAST_INSERT_ID()").Scan(&badge2ID).Error)

	// u1 在窗口内获得 badge1（2026-05-20）
	require.NoError(t, testDB.Exec(
		"INSERT INTO user_badges (user_id, badge_id, earned_at) VALUES (?,?,'2026-05-20 12:00:00')",
		u1, badge1ID,
	).Error)

	// u2 在窗口外获得 badge2（2026-04-10）
	require.NoError(t, testDB.Exec(
		"INSERT INTO user_badges (user_id, badge_id, earned_at) VALUES (?,?,'2026-04-10 08:00:00')",
		u2, badge2ID,
	).Error)

	start := time.Date(2026, 5, 1, 0, 0, 0, 0, time.Local)
	end := time.Date(2026, 5, 31, 23, 59, 59, 0, time.Local)

	// 窗口内：只返回 u1
	rows, err := repo.AggHonors(ctx, tenantID, &start, &end, 10)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, u1, rows[0].UserID)
	require.Equal(t, "勇气勋章主", rows[0].Name)
	require.Equal(t, "勇气之星", rows[0].Badge)
	require.Equal(t, "epic", rows[0].Rarity)
	require.Equal(t, "https://example.com/valor.png", rows[0].IconURL)
	require.Equal(t, "2026-05-20", rows[0].EarnedAt)

	// 窗口外记录被过滤：u2 不出现
	for _, r := range rows {
		require.NotEqual(t, u2, r.UserID)
	}

	// 不限窗口：两条都返回
	all, err := repo.AggHonors(ctx, tenantID, nil, nil, 10)
	require.NoError(t, err)
	require.Len(t, all, 2)

	// limit=1 只返回最新一条（earned_at DESC -> u1 的 2026-05-20 在前）
	limited, err := repo.AggHonors(ctx, tenantID, nil, nil, 1)
	require.NoError(t, err)
	require.Len(t, limited, 1)
	require.Equal(t, u1, limited[0].UserID)
}

func TestAggActivities(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	const tenantID = int64(1)

	repo := repository.New(testDB)

	d1 := insertDimension(t, tenantID, "activity_dim")

	// 活动1：start_at 窗口内 2026-05-10
	require.NoError(t, testDB.Exec(
		"INSERT INTO activities (tenant_id, dimension_id, title, status, start_at) VALUES (?,?,?,?,?)",
		tenantID, d1, "五月团建活动", "published", "2026-05-10 09:00:00",
	).Error)
	var act1ID int64
	require.NoError(t, testDB.Raw("SELECT LAST_INSERT_ID()").Scan(&act1ID).Error)

	// 活动2：start_at 窗口内 2026-05-25
	require.NoError(t, testDB.Exec(
		"INSERT INTO activities (tenant_id, dimension_id, title, status, start_at) VALUES (?,?,?,?,?)",
		tenantID, d1, "五月尾读书会", "published", "2026-05-25 14:00:00",
	).Error)
	var act2ID int64
	require.NoError(t, testDB.Raw("SELECT LAST_INSERT_ID()").Scan(&act2ID).Error)

	// 活动3：start_at 窗口外 2026-04-05
	require.NoError(t, testDB.Exec(
		"INSERT INTO activities (tenant_id, dimension_id, title, status, start_at) VALUES (?,?,?,?,?)",
		tenantID, d1, "四月旧活动", "closed", "2026-04-05 10:00:00",
	).Error)

	start := time.Date(2026, 5, 1, 0, 0, 0, 0, time.Local)
	end := time.Date(2026, 5, 31, 23, 59, 59, 0, time.Local)

	// 窗口内：只返回 5 月的 2 条（start_at DESC：act2 先，act1 后）
	rows, err := repo.AggActivities(ctx, tenantID, &start, &end, 10)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	// start_at DESC：act2(2026-05-25) 排在前，act1(2026-05-10) 排在后
	require.Equal(t, act2ID, rows[0].ID)
	require.Equal(t, "五月尾读书会", rows[0].Title)
	require.Equal(t, "2026-05-25", rows[0].StartAt)

	require.Equal(t, act1ID, rows[1].ID)
	require.Equal(t, "五月团建活动", rows[1].Title)
	require.Equal(t, "2026-05-10", rows[1].StartAt)

	// 窗口外记录被过滤：不应出现四月旧活动
	for _, r := range rows {
		require.NotEqual(t, "四月旧活动", r.Title)
	}

	// 不限窗口 limit=1：只返回最新一条（start_at DESC -> act2）
	limited, err := repo.AggActivities(ctx, tenantID, nil, nil, 1)
	require.NoError(t, err)
	require.Len(t, limited, 1)
	require.Equal(t, act2ID, limited[0].ID)

	// 不限窗口 limit=10：三条全返回
	all, err := repo.AggActivities(ctx, tenantID, nil, nil, 10)
	require.NoError(t, err)
	require.Len(t, all, 3)
}
