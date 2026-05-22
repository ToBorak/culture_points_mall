package handler

import (
	"encoding/json"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/standardsoftware/culture_points_mall/internal/modules/agent/repository"
	"github.com/standardsoftware/culture_points_mall/internal/modules/agent/service"
	"github.com/standardsoftware/culture_points_mall/internal/platform/llm"
	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

type Handler struct {
	Orchestrator *service.Orchestrator
	Sessions     *repository.Repo
}

func New(o *service.Orchestrator, s *repository.Repo) *Handler {
	return &Handler{Orchestrator: o, Sessions: s}
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.POST("/admin/agent/chat", h.chat)
	rg.GET("/admin/agent/sessions", h.listSessions)
}

type chatReq struct {
	SessionID int64  `json:"sessionId"`
	Text      string `json:"text" binding:"required"`
}

func (h *Handler) chat(c *gin.Context) {
	var req chatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	tid := cpmctx.TenantID(c.Request.Context())
	uid := cpmctx.UserID(c.Request.Context())

	var history []llm.Message
	if req.SessionID > 0 {
		msgs, err := h.Sessions.ListMessages(c.Request.Context(), req.SessionID)
		if err == nil {
			for _, m := range msgs {
				var blocks []llm.Block
				_ = json.Unmarshal(m.Content, &blocks)
				history = append(history, llm.Message{Role: llm.Role(m.Role), Content: blocks})
			}
		}
	} else {
		s := &repository.Session{TenantID: tid, OperatorID: uid, Title: truncate(req.Text, 50)}
		if err := h.Sessions.CreateSession(c.Request.Context(), s); err == nil {
			req.SessionID = s.ID
		}
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Writer.Flush()

	if req.SessionID > 0 {
		userBlocks := []llm.Block{{Type: "text", Text: req.Text}}
		raw, _ := json.Marshal(userBlocks)
		_ = h.Sessions.AppendMessage(c.Request.Context(), &repository.Message{
			SessionID: req.SessionID, Role: string(llm.RoleUser), Content: raw,
		})
	}

	_, _ = c.Writer.Write([]byte("event: session\ndata: " + jsonStr(map[string]any{"sessionId": req.SessionID}) + "\n\n"))
	c.Writer.Flush()

	steps, _ := h.Orchestrator.Run(c.Request.Context(), history, req.Text)
	for step := range steps {
		raw, _ := json.Marshal(step)
		_, _ = c.Writer.Write([]byte("event: step\ndata: "))
		_, _ = c.Writer.Write(raw)
		_, _ = c.Writer.Write([]byte("\n\n"))
		c.Writer.Flush()

		if req.SessionID > 0 && (step.Kind == service.StepLLMText || step.Kind == service.StepToolUse || step.Kind == service.StepToolResult) {
			msgRaw, _ := json.Marshal([]any{step})
			role := llm.RoleAssistant
			if step.Kind == service.StepToolResult {
				role = llm.RoleTool
			}
			_ = h.Sessions.AppendMessage(c.Request.Context(), &repository.Message{
				SessionID: req.SessionID, Role: string(role), Content: msgRaw,
			})
		}
	}
}

func (h *Handler) listSessions(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	uid := cpmctx.UserID(c.Request.Context())
	limit, _ := strconv.Atoi(c.Query("limit"))
	rows, err := h.Sessions.ListSessions(c.Request.Context(), tid, uid, limit)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"items": rows})
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func jsonStr(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}
