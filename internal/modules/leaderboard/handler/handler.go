package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/standardsoftware/culture_points_mall/internal/modules/leaderboard/service"
	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

type Handler struct{ Svc *service.Service }

func New(s *service.Service) *Handler { return &Handler{Svc: s} }

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/api/v1/leaderboard", h.list)
}

func (h *Handler) list(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	if tid == 0 {
		tid = 1
	}
	dimID, _ := strconv.ParseInt(c.Query("dimension_id"), 10, 64)
	scope := c.DefaultQuery("scope", "total")
	rows, err := h.Svc.List(c.Request.Context(), service.ListParams{
		TenantID: tid, Scope: scope, DimensionID: dimID, Limit: 50,
	})
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{
		"scope":       scope,
		"window":      c.DefaultQuery("window", "year"),
		"dimensionId": dimID,
		"entries":     rows,
		"total":       len(rows),
	})
}
