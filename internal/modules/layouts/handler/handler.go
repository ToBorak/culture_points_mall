package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/standardsoftware/culture_points_mall/internal/modules/layouts/service"
	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

type Handler struct{ Svc *service.Service }

func New(s *service.Service) *Handler { return &Handler{Svc: s} }

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/api/v1/layout", h.get)             // 员工 H5 读取
	rg.GET("/api/v1/admin/layout", h.adminGet)  // admin 编辑读取（含 modules meta）
	rg.PUT("/api/v1/admin/layout", h.save)      // admin 保存
}

func (h *Handler) get(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	layout, err := h.Svc.Get(c.Request.Context(), tid)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, layout)
}

func (h *Handler) adminGet(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	layout, err := h.Svc.Get(c.Request.Context(), tid)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{
		"layout":           layout,
		"availableModules": service.AvailableModules(),
	})
}

func (h *Handler) save(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	var req service.Layout
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if err := h.Svc.Save(c.Request.Context(), tid, req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
