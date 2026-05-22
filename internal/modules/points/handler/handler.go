package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/standardsoftware/culture_points_mall/internal/modules/points/service"
	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

type Handler struct{ Svc *service.Service }

func New(svc *service.Service) *Handler { return &Handler{Svc: svc} }

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/api/v1/me/transactions", h.listMyTx)
}

func (h *Handler) listMyTx(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	uid := cpmctx.UserID(c.Request.Context())
	if tid == 0 || uid == 0 {
		c.JSON(401, gin.H{"error": "unauthenticated"})
		return
	}
	cursor, _ := strconv.ParseInt(c.Query("cursor"), 10, 64)
	limit, _ := strconv.Atoi(c.Query("limit"))
	rows, err := h.Svc.ListTransactions(c.Request.Context(), tid, uid, cursor, limit)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	type item struct {
		ID          int64  `json:"id"`
		DimensionID int64  `json:"dimensionId"`
		Amount      int    `json:"amount"`
		Reason      string `json:"reason"`
		CreatedAt   string `json:"createdAt"`
	}
	out := make([]item, 0, len(rows))
	for _, r := range rows {
		out = append(out, item{r.ID, r.DimensionID, r.Amount, r.Reason, r.CreatedAt.Format("2006-01-02T15:04:05Z07:00")})
	}
	var nextCursor *int64
	if len(rows) > 0 {
		v := rows[len(rows)-1].ID
		nextCursor = &v
	}
	c.JSON(200, gin.H{"items": out, "nextCursor": nextCursor})
}
