//go:build integration

package repository

import (
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/ory/dockertest/v3"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/standardsoftware/culture_points_mall/internal/migrate"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("pool: %v", err)
	}
	res, err := pool.Run("mysql", "8.4.4", []string{"MYSQL_ROOT_PASSWORD=root", "MYSQL_DATABASE=cpm_test"})
	if err != nil {
		log.Fatalf("run: %v", err)
	}
	dsn := fmt.Sprintf("root:root@tcp(localhost:%s)/cpm_test?parseTime=true&charset=utf8mb4", res.GetPort("3306/tcp"))
	if err := pool.Retry(func() error {
		db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err != nil {
			return err
		}
		testDB = db
		sqlDB, _ := db.DB()
		return sqlDB.Ping()
	}); err != nil {
		log.Fatalf("connect: %v", err)
	}
	if err := (&migrate.Runner{DB: testDB, Dir: "../../../../migrations"}).Up(); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	code := m.Run()
	_ = pool.Purge(res)
	os.Exit(code)
}
