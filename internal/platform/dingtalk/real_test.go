package dingtalk

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/standardsoftware/culture_points_mall/internal/config"
)

func TestRealClient_GetUserByCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1.0/oauth2/accessToken":
			_, _ = w.Write([]byte(`{"accessToken":"AT","expireIn":7200}`))
		case "/topapi/v2/user/getuserinfo":
			b, _ := io.ReadAll(r.Body)
			require.Contains(t, string(b), `"code123"`)
			_, _ = w.Write([]byte(`{"errcode":0,"result":{"userid":"u100","unionid":"un100","sys":true}}`))
		case "/topapi/v2/user/get":
			_, _ = w.Write([]byte(`{"errcode":0,"result":{"userid":"u100","unionid":"un100","name":"张三","avatar":"http://a/x.png","dept_id_list":[10,20]}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	c := NewReal(config.DingTalkCfg{AppKey: "ak", AppSecret: "as"}, rdb)
	c.api.oapiBase = srv.URL
	c.api.apiBase = srv.URL

	u, err := c.GetUserByCode(context.Background(), "code123")
	require.NoError(t, err)
	require.Equal(t, "u100", u.DingUserID)
	require.Equal(t, "张三", u.Name)
	require.Equal(t, "http://a/x.png", u.AvatarURL)
	require.Equal(t, []int64{10, 20}, u.DeptIDs)
	require.Equal(t, "un100", u.UnionID)
	require.True(t, u.IsAdmin)
}

func TestRealClient_QueryMeetingRooms(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1.0/oauth2/accessToken":
			_, _ = w.Write([]byte(`{"accessToken":"AT","expireIn":7200}`))
		case "/v1.0/rooms/meetingRoomLists":
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "un-admin", r.URL.Query().Get("unionId"))
			require.Equal(t, "100", r.URL.Query().Get("maxResults"))
			require.Equal(t, "AT", r.Header.Get("x-acs-dingtalk-access-token"))
			_, _ = w.Write([]byte(`{"hasMore":false,"result":[
				{"roomId":"r1","roomName":"三楼大会议室","roomCapacity":20,"roomStatus":1,"roomLocation":{"title":"总部","desc":"3F"}},
				{"roomId":"r2","roomName":"洽谈室","roomCapacity":6,"roomStatus":1,"roomLocation":{"title":"总部","desc":""}}
			]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	c := NewReal(config.DingTalkCfg{AppKey: "ak", AppSecret: "as"}, rdb)
	c.api.oapiBase = srv.URL
	c.api.apiBase = srv.URL

	rooms, err := c.QueryMeetingRooms(context.Background(), "un-admin")
	require.NoError(t, err)
	require.Len(t, rooms, 2)
	require.Equal(t, "r1", rooms[0].RoomID)
	require.Equal(t, "三楼大会议室", rooms[0].RoomName)
	require.Equal(t, 20, rooms[0].Capacity)
	require.Equal(t, "总部 3F", rooms[0].Location)
	require.Equal(t, "总部", rooms[1].Location) // desc 为空时只剩 title
}

func TestRealClient_CreateCalendarEvent_WithRooms(t *testing.T) {
	var roomBody string
	hitMeetingRooms := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1.0/oauth2/accessToken":
			_, _ = w.Write([]byte(`{"accessToken":"AT","expireIn":7200}`))
		case r.URL.Path == "/topapi/v2/user/get":
			_, _ = w.Write([]byte(`{"errcode":0,"result":{"unionid":"u-union"}}`))
		case strings.HasSuffix(r.URL.Path, "/meetingRooms"):
			hitMeetingRooms = true
			require.Equal(t, "/v1.0/calendar/users/org-union/calendars/primary/events/evt-1/meetingRooms", r.URL.Path)
			b, _ := io.ReadAll(r.Body)
			roomBody = string(b)
			_, _ = w.Write([]byte(`{"result":true}`))
		case strings.HasSuffix(r.URL.Path, "/events"):
			require.Equal(t, "/v1.0/calendar/users/org-union/calendars/primary/events", r.URL.Path)
			_, _ = w.Write([]byte(`{"id":"evt-1"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	c := NewReal(config.DingTalkCfg{AppKey: "ak", AppSecret: "as", CalendarOrganizerUnionID: "org-union"}, rdb)
	c.api.oapiBase = srv.URL
	c.api.apiBase = srv.URL

	eventID, err := c.CreateCalendarEvent(context.Background(), CalendarRequest{
		Title: "周会", StartAt: time.Now(), EndAt: time.Now().Add(time.Hour),
		UserIDs: []string{"u1"}, RoomIDs: []string{"r1", "r2"},
	})
	require.NoError(t, err)
	require.Equal(t, "evt-1", eventID)
	require.True(t, hitMeetingRooms, "应调用加会议室接口")
	require.Contains(t, roomBody, "meetingRoomsToAdd")
	require.Contains(t, roomBody, `"roomId":"r1"`)
	require.Contains(t, roomBody, `"roomId":"r2"`)
}

// 组织者应是操作者本人：即使配了 CalendarOrganizerUnionID，传入 OrganizerUserID 时也优先用操作者解析出的 unionId。
func TestRealClient_CreateCalendarEvent_OrganizerIsOperator(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1.0/oauth2/accessToken":
			_, _ = w.Write([]byte(`{"accessToken":"AT","expireIn":7200}`))
		case r.URL.Path == "/topapi/v2/user/get":
			b, _ := io.ReadAll(r.Body)
			if strings.Contains(string(b), "boss") {
				_, _ = w.Write([]byte(`{"errcode":0,"result":{"unionid":"boss-union"}}`))
			} else {
				_, _ = w.Write([]byte(`{"errcode":0,"result":{"unionid":"att-union"}}`))
			}
		case strings.HasSuffix(r.URL.Path, "/events"):
			require.Equal(t, "/v1.0/calendar/users/boss-union/calendars/primary/events", r.URL.Path)
			_, _ = w.Write([]byte(`{"id":"evt-1"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	// 故意配上一个不同的组织者 unionId，验证操作者优先级高于配置。
	c := NewReal(config.DingTalkCfg{AppKey: "ak", AppSecret: "as", CalendarOrganizerUnionID: "cfg-union"}, rdb)
	c.api.oapiBase = srv.URL
	c.api.apiBase = srv.URL

	eventID, err := c.CreateCalendarEvent(context.Background(), CalendarRequest{
		Title: "周会", StartAt: time.Now(), EndAt: time.Now().Add(time.Hour),
		UserIDs: []string{"att1"}, OrganizerUserID: "boss",
	})
	require.NoError(t, err)
	require.Equal(t, "evt-1", eventID)
}

func TestRealClient_UnimplementedReturnsErr(t *testing.T) {
	c := NewReal(config.DingTalkCfg{}, nil)
	ctx := context.Background()
	_, err := c.ListCalendarResponses(ctx, "evt")
	require.ErrorIs(t, err, errNotImplemented)
	require.ErrorIs(t, c.SendWorkNotice(ctx, nil, Card{}), errNotImplemented)
	_, err = c.SendInteractiveCard(ctx, "t", "tpl", nil)
	require.ErrorIs(t, err, errNotImplemented)
	_, err = c.StartOAProcess(ctx, ApprovalRequest{})
	require.ErrorIs(t, err, errNotImplemented)
}
