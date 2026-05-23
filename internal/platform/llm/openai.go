package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAIClient 实现 OpenAI 兼容协议（OpenAI / DeepSeek / Qwen / Moonshot 等通用）
type OpenAIClient struct {
	APIKey  string
	BaseURL string
	Model   string
	HTTPC   *http.Client
}

func NewOpenAI(apiKey, baseURL, model string) *OpenAIClient {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &OpenAIClient{
		APIKey: apiKey, BaseURL: baseURL, Model: model,
		HTTPC: &http.Client{Timeout: 120 * time.Second},
	}
}

// ---- OpenAI wire format ----

type oaToolFn struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
}
type oaTool struct {
	Type     string   `json:"type"`
	Function oaToolFn `json:"function"`
}

type oaToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type oaMsg struct {
	Role       string       `json:"role"`
	Content    *string      `json:"content"` // 显式存在，可为 null，DeepSeek 要求 content 字段必须显式呈现
	ToolCalls  []oaToolCall `json:"tool_calls,omitempty"`
	ToolCallID string       `json:"tool_call_id,omitempty"`
	Name       string       `json:"name,omitempty"`
}

func strPtr(s string) *string { return &s }

type oaReq struct {
	Model       string   `json:"model"`
	Messages    []oaMsg  `json:"messages"`
	Tools       []oaTool `json:"tools,omitempty"`
	MaxTokens   int      `json:"max_tokens,omitempty"`
	Temperature float64  `json:"temperature,omitempty"`
	Stream      bool     `json:"stream,omitempty"`
}

type oaResp struct {
	Choices []struct {
		Index        int    `json:"index"`
		Message      oaMsg  `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Model string `json:"model"`
}

// ---- conversion: internal Message -> OpenAI msg list ----

func toOpenAIMessages(system string, msgs []Message) []oaMsg {
	out := make([]oaMsg, 0, len(msgs)+1)
	if system != "" {
		out = append(out, oaMsg{Role: "system", Content: strPtr(system)})
	}
	for _, m := range msgs {
		switch m.Role {
		case RoleUser:
			// Anthropic 风格里 tool_result 块被嵌在 user 消息里，需要拆成 OpenAI 的 tool 消息
			var text string
			var toolMsgs []oaMsg
			for _, b := range m.Content {
				switch b.Type {
				case "text":
					text += b.Text
				case "tool_result":
					if b.ToolRes == nil {
						continue
					}
					content := b.ToolRes.Content
					if b.ToolRes.IsError {
						content = "ERROR: " + content
					}
					toolMsgs = append(toolMsgs, oaMsg{
						Role:       "tool",
						Content:    strPtr(content),
						ToolCallID: b.ToolRes.ToolUseID,
					})
				}
			}
			if text != "" {
				out = append(out, oaMsg{Role: "user", Content: strPtr(text)})
			}
			out = append(out, toolMsgs...)
		case RoleAssistant:
			var text string
			var calls []oaToolCall
			for _, b := range m.Content {
				switch b.Type {
				case "text":
					text += b.Text
				case "tool_use":
					if b.ToolUse == nil {
						continue
					}
					raw, _ := json.Marshal(b.ToolUse.Input)
					var call oaToolCall
					call.ID = b.ToolUse.ID
					call.Type = "function"
					call.Function.Name = b.ToolUse.Name
					call.Function.Arguments = string(raw)
					calls = append(calls, call)
				}
			}
			// 当只有 tool_calls 没文本时，content 必须为 null
			msg := oaMsg{Role: "assistant", ToolCalls: calls}
			if text != "" {
				msg.Content = strPtr(text)
			} else if len(calls) == 0 {
				msg.Content = strPtr("")
			}
			// else: leave Content as nil pointer → encoded as `"content": null`
			out = append(out, msg)
		case RoleTool:
			for _, b := range m.Content {
				if b.Type != "tool_result" || b.ToolRes == nil {
					continue
				}
				content := b.ToolRes.Content
				if b.ToolRes.IsError {
					content = "ERROR: " + content
				}
				out = append(out, oaMsg{
					Role:       "tool",
					Content:    strPtr(content),
					ToolCallID: b.ToolRes.ToolUseID,
				})
			}
		case RoleSystem:
			out = append(out, oaMsg{Role: "system", Content: strPtr(blocksText(m.Content))})
		}
	}
	return out
}

func blocksText(blocks []Block) string {
	var s string
	for _, b := range blocks {
		if b.Type == "text" {
			s += b.Text
		}
	}
	return s
}

func toOpenAITools(defs []ToolDef) []oaTool {
	if len(defs) == 0 {
		return nil
	}
	out := make([]oaTool, 0, len(defs))
	for _, d := range defs {
		out = append(out, oaTool{
			Type: "function",
			Function: oaToolFn{
				Name:        d.Name,
				Description: d.Description,
				Parameters:  d.InputSchema,
			},
		})
	}
	return out
}

// ---- Messages ----

func (c *OpenAIClient) Messages(ctx context.Context, req MessagesRequest) (MessagesResponse, error) {
	if req.MaxTokens == 0 {
		req.MaxTokens = 4096
	}
	payload := oaReq{
		Model:     c.Model,
		Messages:  toOpenAIMessages(req.System, req.Messages),
		Tools:     toOpenAITools(req.Tools),
		MaxTokens: req.MaxTokens,
	}
	body, _ := json.Marshal(payload)
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/chat/completions", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTPC.Do(httpReq)
	if err != nil {
		return MessagesResponse{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return MessagesResponse{}, fmt.Errorf("openai %d: %s", resp.StatusCode, string(raw))
	}
	var out oaResp
	if err := json.Unmarshal(raw, &out); err != nil {
		return MessagesResponse{}, fmt.Errorf("openai decode: %w (raw: %s)", err, string(raw))
	}
	if len(out.Choices) == 0 {
		return MessagesResponse{}, fmt.Errorf("openai: empty choices")
	}
	choice := out.Choices[0]
	blocks := make([]Block, 0, 2)
	if choice.Message.Content != nil && *choice.Message.Content != "" {
		blocks = append(blocks, Block{Type: "text", Text: *choice.Message.Content})
	}
	for _, call := range choice.Message.ToolCalls {
		var input map[string]any
		_ = json.Unmarshal([]byte(call.Function.Arguments), &input)
		if input == nil {
			input = map[string]any{}
		}
		blocks = append(blocks, Block{
			Type:    "tool_use",
			ToolUse: &ToolUse{ID: call.ID, Name: call.Function.Name, Input: input},
		})
	}
	stop := StopEnd
	switch choice.FinishReason {
	case "tool_calls":
		stop = StopTool
	case "length":
		stop = StopMax
	case "stop":
		stop = StopEnd
	}
	return MessagesResponse{
		Content:    blocks,
		StopReason: stop,
		Model:      out.Model,
		Usage: struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		}{InputTokens: out.Usage.PromptTokens, OutputTokens: out.Usage.CompletionTokens},
	}, nil
}

// MessagesStream 暂未实现 SSE 解析；orchestrator 当前仅使用非流式 Messages。
func (c *OpenAIClient) MessagesStream(_ context.Context, _ MessagesRequest) (<-chan StreamEvent, error) {
	return nil, ErrNotImplemented
}

var ErrNotImplemented = fmt.Errorf("openai stream not implemented")
