package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClaudeClient_Messages_MockServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/messages", r.URL.Path)
		require.NotEmpty(t, r.Header.Get("x-api-key"))
		body := MessagesResponse{
			Content:    []Block{{Type: "text", Text: "你好"}},
			StopReason: StopEnd,
			Model:      "claude-sonnet-4-7",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer srv.Close()

	c := NewClaude("test-key", srv.URL, "claude-sonnet-4-7")
	resp, err := c.Messages(context.Background(), MessagesRequest{
		Messages: []Message{{Role: RoleUser, Content: []Block{{Type: "text", Text: "ping"}}}},
	})
	require.NoError(t, err)
	require.Len(t, resp.Content, 1)
	require.Equal(t, "你好", resp.Content[0].Text)
	require.Equal(t, StopEnd, resp.StopReason)
}
