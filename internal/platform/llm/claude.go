package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ClaudeClient struct {
	APIKey  string
	BaseURL string
	Model   string
	HTTPC   *http.Client
}

func NewClaude(apiKey, baseURL, model string) *ClaudeClient {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	if model == "" {
		model = "claude-sonnet-4-7"
	}
	return &ClaudeClient{
		APIKey: apiKey, BaseURL: baseURL, Model: model,
		HTTPC: &http.Client{Timeout: 120 * time.Second},
	}
}

type claudeReq struct {
	Model     string    `json:"model"`
	System    string    `json:"system,omitempty"`
	Messages  []Message `json:"messages"`
	Tools     []ToolDef `json:"tools,omitempty"`
	MaxTokens int       `json:"max_tokens"`
	Stream    bool      `json:"stream,omitempty"`
}

func (c *ClaudeClient) Messages(ctx context.Context, req MessagesRequest) (MessagesResponse, error) {
	if req.MaxTokens == 0 {
		req.MaxTokens = 4096
	}
	body, _ := json.Marshal(claudeReq{
		Model: c.Model, System: req.System, Messages: req.Messages,
		Tools: req.Tools, MaxTokens: req.MaxTokens,
	})
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/v1/messages", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("x-api-key", c.APIKey)

	resp, err := c.HTTPC.Do(httpReq)
	if err != nil {
		return MessagesResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return MessagesResponse{}, fmt.Errorf("claude %d: %s", resp.StatusCode, string(raw))
	}
	var out MessagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return MessagesResponse{}, err
	}
	return out, nil
}

func (c *ClaudeClient) MessagesStream(ctx context.Context, req MessagesRequest) (<-chan StreamEvent, error) {
	req.Stream = true
	if req.MaxTokens == 0 {
		req.MaxTokens = 4096
	}
	body, _ := json.Marshal(claudeReq{
		Model: c.Model, System: req.System, Messages: req.Messages,
		Tools: req.Tools, MaxTokens: req.MaxTokens, Stream: true,
	})
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/v1/messages", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("x-api-key", c.APIKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.HTTPC.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("claude stream %d: %s", resp.StatusCode, string(raw))
	}

	ch := make(chan StreamEvent, 32)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		var event string
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case strings.HasPrefix(line, "event:"):
				event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			case strings.HasPrefix(line, "data:"):
				data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				if event == "" || data == "" {
					continue
				}
				ch <- decodeSSE(event, data)
				event = ""
			}
		}
		if err := scanner.Err(); err != nil {
			ch <- StreamEvent{Type: "error", Err: err}
		}
	}()
	return ch, nil
}

func decodeSSE(event, data string) StreamEvent {
	switch event {
	case "message_start":
		return StreamEvent{Type: "message_start"}
	case "content_block_start":
		var d struct {
			Index        int `json:"index"`
			ContentBlock struct {
				Type  string         `json:"type"`
				ID    string         `json:"id,omitempty"`
				Name  string         `json:"name,omitempty"`
				Input map[string]any `json:"input,omitempty"`
			} `json:"content_block"`
		}
		_ = json.Unmarshal([]byte(data), &d)
		if d.ContentBlock.Type == "tool_use" {
			return StreamEvent{
				Type:       "content_block_start",
				BlockIndex: d.Index,
				ToolUse:    &ToolUse{ID: d.ContentBlock.ID, Name: d.ContentBlock.Name, Input: d.ContentBlock.Input},
			}
		}
		return StreamEvent{Type: "content_block_start", BlockIndex: d.Index}
	case "content_block_delta":
		var d struct {
			Index int `json:"index"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				PartialJSON string `json:"partial_json"`
			} `json:"delta"`
		}
		_ = json.Unmarshal([]byte(data), &d)
		if d.Delta.Type == "input_json_delta" {
			return StreamEvent{Type: "content_block_delta", BlockIndex: d.Index, Delta: d.Delta.PartialJSON}
		}
		return StreamEvent{Type: "content_block_delta", BlockIndex: d.Index, Delta: d.Delta.Text}
	case "content_block_stop":
		var d struct {
			Index int `json:"index"`
		}
		_ = json.Unmarshal([]byte(data), &d)
		return StreamEvent{Type: "content_block_stop", BlockIndex: d.Index}
	case "message_delta":
		var d struct {
			Delta struct {
				StopReason string `json:"stop_reason"`
			} `json:"delta"`
		}
		_ = json.Unmarshal([]byte(data), &d)
		return StreamEvent{Type: "message_delta", StopReason: StopReason(d.Delta.StopReason)}
	case "message_stop":
		return StreamEvent{Type: "message_stop"}
	case "error":
		return StreamEvent{Type: "error", Err: errors.New(data)}
	default:
		return StreamEvent{Type: event}
	}
}
