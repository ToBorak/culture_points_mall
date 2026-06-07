//go:build integration

package migrate_test

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/standardsoftware/culture_points_mall/internal/migrate"
)

const migrationsDir = "../../migrations"

// rootDSN 指向容器（不带库名），用于按用例动态建库。
var rootDSN string

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("dockertest.NewPool: %v", err)
	}

	// 优先本地已有的 8.4.4，避免网络 pull 失败（与其它集成测试一致）。
	res, err := pool.Run("mysql", "8.4.4", []string{"MYSQL_ROOT_PASSWORD=root"})
	if err != nil {
		log.Fatalf("pool.Run mysql: %v", err)
	}
	defer func() { _ = pool.Purge(res) }()

	rootDSN = fmt.Sprintf("root:root@tcp(127.0.0.1:%s)/", res.GetPort("3306/tcp"))

	pool.MaxWait = 120 * time.Second
	if err := pool.Retry(func() error {
		db, err := gorm.Open(mysql.Open(rootDSN), &gorm.Config{})
		if err != nil {
			return err
		}
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		return sqlDB.Ping()
	}); err != nil {
		log.Fatalf("connect mysql: %v", err)
	}

	os.Exit(m.Run())
}

// freshDB 为每个用例新建一个独立空库，返回连到该库的 *gorm.DB。
// 用例间互不影响，便于单独模拟 fresh / 重跑 / 老库接入等场景。
func freshDB(t *testing.T) *gorm.DB {
	t.Helper()
	admin, err := gorm.Open(mysql.Open(rootDSN), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)

	name := "mig_" + sanitize(t.Name())
	require.NoError(t, admin.Exec("DROP DATABASE IF EXISTS "+name).Error)
	require.NoError(t, admin.Exec("CREATE DATABASE "+name+" CHARACTER SET utf8mb4").Error)

	dsn := rootDSN + name + "?charset=utf8mb4&parseTime=true&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		// 与生产 storage.NewMySQL 一致，用 Warn 级别，以便观察迁移期是否刷红。
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	require.NoError(t, err)
	return db
}

func sanitize(name string) string {
	repl := func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			return r
		default:
			return '_'
		}
	}
	return strings.Map(repl, name)
}

// upFileCount 返回 migrations 目录下 *.up.sql 的数量（随分支变化，不写死）。
func upFileCount(t *testing.T) int64 {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	require.NoError(t, err)
	require.NotEmpty(t, files, "应至少有一个 .up.sql 迁移文件")
	return int64(len(files))
}

func migrationCount(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM schema_migrations").Scan(&n).Error)
	return n
}

// 全新库：逐条 apply，并把每个版本写入 schema_migrations。
func TestRunnerUp_FreshAppliesAndRecordsVersions(t *testing.T) {
	db := freshDB(t)
	r := &migrate.Runner{DB: db, Dir: migrationsDir}

	require.NoError(t, r.Up())

	require.True(t, db.Migrator().HasTable("value_dimensions"), "首迁移建的表应存在")
	require.Equal(t, upFileCount(t), migrationCount(t, db), "应记录全部已应用版本")
}

// 重复执行：第二次 Up() 不得因 "table already exists" 报错，且不重复记录版本。
func TestRunnerUp_IdempotentRerun(t *testing.T) {
	db := freshDB(t)
	r := &migrate.Runner{DB: db, Dir: migrationsDir}

	require.NoError(t, r.Up())
	require.NoError(t, r.Up(), "对已建库重跑不应报错")

	require.Equal(t, upFileCount(t), migrationCount(t, db), "重跑不应重复记录版本")
}

// 老库接入：库里已有表但无版本表（删掉 schema_migrations 模拟），
// 再次 Up() 应容忍 "已存在" 类错误、补记版本、整体不报错。
func TestRunnerUp_AdoptsExistingUntrackedDB(t *testing.T) {
	db := freshDB(t)
	r := &migrate.Runner{DB: db, Dir: migrationsDir}

	require.NoError(t, r.Up())
	require.NoError(t, db.Exec("DROP TABLE schema_migrations").Error)

	require.NoError(t, r.Up(), "接入老库不应因表已存在而报错")
	require.Equal(t, upFileCount(t), migrationCount(t, db), "接入后应补记全部版本")
}
