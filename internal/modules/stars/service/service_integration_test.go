//go:build integration

package service_test

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

	"github.com/standardsoftware/culture_points_mall/internal/config"
	"github.com/standardsoftware/culture_points_mall/internal/migrate"
	pointsrepo "github.com/standardsoftware/culture_points_mall/internal/modules/points/repository"
	pointssvc "github.com/standardsoftware/culture_points_mall/internal/modules/points/service"
	"github.com/standardsoftware/culture_points_mall/internal/modules/stars/domain"
	starsrepo "github.com/standardsoftware/culture_points_mall/internal/modules/stars/repository"
	starssvc "github.com/standardsoftware/culture_points_mall/internal/modules/stars/service"
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

	// 从测试文件目录（internal/modules/stars/service/）向上 4 层到项目根
	r := &migrate.Runner{DB: testDB, Dir: "../../../../migrations"}
	if err := r.Up(); err != nil {
		log.Fatalf("migrate up: %v", err)
	}

	code := m.Run()
	os.Exit(code)
}

// newTestSvc 构建真实的 stars.Service（不 mock 任何项目自有代码）。
func newTestSvc() *starssvc.Service {
	vr := valuesrepo.New(testDB)
	vs := valuessvc.New(vr)
	pr := pointsrepo.New(testDB)
	ps := pointssvc.New(testDB, pr, vs, nil)
	repo := starsrepo.New(testDB)
	return starssvc.New(repo, ps, config.StarsCfg{
		NominatePoints:      2,
		NominatedPoints:     4,
		WinnerPoints:        8,
		NominateMonthlyCap:  6,
		NominatedMonthlyCap: 16,
	})
}

// truncateAll 隔离每个用例：清空相关表。
func truncateAll(t *testing.T) {
	t.Helper()
	tables := []string{
		"star_winners",
		"star_nominations",
		"star_seasons",
		"point_transactions",
		"user_dimension_scores",
		"users",
		"value_dimensions",
	}
	for _, tbl := range tables {
		require.NoError(t, testDB.Exec("TRUNCATE "+tbl).Error)
	}
}

// insertDimension 插入一个价值观维度，返回其 ID。
func insertDimension(t *testing.T, ctx context.Context, vs *valuessvc.Service, tenantID int64, code string) int64 {
	t.Helper()
	d := &valuesdomain.Dimension{
		TenantID:  tenantID,
		Code:      code,
		Name:      code,
		Weight:    1.0,
		SortOrder: 1,
		Enabled:   true,
	}
	require.NoError(t, vs.Upsert(ctx, d))
	// 查回 ID
	var row valuesdomain.Dimension
	require.NoError(t, testDB.Where("tenant_id = ? AND code = ?", tenantID, code).First(&row).Error)
	return row.ID
}

// insertUser 插入一个用户，返回其 ID。
func insertUser(t *testing.T, tenantID int64, name string) int64 {
	t.Helper()
	require.NoError(t, testDB.Exec(
		"INSERT INTO users (tenant_id, name) VALUES (?, ?)", tenantID, name,
	).Error)
	var id int64
	require.NoError(t, testDB.Raw("SELECT LAST_INSERT_ID()").Scan(&id).Error)
	return id
}

// insertSeason 插入一个提报中的季次，返回其 ID。
func insertSeason(t *testing.T, ctx context.Context, svc *starssvc.Service, tenantID int64, code string) int64 {
	t.Helper()
	sn := &domain.Season{
		TenantID:    tenantID,
		Name:        "测试季 " + code,
		QuarterCode: code,
		Status:      domain.SeasonNominating,
	}
	require.NoError(t, svc.Repo.CreateSeason(ctx, sn))
	return sn.ID
}

// sumPoints 查询 point_transactions 中某用户在某维度的正积分总和。
func sumPoints(t *testing.T, tenantID, userID, dimensionID int64) int {
	t.Helper()
	var total int64
	require.NoError(t, testDB.Raw(
		"SELECT COALESCE(SUM(amount),0) FROM point_transactions WHERE tenant_id=? AND user_id=? AND dimension_id=? AND amount>0",
		tenantID, userID, dimensionID,
	).Scan(&total).Error)
	return int(total)
}

