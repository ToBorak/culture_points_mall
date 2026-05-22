package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeTool struct{}

func (fakeTool) Name() string                { return "echo" }
func (fakeTool) Description() string         { return "echo back" }
func (fakeTool) InputSchema() map[string]any { return map[string]any{"type": "object"} }
func (fakeTool) Execute(_ context.Context, in map[string]any) (map[string]any, error) {
	return in, nil
}

func TestRegistry_Call(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(fakeTool{})
	res := r.Call(context.Background(), "echo", map[string]any{"msg": "hi"})
	require.False(t, res.IsError)
	require.Equal(t, "hi", res.Output["msg"])
}

func TestRegistry_DuplicateRegister(t *testing.T) {
	r := NewRegistry()
	require.NoError(t, r.Register(fakeTool{}))
	require.Error(t, r.Register(fakeTool{}))
}

func TestRegistry_NotFound(t *testing.T) {
	r := NewRegistry()
	res := r.Call(context.Background(), "unknown", nil)
	require.True(t, res.IsError)
}
