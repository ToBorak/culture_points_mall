//go:build integration

package mcp

import (
	"bufio"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/standardsoftware/culture_points_mall/internal/modules/agent/tools"
)

type echoTool struct{}

func (echoTool) Name() string                { return "echo" }
func (echoTool) Description() string         { return "echo back input" }
func (echoTool) InputSchema() map[string]any { return map[string]any{"type": "object"} }
func (echoTool) Execute(_ context.Context, in map[string]any) (map[string]any, error) {
	return in, nil
}

func TestMCP_ListAndCall_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reg := tools.NewRegistry()
	reg.MustRegister(echoTool{})
	srv := NewServer(reg, func(string) (int64, bool) { return 1, true })

	r := gin.New()
	srv.Register(r)
	ts := httptest.NewServer(r)
	defer ts.Close()

	// SSE 订阅，等待至少 2 条 data: 行（initialize + tools/call 各一条 response）
	var wg sync.WaitGroup
	wg.Add(1)
	got := 0
	go func() {
		defer wg.Done()
		req, _ := http.NewRequest("GET", ts.URL+"/mcp/sse?session=s1", nil)
		req.Header.Set("Authorization", "Bearer x")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data:") {
				got++
				if got >= 2 {
					return
				}
			}
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()

	// 给 SSE 订阅一点时间建立
	time.Sleep(200 * time.Millisecond)

	// initialize
	body := []byte(`{"jsonrpc":"2.0","id":"1","method":"initialize"}`)
	req, _ := http.NewRequest("POST", ts.URL+"/mcp/messages?session=s1", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer x")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, 202, resp.StatusCode)

	// tools/call echo
	body = []byte(`{"jsonrpc":"2.0","id":"2","method":"tools/call","params":{"name":"echo","arguments":{"hi":1}}}`)
	req, _ = http.NewRequest("POST", ts.URL+"/mcp/messages?session=s1", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer x")
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, 202, resp.StatusCode)

	wg.Wait()
	require.GreaterOrEqual(t, got, 2)
}