func TestNominate_AwardsPointsWithMonthlyCap(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	svc := newTestSvc()
	const tenantID = int64(1)

	// 准备价值观维度（4 个，规避 uk_dedup 复用同一对人）
	vr := valuesrepo.New(testDB)
	vs := valuessvc.New(vr)
	dimIDs := make([]int64, 4)
	for i := 0; i < 4; i++ {
		dimIDs[i] = insertDimension(t, ctx, vs, tenantID, fmt.Sprintf("dim%d", i))
	}

	// 准备两个用户
	u1 := insertUser(t, tenantID, "提报人U1")
	u2 := insertUser(t, tenantID, "被提名人U2")

	// 建季次
	seasonID := insertSeason(t, ctx, svc, tenantID, "2026Q2")

	// 前 3 次提报（不同 dimension，规避 uk_dedup）
	for i := 0; i < 3; i++ {
		_, err := svc.Nominate(ctx, starssvc.NominateCmd{
			TenantID:    tenantID,
			SeasonID:    seasonID,
			NominatorID: u1,
			NomineeID:   u2,
			DimensionID: dimIDs[i],
			CaseText:    fmt.Sprintf("case %d", i),
		})
		require.NoError(t, err)
	}

	// U1 提报积分 = 3*2=6，U2 被提名积分 = 3*4=12
	require.Equal(t, 6, sumPoints(t, tenantID, u1, dimIDs[0])+
		sumPoints(t, tenantID, u1, dimIDs[1])+
		sumPoints(t, tenantID, u1, dimIDs[2]))
	require.Equal(t, 12, sumPoints(t, tenantID, u2, dimIDs[0])+
		sumPoints(t, tenantID, u2, dimIDs[1])+
		sumPoints(t, tenantID, u2, dimIDs[2]))

	// 第 4 次提报（dim3）：U1 已到月上限 6，不再加分；U2 被提名累计到 16
	_, err := svc.Nominate(ctx, starssvc.NominateCmd{
		TenantID:    tenantID,
		SeasonID:    seasonID,
		NominatorID: u1,
		NomineeID:   u2,
		DimensionID: dimIDs[3],
		CaseText:    "case 3",
	})
	require.NoError(t, err)

	// U1 提报积分仍为 6（封顶，不再增加）
	u1Total := sumPoints(t, tenantID, u1, dimIDs[0]) +
		sumPoints(t, tenantID, u1, dimIDs[1]) +
		sumPoints(t, tenantID, u1, dimIDs[2]) +
		sumPoints(t, tenantID, u1, dimIDs[3])
	require.Equal(t, 6, u1Total, "U1 提报积分应封顶在 6")

	// U2 被提名积分 = 12+4=16
	u2Total := sumPoints(t, tenantID, u2, dimIDs[0]) +
		sumPoints(t, tenantID, u2, dimIDs[1]) +
		sumPoints(t, tenantID, u2, dimIDs[2]) +
		sumPoints(t, tenantID, u2, dimIDs[3])
	require.Equal(t, 16, u2Total, "U2 被提名积分应为 16")
}

func TestNominate_DuplicateRejected(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	svc := newTestSvc()
	const tenantID = int64(1)

	vr := valuesrepo.New(testDB)
	vs := valuessvc.New(vr)
	dimID := insertDimension(t, ctx, vs, tenantID, "dim_dup")
	u1 := insertUser(t, tenantID, "提报人")
	u2 := insertUser(t, tenantID, "被提名人")
	seasonID := insertSeason(t, ctx, svc, tenantID, "2026Q3")

	// 第一次成功
	_, err := svc.Nominate(ctx, starssvc.NominateCmd{
		TenantID:    tenantID,
		SeasonID:    seasonID,
		NominatorID: u1,
		NomineeID:   u2,
		DimensionID: dimID,
		CaseText:    "first",
	})
	require.NoError(t, err)

	// 同季同提报人同对象同维度 -> ErrDuplicateNomination
	_, err = svc.Nominate(ctx, starssvc.NominateCmd{
		TenantID:    tenantID,
		SeasonID:    seasonID,
		NominatorID: u1,
		NomineeID:   u2,
		DimensionID: dimID,
		CaseText:    "second",
	})
	require.ErrorIs(t, err, starssvc.ErrDuplicateNomination)
}

