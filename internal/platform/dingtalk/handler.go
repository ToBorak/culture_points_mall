package dingtalk

import (
	"encoding/json"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

type MockHandler struct {
	DB  *gorm.DB
	Bus *Bus
}

func NewMockHandler(db *gorm.DB, bus *Bus) *MockHandler { return &MockHandler{DB: db, Bus: bus} }

func (h *MockHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/admin/dingtalk/mock-outbox", h.list)
	rg.GET("/admin/dingtalk/mock-outbox/stream", h.stream)
}

func (h *MockHandler) list(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	if tid == 0 {
		tid = 1
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows []mockOutbox
	err := h.DB.WithContext(c.Request.Context()).
		Where("tenant_id = ?", tid).Order("id DESC").Limit(limit).Find(&rows).Error
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	type item struct {
		ID        int64           `json:"id"`
		API       string          `json:"api"`
		Target    string          `json:"target"`
		Payload   json.RawMessage `json:"payload"`
		CreatedAt string          `json:"createdAt"`
	}
	out := make([]item, 0, len(rows))
	for _, r := range rows {
		out = append(out, item{r.ID, r.API, r.Target, r.Payload, r.CreatedAt.Format("2006-01-02T15:04:05Z07:00")})
	}
	c.JSON(200, gin.H{"items": out})
}

func (h *MockHandler) stream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	sub := h.Bus.Subscribe()
	notify := c.Writer.CloseNotify()
	for {
		select {
		case ev := <-sub:
			raw, _ := json.Marshal(ev)
			_, _ = c.Writer.Write([]byte("data: "))
			_, _ = c.Writer.Write(raw)
			_, _ = c.Writer.Write([]byte("\n\n"))
			c.Writer.Flush()
		case <-notify:
			return
		}
	}
}
