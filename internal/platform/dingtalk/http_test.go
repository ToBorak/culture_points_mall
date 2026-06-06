package dingtalk

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCaller_OapiPost_TokenInQueryAndErrcode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/topapi/demo", r.URL.Path)
		require.Equal(t, "tok123", r.URL.Query().Get("access_token"))
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","result":{"v":7}}`))
	}))
	defer srv.Close()

	c := newCaller()
	c.oapiBase = srv.URL
	var out struct {
		Result struct{ V int `json:"v"` } `json:"result"`
	}
	require.NoError(t, c.oapiPost(context.Background(), "/topapi/demo", "tok123", map[string]any{"a": 1}, &out))
	require.Equal(t, 7, out.Result.V)
}

func TestCaller_OapiPost_ErrcodeNonZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errcode":40001,"errmsg":"invalid token"}`))
	}))
	defer srv.Close()
	c := newCaller()
	c.oapiBase = srv.URL
	err := c.oapiPost(context.Background(), "/topapi/demo", "bad", map[string]any{}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "40001")
}

func TestCaller_ApiPost_HeaderTokenAndStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "utok", r.Header.Get("x-acs-dingtalk-access-token"))
		_, _ = w.Write([]byte(`{"accessToken":"AT","expireIn":7200}`))
	}))
	defer srv.Close()
	c := newCaller()
	c.apiBase = srv.URL
	var out struct {
		AccessToken string `json:"accessToken"`
	}
	require.NoError(t, c.apiPost(context.Background(), "/v1.0/x", "utok", map[string]any{}, &out))
	require.Equal(t, "AT", out.AccessToken)
}

func TestCaller_ApiPost_Non2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"code":"unauthorized"}`))
	}))
	defer srv.Close()
	c := newCaller()
	c.apiBase = srv.URL
	err := c.apiPost(context.Background(), "/v1.0/x", "", map[string]any{}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
}

func TestCaller_OapiPost_StatusErrorRedactsToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`oops`))
	}))
	defer srv.Close()
	c := newCaller()
	c.oapiBase = srv.URL
	err := c.oapiPost(context.Background(), "/topapi/demo", "supersecret", map[string]any{}, nil)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "supersecret")
}