func TestSelectWinners_Idempotent(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	svc := newTestSvc()
	const tenantID = int64(1)

	// 准备维度和用户
	vr := valuesrepo.New(testDB)
	vs := valuessvc.New(vr)
	dimID := insertDimension(t, ctx, vs, tenantID, "dim_winner")
	_ = insertUser(t, tenantID, "任意用户U1") // 占位，确保 LAST_INSERT_ID 推进
	u2 := insertUser(t, tenantID, "当选用户U2")

	// 建季次并推进到 judging 状态
	sn := &domain.Season{
		TenantID:    tenantID,
		Name:        "定榜测试季",
		QuarterCode: "2026Q9",
		Status:      domain.SeasonJudging,
	}
	require.NoError(t, svc.Repo.CreateSeason(ctx, sn))
	seasonID := sn.ID

	picks := []starssvc.Pick{
		{UserID: u2, DimensionID: dimID, Citation: "优秀表现"},
	}

	// 第一次 SelectWinners
	require.NoError(t, svc.SelectWinners(ctx, tenantID, seasonID, picks))

	// 断言：star_winners 1 行
	var winnerCount int64
	require.NoError(t, testDB.Raw(
		"SELECT COUNT(*) FROM star_winners WHERE season_id=? AND user_id=? AND dimension_id=?",
		seasonID, u2, dimID,
	).Scan(&winnerCount).Error)
	require.Equal(t, int64(1), winnerCount, "第一次定榜后 star_winners 应有 1 行")

	// 断言：U2 在该维度评选积分 = WinnerPoints = 8
	pts1 := sumPoints(t, tenantID, u2, dimID)
	require.Equal(t, 8, pts1, "第一次定榜后评选积分应为 8")

	// 第二次用相同参数再调（幂等重跑）
	require.NoError(t, svc.SelectWinners(ctx, tenantID, seasonID, picks))

	// 断言：star_winners 仍 1 行（不重复插入）
	require.NoError(t, testDB.Raw(
		"SELECT COUNT(*) FROM star_winners WHERE season_id=? AND user_id=? AND dimension_id=?",
		seasonID, u2, dimID,
	).Scan(&winnerCount).Error)
	require.Equal(t, int64(1), winnerCount, "第二次定榜后 star_winners 仍应为 1 行（幂等）")

	// 断言：积分仍 = 8（不翻倍）
	pts2 := sumPoints(t, tenantID, u2, dimID)
	require.Equal(t, 8, pts2, "第二次定榜后积分不应翻倍，仍为 8")
}

func TestNominate_SeasonNotOpen(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	svc := newTestSvc()
	const tenantID = int64(1)

	vr := valuesrepo.New(testDB)
	vs := valuessvc.New(vr)
	dimID := insertDimension(t, ctx, vs, tenantID, "dim_closed")
	u1 := insertUser(t, tenantID, "提报人2")
	u2 := insertUser(t, tenantID, "被提名人2")

	// 建一个 judging 状态的季次
	sn := &domain.Season{
		TenantID:    tenantID,
		Name:        "评审季",
		QuarterCode: "2026Q4",
		Status:      domain.SeasonJudging,
	}
	require.NoError(t, svc.Repo.CreateSeason(ctx, sn))

	_, err := svc.Nominate(ctx, starssvc.NominateCmd{
		TenantID:    tenantID,
		SeasonID:    sn.ID,
		NominatorID: u1,
		NomineeID:   u2,
		DimensionID: dimID,
		CaseText:    "late entry",
	})
	require.ErrorIs(t, err, starssvc.ErrSeasonNotOpen)
}
