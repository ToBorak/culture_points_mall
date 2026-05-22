package tools

import "context"

type Tool interface {
	Name() string
	Description() string
	InputSchema() map[string]any
	Execute(ctx context.Context, input map[string]any) (output map[string]any, err error)
}

type Result struct {
	Output  map[string]any
	IsError bool
	Message string
}
