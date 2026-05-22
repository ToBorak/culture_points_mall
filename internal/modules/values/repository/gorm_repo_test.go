//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/standardsoftware/culture_points_mall/internal/config"
	"github.com/standardsoftware/culture_points_mall/internal/modules/values/domain"
	"github.com/standardsoftware/culture_points_mall/internal/platform/storage"
	"github.com/stretchr/testify/require"
)

func TestGormRepo_Upsert_List_Integration(t *testing.T) {
	db, err := storage.NewMySQL(config.MySQLCfg{DSN: "root:root@tcp(127.0.0.1:33306)/cpm_test?charset=utf8mb4&parseTime=true&loc=Local"})
	require.NoError(t, err)
	require.NoError(t, db.Exec("TRUNCATE value_dimensions").Error)

	r := New(db)
	ctx := context.Background()
	require.NoError(t, r.Upsert(ctx, &domain.Dimension{TenantID: 1, Code: "customer_first", Name: "客户至上", Weight: 1.0, SortOrder: 1, Enabled: true}))

	got, err := r.ListByTenant(ctx, 1)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "customer_first", got[0].Code)
}
