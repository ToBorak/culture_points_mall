package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClaudeStream_TextDeltas(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fl := w.(http.Flusher)
		write := func(event, data string) {
			_, _ = w.Write([]byte("event: " + event + "\n"))
			_, _ = w.Write([]byte("data: " + data + "\n\n"))
			fl.Flush()
		}
		write("message_start", `{}`)
		write("content_block_start", `{"index":0,"content_block":{"type":"text"}}`)
		write("content_block_delta", `{"index":0,"delta":{"type":"text_delta","text":"你好"}}`)
		write("content_block_delta", `{"index":0,"delta":{"type":"text_delta","text":"世界"}}`)
		write("content_block_stop", `{"index":0}`)
		write("message_delta", `{"delta":{"stop_reason":"end_turn"}}`)
		write("message_stop", `{}`)
	}))
	defer srv.Close()

	c := NewClaude("k", srv.URL, "claude-sonnet-4-7")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ch, err := c.MessagesStream(ctx, MessagesRequest{Messages: []Message{{Role: RoleUser, Content: []Block{{Type: "text", Text: "ping"}}}}})
	require.NoError(t, err)
	var got string
	var stop StopReason
	for ev := range ch {
		if ev.Delta != "" {
			got += ev.Delta
		}
		if ev.StopReason != "" {
			stop = ev.StopReason
		}
	}
	require.Equal(t, "你好世界", got)
	require.Equal(t, StopEnd, stop)
}
