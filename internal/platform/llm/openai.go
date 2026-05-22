package llm

import (
	"context"
	"errors"
)

type OpenAIClient struct {
	APIKey  string
	BaseURL string
	Model   string
}

var ErrNotImplemented = errors.New("provider implementation not in this Phase")

func (c *OpenAIClient) Messages(_ context.Context, _ MessagesRequest) (MessagesResponse, error) {
	return MessagesResponse{}, ErrNotImplemented
}

func (c *OpenAIClient) MessagesStream(_ context.Context, _ MessagesRequest) (<-chan StreamEvent, error) {
	return nil, ErrNotImplemented
}
