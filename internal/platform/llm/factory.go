package llm

import (
	"fmt"
	"strings"

	"github.com/standardsoftware/culture_points_mall/internal/config"
)

func NewFromConfig(cfg *config.Config) (Client, error) {
	switch cfg.LLM.Provider {
	case "claude":
		return NewClaude(cfg.LLM.Claude.APIKey, cfg.LLM.Claude.BaseURL, cfg.LLM.Claude.Model), nil
	case "openai":
		return NewOpenAI(cfg.LLM.OpenAI.APIKey, cfg.LLM.OpenAI.BaseURL, cfg.LLM.OpenAI.Model), nil
	case "deepseek":
		base := cfg.LLM.DeepSeek.BaseURL
		if base == "" {
			base = "https://api.deepseek.com"
		}
		// DeepSeek 用 /v1 前缀的 OpenAI 兼容端点
		if !strings.HasSuffix(base, "/v1") {
			base = strings.TrimRight(base, "/") + "/v1"
		}
		model := cfg.LLM.DeepSeek.Model
		if model == "" {
			model = "deepseek-chat"
		}
		return NewOpenAI(cfg.LLM.DeepSeek.APIKey, base, model), nil
	case "qwen":
		base := cfg.LLM.Qwen.BaseURL
		if base == "" {
			base = "https://dashscope.aliyuncs.com/compatible-mode/v1"
		}
		return NewOpenAI(cfg.LLM.Qwen.APIKey, base, cfg.LLM.Qwen.Model), nil
	default:
		return nil, fmt.Errorf("llm provider %q not configured", cfg.LLM.Provider)
	}
}
