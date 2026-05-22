//go:build integration

package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/ory/dockertest/v3"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/standardsoftware/culture_points_mall/internal/migrate"
	mallrepo "github.com/standardsoftware/culture_points_mall/internal/modules/mall/repository"
	pointsrepo "github.com/standardsoftware/culture_points_mall/internal/modules/points/repository"
	pointssvc "github.com/standardsoftware/culture_points_mall/internal/modules/points/service"
	valuesdomain "github.com/standardsoftware/culture_points_mall/internal/modules/values/domain"
	valuesrepo "github.com/standardsoftware/culture_points_mall/internal/modules/values/repository"
	valuessvc "github.com/standardsoftware/culture_points_mall/internal/modules/values/service"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatal(err)
	}
	res, err := pool.Run("mysql", "8.4.4", []string{"MYSQL_ROOT_PASSWORD=root", "MYSQL_DATABASE=cpm_test"})
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = pool.Purge(res) }()

	dsn := fmt.Sprintf("root:root@tcp(127.0.0.1:%s)/cpm_test?charset=utf8mb4&parseTime=true&loc=Local", res.GetPort("3306/tcp"))
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
		log.Fatal(err)
	}

	r := &migrate.Runner{DB: testDB, Dir: "../../../../migrations"}
	if err := r.Up(); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	os.Exit(m.Run())
}

func TestDraw_TCC_Integration(t *testing.T) {
	ctx := context.Background()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	// 清空相关表，保证测试隔离
	require.NoError(t, testDB.Exec("TRUNCATE value_dimensions").Error)
	require.NoError(t, testDB.Exec("TRUNCATE point_transactions").Error)
	require.NoError(t, testDB.Exec("TRUNCATE user_dimension_scores").Error)
	require.NoError(t, testDB.Exec("TRUNCATE mall_items").Error)
	require.NoError(t, testDB.Exec("TRUNCATE mall_blindbox_pool").Error)
	require.NoError(t, testDB.Exec("TRUNCATE mall_blindbox_freeze").Error)
	require.NoError(t, testDB.Exec("TRUNCATE mall_orders").Error)

	// 准备 values
	vr := valuesrepo.New(testDB)
	require.NoError(t, vr.Upsert(ctx, &valuesdomain.Dimension{TenantID: 1, Code: "customer_first", Name: "客户至上", Weight: 1.0, SortOrder: 1, Enabled: true}))
	vs := valuessvc.New(vr)

	// 准备 points（带 redis）
	pr := pointsrepo.New(testDB)
	ps := pointssvc.New(testDB, pr, vs, rdb)

	// 给 user 100 灌入足够积分（500 分给到 customer_first）
	require.NoError(t, testDB.Exec(
		`INSERT INTO user_dimension_scores (user_id, tenant_id, dimension_id, total_score, quarter_score, year_score)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE total_score = ?, quarter_score = ?, year_score = ?`,
		int64(100), int64(1), int64(1), 500, 500, 500, 500, 500, 500,
	).Error)

	// 准备 mall：商品 + 奖品池
	mr2 := mallrepo.New(testDB)
	require.NoError(t, testDB.Exec(
		`INSERT INTO mall_items (id, tenant_id, type, name, cost) VALUES (?, ?, ?, ?, ?)`,
		int64(100), int64(1), "blindbox", "测试盲盒", 80,
	).Error)
	// 60% 未中奖 / 40% 咖啡券
	require.NoError(t, testDB.Exec(
		`INSERT INTO mall_blindbox_pool (box_item_id, prize_name, prize_image, weight) VALUES (?, ?, ?, ?), (?, ?, ?, ?)`,
		int64(100), "未中奖（鼓励气泡）", "", 60,
		int64(100), "咖啡券", "", 40,
	).Error)

	svc := New(mr2, ps, vs)

	// 连续抽 10 次，统计 win/miss
	winCount, missCount := 0, 0
	for i := 0; i < 10; i++ {
		res, err := svc.Draw(ctx, 1, 100, 100)
		require.NoError(t, err)
		if res.Win {
			winCount++
		} else {
			missCount++
		}
	}
	t.Logf("draw 10 times: %d win / %d miss", winCount, missCount)

	// 验证 freeze 状态
	var confirmedCnt int64
	testDB.Raw(`SELECT COUNT(*) FROM mall_blindbox_freeze WHERE status = 'confirmed' AND user_id = 100`).Scan(&confirmedCnt)
	require.Equal(t, int64(winCount), confirmedCnt, "confirmed freeze count should equal wins")

	var cancelledCnt int64
	testDB.Raw(`SELECT COUNT(*) FROM mall_blindbox_freeze WHERE status = 'cancelled' AND user_id = 100`).Scan(&cancelledCnt)
	require.Equal(t, int64(missCount), cancelledCnt, "cancelled freeze count should equal misses")

	// 验证用户积分仅扣减 win 数 × 80
	total, err := pr.GetTotalScore(ctx, 1, 100)
	require.NoError(t, err)
	require.Equal(t, 500-winCount*80, total, "score should only deduct for wins")

	// 验证超期 freeze sweep（手动插入一条过期）
	expired := time.Now().Add(-1 * time.Minute)
	require.NoError(t, testDB.Exec(
		`INSERT INTO mall_blindbox_freeze (tx_id, user_id, box_item_id, amount, status, expires_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"tx-expired-test", int64(100), int64(100), 80, "try", expired,
	).Error)

	expiredRows, err := mr2.ListExpiredFreeze(ctx, time.Now(), 10)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(expiredRows), 1)
}
