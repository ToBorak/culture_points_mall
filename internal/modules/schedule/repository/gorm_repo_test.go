//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/standardsoftware/culture_points_mall/internal/modules/schedule/domain"
)

func TestScheduleRepo_CreateAndList(t *testing.T) {
	require.NoError(t, testDB.Exec("TRUNCATE schedules").Error)
	r := New(testDB)
	ctx := context.Background()
	s := &domain.Schedule{
		TenantID: 1, Title: "周会", StartAt: time.Now(), EndAt: time.Now().Add(time.Hour),
		Location: "线上", Detail: "聊聊", AttendeeUserIDs: []string{"u1", "u2"}, GroupIDs: []string{"culture"},
		PushCalendar: true, PushGroup: true, Status: domain.StatusPublished, CalendarEventID: "evt-1",
	}
	require.NoError(t, r.Create(ctx, s))
	require.NotZero(t, s.ID)

	rows, err := r.ListByTenant(ctx, 1, 10)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "周会", rows[0].Title)
	require.Equal(t, []string{"u1", "u2"}, rows[0].AttendeeUserIDs)
	require.Equal(t, "evt-1", rows[0].CalendarEventID)
}
