package dingtalk

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE dingtalk_mock_outbox (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tenant_id INTEGER NOT NULL,
		api TEXT NOT NULL,
		target TEXT,
		payload TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`).Error)
	return db
}

func TestMockClient_SendWorkNotice(t *testing.T) {
	db := setupSQLite(t)
	bus := NewBus()
	sub := bus.Subscribe()
	m := NewMock(db, bus)

	ctx := context.Background()
	require.NoError(t, m.SendWorkNotice(ctx, []string{"u1", "u2"}, Card{Title: "测试", Detail: "你好"}))

	var count int64
	require.NoError(t, db.Table("dingtalk_mock_outbox").Count(&count).Error)
	require.Equal(t, int64(1), count)

	ev := <-sub
	require.Equal(t, "send_work_notice", ev.API)
	require.Equal(t, "u1,u2", ev.Target)
}

func TestMockGetUserByCode_FillsNewFields(t *testing.T) {
	m := NewMock(nil, NewBus())
	u, err := m.GetUserByCode(context.Background(), "abc")
	require.NoError(t, err)
	require.Equal(t, "mock_abc", u.DingUserID)
	require.NotEmpty(t, u.UnionID)
	require.False(t, u.IsAdmin)
}
