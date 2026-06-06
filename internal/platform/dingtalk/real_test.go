package dingtalk

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

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

func TestRealClient_UnimplementedReturnsErr(t *testing.T) {
	c := NewReal(config.DingTalkCfg{}, nil)
	_, err := c.ListCalendarResponses(context.Background(), "event-1")
	require.ErrorIs(t, err, errNotImplemented)
	require.Error(t, c.BotBroadcast(context.Background(), "g", Card{}))
}
