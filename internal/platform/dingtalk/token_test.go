package dingtalk

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestTokenManager_FetchThenCache(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1.0/oauth2/accessToken", r.URL.Path)
		atomic.AddInt32(&hits, 1)
		_, _ = w.Write([]byte(`{"accessToken":"AT-1","expireIn":7200}`))
	}))
	defer srv.Close()

	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	cl := newCaller()
	cl.apiBase = srv.URL
	tm := &tokenManager{api: cl, rdb: rdb, appKey: "ak", appSecret: "as"}

	tok, err := tm.corpToken(context.Background())
	require.NoError(t, err)
	require.Equal(t, "AT-1", tok)

	tok2, err := tm.corpToken(context.Background())
	require.NoError(t, err)
	require.Equal(t, "AT-1", tok2)
	require.Equal(t, int32(1), atomic.LoadInt32(&hits), "第二次应命中缓存不再请求")
}
