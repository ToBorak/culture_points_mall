package llm

import "context"

type Client interface {
	Messages(ctx context.Context, req MessagesRequest) (MessagesResponse, error)
	MessagesStream(ctx context.Context, req MessagesRequest) (<-chan StreamEvent, error)
}
