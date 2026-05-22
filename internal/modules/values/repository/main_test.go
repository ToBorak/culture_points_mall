//go:build integration

package repository

import (
	"log"
	"os"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/standardsoftware/culture_points_mall/internal/migrate"
)

var integrationDB *gorm.DB

func TestMain(m *testing.M) {
	// connect without selecting a database so we can drop/recreate cpm_test
	adminDSN := "root:root@tcp(127.0.0.1:33306)/?charset=utf8mb4&parseTime=true&loc=Local"
	adminDB, err := gorm.Open(mysql.Open(adminDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("open admin mysql: %v", err)
	}
	if err := adminDB.Exec("DROP DATABASE IF EXISTS cpm_test").Error; err != nil {
		log.Fatalf("drop db: %v", err)
	}
	if err := adminDB.Exec("CREATE DATABASE cpm_test CHARACTER SET utf8mb4").Error; err != nil {
		log.Fatalf("create db: %v", err)
	}

	// now connect to cpm_test and run migrations
	dsn := "root:root@tcp(127.0.0.1:33306)/cpm_test?charset=utf8mb4&parseTime=true&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("open cpm_test: %v", err)
	}
	integrationDB = db

	r := &migrate.Runner{DB: db, Dir: "../../../../migrations"}
	if err := r.Up(); err != nil {
		log.Fatalf("migrate up: %v", err)
	}

	os.Exit(m.Run())
}
