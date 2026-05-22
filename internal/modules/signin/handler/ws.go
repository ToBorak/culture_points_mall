package handler

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"nhooyr.io/websocket"
)

func (h *Handler) RegisterWS(rg *gin.RouterGroup) {
	rg.GET("/admin/activities/:id/signin-codes/stream", h.codeStream)
}

func (h *Handler) codeStream(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	conn, err := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusInternalError, "closed")
	ctx := c.Request.Context()
	period := time.Duration(h.Svc.WindowSecs) * time.Second / 2
	if period <= 0 {
		period = 30 * time.Second
	}
	ticker := time.NewTicker(period)
	defer ticker.Stop()
	send := func() error {
		payload := map[string]any{"code": h.Svc.CurrentCode(id), "expiresIn": h.Svc.WindowSecs}
		raw, _ := json.Marshal(payload)
		return conn.Write(ctx, websocket.MessageText, raw)
	}
	if err := send(); err != nil {
		return
	}
	for {
		select {
		case <-ticker.C:
			if err := send(); err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
