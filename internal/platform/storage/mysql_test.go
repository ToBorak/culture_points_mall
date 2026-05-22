//go:build integration

package storage

import (
	"testing"

	"github.com/standardsoftware/culture_points_mall/internal/config"
	"github.com/stretchr/testify/require"
)

func TestMySQLConnect_Integration(t *testing.T) {
	cfg := config.MySQLCfg{DSN: "root:root@tcp(127.0.0.1:33306)/cpm_test?charset=utf8mb4&parseTime=true&loc=Local"}
	db, err := NewMySQL(cfg)
	require.NoError(t, err)
	require.NotNil(t, db)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
}
