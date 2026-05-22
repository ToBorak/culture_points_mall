package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/standardsoftware/culture_points_mall/internal/modules/agent/tools"
	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

type Server struct {
	Registry *tools.Registry
	Auth     func(token string) (tenantID int64, ok bool)

	mu     sync.Mutex
	queues map[string]chan Response
}

func NewServer(reg *tools.Registry, auth func(string) (int64, bool)) *Server {
	return &Server{Registry: reg, Auth: auth, queues: make(map[string]chan Response)}
}

func (s *Server) Register(r *gin.Engine) {
	r.GET("/mcp/sse", s.sse)
	r.POST("/mcp/messages", s.message)
}

func (s *Server) auth(c *gin.Context) (int64, bool) {
	token := c.GetHeader("Authorization")
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}
	return s.Auth(token)
}

func (s *Server) sse(c *gin.Context) {
	_, ok := s.auth(c)
	if !ok {
		c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	sessionID := c.Query("session")
	if sessionID == "" {
		c.AbortWithStatusJSON(400, gin.H{"error": "session required"})
		return
	}
	s.mu.Lock()
	q, exists := s.queues[sessionID]
	if !exists {
		q = make(chan Response, 16)
		s.queues[sessionID] = q
	}
	s.mu.Unlock()

	notify := c.Writer.CloseNotify()
	for {
		select {
		case resp := <-q:
			raw, _ := json.Marshal(resp)
			_, _ = c.Writer.Write([]byte("event: message\ndata: "))
			_, _ = c.Writer.Write(raw)
			_, _ = c.Writer.Write([]byte("\n\n"))
			c.Writer.Flush()
		case <-notify:
			s.mu.Lock()
			delete(s.queues, sessionID)
			s.mu.Unlock()
			return
		case <-time.After(30 * time.Second):
			_, _ = c.Writer.Write([]byte(": keepalive\n\n"))
			c.Writer.Flush()
		}
	}
}

func (s *Server) message(c *gin.Context) {
	tid, ok := s.auth(c)
	if !ok {
		c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
		return
	}
	sessionID := c.Query("session")
	if sessionID == "" {
		c.AbortWithStatusJSON(400, gin.H{"error": "session required"})
		return
	}
	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(400, gin.H{"error": err.Error()})
		return
	}

	ctx := cpmctx.WithTenant(c.Request.Context(), tid)
	resp := s.dispatch(ctx, req)

	s.mu.Lock()
	q := s.queues[sessionID]
	s.mu.Unlock()
	if q != nil {
		q <- resp
	}
	c.Status(http.StatusAccepted)
}

func (s *Server) dispatch(ctx context.Context, req Request) Response {
	switch req.Method {
	case "initialize":
		return Response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
			"protocolVersion": "2025-06-18",
			"serverInfo":      map[string]any{"name": "culture-points-mall", "version": "0.0.1"},
			"capabilities":    map[string]any{"tools": map[string]any{}},
		}}
	case "tools/list":
		list := s.Registry.List()
		descs := make([]ToolDescriptor, 0, len(list))
		for _, t := range list {
			descs = append(descs, ToolDescriptor{Name: t.Name(), Description: t.Description(), InputSchema: t.InputSchema()})
		}
		return Response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"tools": descs}}
	case "tools/call":
		var p CallParams
		_ = json.Unmarshal(req.Params, &p)
		res := s.Registry.Call(ctx, p.Name, p.Arguments)
		if res.IsError {
			return Response{JSONRPC: "2.0", ID: req.ID, Result: CallResult{
				Content: []ContentBlock{{Type: "text", Text: res.Message}},
				IsError: true,
			}}
		}
		raw, _ := json.Marshal(res.Output)
		return Response{JSONRPC: "2.0", ID: req.ID, Result: CallResult{
			Content: []ContentBlock{{Type: "text", Text: string(raw)}},
		}}
	default:
		return Response{JSONRPC: "2.0", ID: req.ID, Error: &Error{Code: -32601, Message: "method not found"}}
	}
}
