//go:build integration

package service

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
	pointsrepo "github.com/standardsoftware/culture_points_mall/internal/modules/points/repository"
	valuesdomain "github.com/standardsoftware/culture_points_mall/internal/modules/values/domain"
	valuesrepo "github.com/standardsoftware/culture_points_mall/internal/modules/values/repository"
	valuessvc "github.com/standardsoftware/culture_points_mall/internal/modules/values/service"
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

	// 从测试文件目录（internal/modules/points/service/）向上 4 层到项目根
	r := &migrate.Runner{DB: testDB, Dir: "../../../../migrations"}
	if err := r.Up(); err != nil {
		log.Fatalf("migrate up: %v", err)
	}

	code := m.Run()
	os.Exit(code)
}

func TestService_AddPoints_RealMySQL(t *testing.T) {
	ctx := context.Background()

	// 隔离：每次测试清空相关表
	require.NoError(t, testDB.Exec("TRUNCATE value_dimensions").Error)
	require.NoError(t, testDB.Exec("TRUNCATE point_transactions").Error)
	require.NoError(t, testDB.Exec("TRUNCATE user_dimension_scores").Error)

	// 准备价值观维度
	vr := valuesrepo.New(testDB)
	vs := valuessvc.New(vr)
	require.NoError(t, vs.Upsert(ctx, &valuesdomain.Dimension{
		TenantID:  1,
		Code:      "customer_first",
		Name:      "客户至上",
		Weight:    1.0,
		SortOrder: 1,
		Enabled:   true,
	}))

	// 积分 service
	pr := pointsrepo.New(testDB)
	s := New(testDB, pr, vs, nil)

	tx, err := s.AddPoints(ctx, AddPointsCmd{
		TenantID: 1,
		UserID:   100,
		Amount:   12,
		DimCode:  "customer_first",
		Reason:   "客户走访",
	})
	require.NoError(t, err)
	require.NotZero(t, tx.ID)

	// 验证快照
	scores, dims, total, err := s.GetUserScores(ctx, 1, 100)
	require.NoError(t, err)
	require.Len(t, scores, 1)
	require.Equal(t, 12, scores[0].TotalScore)
	require.Equal(t, 12, total)
	require.Len(t, dims, 1)

	// 验证流水
	txs, err := s.ListTransactions(ctx, 1, 100, 0, 10)
	require.NoError(t, err)
	require.Len(t, txs, 1)
	require.Equal(t, 12, txs[0].Amount)
	require.Equal(t, "客户走访", txs[0].Reason)

	// 等 1 秒确保 updated_at 不会冲突
	time.Sleep(time.Second)
}
