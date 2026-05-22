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
	activitiesdomain "github.com/standardsoftware/culture_points_mall/internal/modules/activities/domain"
	activitiesrepo "github.com/standardsoftware/culture_points_mall/internal/modules/activities/repository"
	activitiessvc "github.com/standardsoftware/culture_points_mall/internal/modules/activities/service"
	achvrepo "github.com/standardsoftware/culture_points_mall/internal/modules/achievements/repository"
	achvsvc "github.com/standardsoftware/culture_points_mall/internal/modules/achievements/service"
	pointsrepo "github.com/standardsoftware/culture_points_mall/internal/modules/points/repository"
	pointssvc "github.com/standardsoftware/culture_points_mall/internal/modules/points/service"
	signinrepo "github.com/standardsoftware/culture_points_mall/internal/modules/signin/repository"
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

func TestSignin_FullFlow_RealMySQL(t *testing.T) {
	ctx := context.Background()

	// 1) 准备价值观维度
	vr := valuesrepo.New(testDB)
	require.NoError(t, vr.Upsert(ctx, &valuesdomain.Dimension{TenantID: 1, Code: "team_collab", Name: "团队协作", Weight: 1.0, SortOrder: 2, Enabled: true}))
	vs := valuessvc.New(vr)

	// 2) 准备 activities service + 创建活动（dimension=team_collab, reward=15）
	ar := activitiesrepo.New(testDB)
	as := activitiessvc.New(ar, vs)
	act, err := as.Create(ctx, activitiessvc.CreateCmd{TenantID: 1, DimensionCode: "team_collab", Title: "签到集成测试", PointsReward: 15})
	require.NoError(t, err)

	// 3) 准备 points / achievements
	pr := pointsrepo.New(testDB)
	ps := pointssvc.New(testDB, pr, vs, nil)
	achvS := achvsvc.New(&achvsvc.Wrap{Inner: achvrepo.New(testDB)}, ps, vs)

	// 4) signin service
	sr := signinrepo.New(testDB)
	secret := "int-test-secret"
	s := New(sr, as, ps, achvS, secret, 60)

	// 5) 生成当前 HMAC code，直接走 service.Check
	code := s.CurrentCode(act.ID)
	res, err := s.Check(ctx, CheckCmd{TenantID: 1, UserID: 99, ActivityID: act.ID, Code: code})
	require.NoError(t, err)
	require.True(t, res.OK)
	require.NotZero(t, res.TransactionID)

	// 6) 验证 points 入账
	_, _, total, err := ps.GetUserScores(ctx, 1, 99)
	require.NoError(t, err)
	require.Equal(t, 15, total)

	// 7) 重复签到 → ErrAlreadySignedIn
	_, err = s.Check(ctx, CheckCmd{TenantID: 1, UserID: 99, ActivityID: act.ID, Code: code})
	require.ErrorIs(t, err, ErrAlreadySignedIn)

	// 8) 错误 code → rejected（不抛错）
	res, err = s.Check(ctx, CheckCmd{TenantID: 1, UserID: 100, ActivityID: act.ID, Code: "wrong"})
	require.NoError(t, err)
	require.False(t, res.OK)
	_ = activitiesdomain.StatusPublished // silence unused
}
