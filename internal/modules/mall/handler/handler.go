package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/standardsoftware/culture_points_mall/internal/modules/mall/repository"
	"github.com/standardsoftware/culture_points_mall/internal/modules/mall/service"
	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

type Handler struct {
	Repo *repository.GormRepo
	Svc  *service.Service
}

func New(r *repository.GormRepo, s *service.Service) *Handler { return &Handler{Repo: r, Svc: s} }

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/api/v1/mall/items", h.list)
	rg.POST("/api/v1/mall/blindbox/draw", h.draw)
	rg.POST("/api/v1/admin/mall/items", h.create)
}

func (h *Handler) list(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	if tid == 0 {
		tid = 1
	}
	rows, err := h.Repo.ListItems(c.Request.Context(), tid, c.Query("type"))
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"items": rows})
}

type drawReq struct {
	BoxID int64 `json:"boxId" binding:"required"`
}

func (h *Handler) draw(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	uid := cpmctx.UserID(c.Request.Context())
	var req drawReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	res, err := h.Svc.Draw(c.Request.Context(), tid, uid, req.BoxID)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, res)
}

type createItemReq struct {
	Type     string `json:"type" binding:"required"`
	Name     string `json:"name" binding:"required"`
	Cost     int    `json:"cost" binding:"required"`
	Stock    *int   `json:"stock"`
	ImageURL string `json:"image_url"`
}

func (h *Handler) create(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	if tid == 0 {
		tid = 1
	}
	var req createItemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	it, err := h.Svc.CreateItem(c.Request.Context(), service.CreateItemCmd{
		TenantID: tid,
		Type:     req.Type,
		Name:     req.Name,
		Cost:     req.Cost,
		Stock:    req.Stock,
		ImageURL: req.ImageURL,
	})
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, it)
}
