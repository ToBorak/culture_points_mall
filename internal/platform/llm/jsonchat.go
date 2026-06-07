package llm

import (
	"context"
	"errors"
	"strings"
)

// MessagesText 调用 LLM 返回纯文本（拼接所有 text block）。供问答/草稿等非 JSON 场景。
func MessagesText(ctx context.Context, c Client, system, user string, maxTokens int) (string, error) {
	if maxTokens <= 0 {
		maxTokens = 1024
	}
	resp, err := c.Messages(ctx, MessagesRequest{
		System:    system,
		Messages:  []Message{{Role: RoleUser, Content: []Block{{Type: "text", Text: user}}}},
		MaxTokens: maxTokens,
	})
	if err != nil {
		return "", err
	}
	var text string
	for _, b := range resp.Content {
		if b.Type == "text" {
			text += b.Text
		}
	}
	return strings.TrimSpace(text), nil
}

// MessagesJSON 调用 LLM 并清洗出 JSON 字符串：剥 ```json 代码块、截首个 { 到末个 }。
// 与 insights.callLLMJSON 同款，抽出供各模块复用。
func MessagesJSON(ctx context.Context, c Client, system, user string, maxTokens int) (string, error) {
	text, err := MessagesText(ctx, c, system+"\n\n严格输出 JSON，不要包代码块，不要多余文字。", user, maxTokens)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)
	}
	if first := strings.Index(text, "{"); first >= 0 {
		if last := strings.LastIndex(text, "}"); last > first {
			text = text[first : last+1]
		}
	}
	if text == "" {
		return "", errors.New("llm returned empty json")
	}
	return text, nil
}
