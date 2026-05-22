package llm

import (
	"fmt"

	"github.com/standardsoftware/culture_points_mall/internal/config"
)

func NewFromConfig(cfg *config.Config) (Client, error) {
	switch cfg.LLM.Provider {
	case "claude":
		return NewClaude(cfg.LLM.Claude.APIKey, cfg.LLM.Claude.BaseURL, cfg.LLM.Claude.Model), nil
	case "openai":
		return &OpenAIClient{APIKey: cfg.LLM.OpenAI.APIKey, BaseURL: cfg.LLM.OpenAI.BaseURL, Model: cfg.LLM.OpenAI.Model}, nil
	default:
		return nil, fmt.Errorf("llm provider %q not configured", cfg.LLM.Provider)
	}
}
