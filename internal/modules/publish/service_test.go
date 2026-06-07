package publish

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	activitiesdomain "github.com/standardsoftware/culture_points_mall/internal/modules/activities/domain"
	activitiessvc "github.com/standardsoftware/culture_points_mall/internal/modules/activities/service"
	scheduledomain "github.com/standardsoftware/culture_points_mall/internal/modules/schedule/domain"
	schedulesvc "github.com/standardsoftware/culture_points_mall/internal/modules/schedule/service"
	usersdomain "github.com/standardsoftware/culture_points_mall/internal/modules/users/domain"
)

type fakeAct struct {
	cmd activitiessvc.CreateCmd
	err error
}

func (f *fakeAct) Create(_ context.Context, c activitiessvc.CreateCmd) (*activitiesdomain.Activity, error) {
	f.cmd = c
	if f.err != nil {
		return nil, f.err
	}
	return &activitiesdomain.Activity{ID: 7, Title: c.Title, Status: activitiesdomain.StatusPublished}, nil
}

type fakeSched struct{ cmd schedulesvc.CreateCmd }

func (f *fakeSched) Create(_ context.Context, c schedulesvc.CreateCmd) (*scheduledomain.Schedule, error) {
	f.cmd = c
	return &scheduledomain.Schedule{ID: 11, Status: scheduledomain.StatusPublished, CalendarEventID: "evt-1", ResultNote: "日历OK:evt-1"}, nil
}

type fakeUsers struct{ users []usersdomain.User }

func (f *fakeUsers) List(_ context.Context, _ int64) ([]usersdomain.User, error) { return f.users, nil }

func TestPublish_AllAttendees(t *testing.T) {
	act := &fakeAct{}
	sched := &fakeSched{}
	users := &fakeUsers{users: []usersdomain.User{
		{DingUserID: "u1"}, {DingUserID: ""}, {DingUserID: "u2"},
	}}
	s := New(act, sched, users)
	now := time.Now()
	res := s.Publish(context.Background(), Cmd{
		TenantID: 1, Title: "团队分享会", DimensionCode: "team",
		StartAt: now, EndAt: now.Add(time.Hour),
		RoomIDs: []string{"r1"}, AttendeeAll: true,
	})
	require.Equal(t, int64(7), res.ActivityID)
	require.Equal(t, "evt-1", res.CalendarEventID)
	require.Equal(t, 2, res.AttendeeCount) // 空 ding_user_id 被过滤掉
	require.Equal(t, []string{"u1", "u2"}, sched.cmd.AttendeeUserIDs)
	require.Equal(t, []string{"r1"}, sched.cmd.RoomIDs)
	require.True(t, sched.cmd.PushCalendar)
	require.Len(t, res.Stages, 2)
	require.True(t, res.Stages[0].OK && res.Stages[1].OK)
}

func TestPublish_ExplicitAttendees(t *testing.T) {
	act := &fakeAct{}
	sched := &fakeSched{}
	s := New(act, sched, &fakeUsers{})
	now := time.Now()
	res := s.Publish(context.Background(), Cmd{
		TenantID: 1, Title: "小范围评审", DimensionCode: "team",
		StartAt: now, EndAt: now.Add(time.Hour),
		AttendeeAll: false, AttendeeUserIDs: []string{"x1", "x2", "x3"},
	})
	require.Equal(t, 3, res.AttendeeCount)
	require.Equal(t, []string{"x1", "x2", "x3"}, sched.cmd.AttendeeUserIDs)
}

func TestPublish_SkipSchedule(t *testing.T) {
	act := &fakeAct{}
	sched := &fakeSched{}
	users := &fakeUsers{users: []usersdomain.User{{DingUserID: "u1"}, {DingUserID: "u2"}}}
	s := New(act, sched, users)
	now := time.Now()
	res := s.Publish(context.Background(), Cmd{
		TenantID: 1, Title: "只发活动不建日程", DimensionCode: "team",
		StartAt: now, EndAt: now.Add(time.Hour),
		AttendeeAll: true, SkipSchedule: true,
	})
	require.Equal(t, int64(7), res.ActivityID)
	require.True(t, res.ScheduleSkipped)
	require.Empty(t, res.CalendarEventID)
	require.Equal(t, 0, res.AttendeeCount)     // 没去解析参与人
	require.Len(t, res.Stages, 1)              // 只有 create_activity 一个阶段
	require.True(t, res.Stages[0].OK)
	require.Empty(t, sched.cmd.Title)          // schedule.Create 完全没被调用
}

func TestPublish_ActivityErrorShortCircuits(t *testing.T) {
	act := &fakeAct{err: activitiessvc.ErrInvalidDimension}
	sched := &fakeSched{}
	s := New(act, sched, &fakeUsers{})
	now := time.Now()
	res := s.Publish(context.Background(), Cmd{TenantID: 1, Title: "x", DimensionCode: "bad", StartAt: now, EndAt: now})
	require.Equal(t, int64(0), res.ActivityID)
	require.Len(t, res.Stages, 1)
	require.False(t, res.Stages[0].OK)
	require.Empty(t, sched.cmd.Title) // schedule 未被调用
}
