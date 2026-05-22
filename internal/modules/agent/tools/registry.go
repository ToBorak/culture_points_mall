package tools

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

func NewRegistry() *Registry { return &Registry{tools: make(map[string]Tool)} }

func (r *Registry) Register(t Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[t.Name()]; exists {
		return fmt.Errorf("tool already registered: %s", t.Name())
	}
	r.tools[t.Name()] = t
	return nil
}

func (r *Registry) MustRegister(t Tool) {
	if err := r.Register(t); err != nil {
		panic(err)
	}
}

func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	return out
}

func (r *Registry) Call(ctx context.Context, name string, input map[string]any) Result {
	t, ok := r.Get(name)
	if !ok {
		return Result{IsError: true, Message: fmt.Sprintf("tool not found: %s", name)}
	}
	out, err := t.Execute(ctx, input)
	if err != nil {
		return Result{IsError: true, Message: err.Error()}
	}
	return Result{Output: out}
}

var ErrToolNotFound = errors.New("tool not found")
