package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/standardsoftware/culture_points_mall/internal/modules/values/domain"
	"github.com/standardsoftware/culture_points_mall/internal/modules/values/service"
	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

type Handler struct{ Svc *service.Service }

func New(svc *service.Service) *Handler { return &Handler{Svc: svc} }

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/api/v1/values/dimensions", h.list)
	rg.GET("/admin/values/dimensions", h.list)
	rg.POST("/admin/values/dimensions", h.upsert)
}

type dimResp struct {
	ID        int64   `json:"id"`
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	Keywords  string  `json:"keywords"`
	Weight    float64 `json:"weight"`
	SortOrder int     `json:"sortOrder"`
	Enabled   bool    `json:"enabled"`
}

func toResp(d domain.Dimension) dimResp {
	return dimResp{d.ID, d.Code, d.Name, d.Keywords, d.Weight, d.SortOrder, d.Enabled}
}

func (h *Handler) list(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	if tid == 0 {
		tid = 1
	}
	rows, err := h.Svc.GetDimensions(c.Request.Context(), tid)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	out := make([]dimResp, 0, len(rows))
	for _, r := range rows {
		out = append(out, toResp(r))
	}
	c.JSON(200, gin.H{"items": out})
}

type upsertReq struct {
	Code      string  `json:"code" binding:"required"`
	Name      string  `json:"name" binding:"required"`
	Keywords  string  `json:"keywords"`
	Weight    float64 `json:"weight"`
	SortOrder int     `json:"sortOrder"`
	Enabled   bool    `json:"enabled"`
}

func (h *Handler) upsert(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	if tid == 0 {
		tid = 1
	}
	var req upsertReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.Weight == 0 {
		req.Weight = 1.0
	}
	if err := h.Svc.Upsert(c.Request.Context(), &domain.Dimension{
		TenantID: tid, Code: req.Code, Name: req.Name, Keywords: req.Keywords,
		Weight: req.Weight, SortOrder: req.SortOrder, Enabled: req.Enabled,
	}); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
