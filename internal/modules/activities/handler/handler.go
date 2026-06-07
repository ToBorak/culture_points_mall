package handler

import (
	"errors"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/standardsoftware/culture_points_mall/internal/modules/activities/domain"
	"github.com/standardsoftware/culture_points_mall/internal/modules/activities/service"
	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

type Handler struct{ Svc *service.Service }

func New(s *service.Service) *Handler { return &Handler{Svc: s} }

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/api/v1/activities", h.list)
	rg.GET("/api/v1/activities/:id", h.detail)
	rg.POST("/api/v1/activities/:id/enroll", h.enroll)
	rg.DELETE("/api/v1/activities/:id/enroll", h.unenroll)
}

func (h *Handler) RegisterAdmin(rg *gin.RouterGroup) {
	rg.POST("/admin/activities", h.create)
}

type createReq struct {
	DimensionCode string     `json:"dimensionCode" binding:"required"`
	Title         string     `json:"title" binding:"required"`
	StartAt       *time.Time `json:"startAt"`
	EndAt         *time.Time `json:"endAt"`
	Capacity      *int       `json:"capacity"`
	PointsReward  int        `json:"pointsReward"`
}

func (h *Handler) create(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	if tid == 0 {
		tid = 1
	}
	var req createReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	a, err := h.Svc.Create(c.Request.Context(), service.CreateCmd{
		TenantID: tid, DimensionCode: req.DimensionCode, Title: req.Title,
		StartAt: req.StartAt, EndAt: req.EndAt, Capacity: req.Capacity,
		PointsReward: req.PointsReward,
	})
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"id": a.ID, "title": a.Title, "status": a.Status, "dimensionId": a.DimensionID})
}

func (h *Handler) list(c *gin.Context) {
	ctx := c.Request.Context()
	tid := cpmctx.TenantID(ctx)
	if tid == 0 {
		tid = 1
	}
	uid := cpmctx.UserID(ctx)
	rows, err := h.Svc.ListView(ctx, tid, uid, domain.Status(c.Query("status")))
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"items": rows})
}

func (h *Handler) detail(c *gin.Context) {
	ctx := c.Request.Context()
	tid := cpmctx.TenantID(ctx)
	if tid == 0 {
		tid = 1
	}
	uid := cpmctx.UserID(ctx)
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	v, err := h.Svc.Detail(ctx, tid, uid, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(404, gin.H{"error": "活动不存在"})
			return
		}
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, v)
}

func (h *Handler) enroll(c *gin.Context) {
	ctx := c.Request.Context()
	tid := cpmctx.TenantID(ctx)
	if tid == 0 {
		tid = 1
	}
	uid := cpmctx.UserID(ctx)
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	v, err := h.Svc.Enroll(ctx, tid, uid, id)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			c.JSON(404, gin.H{"error": "活动不存在"})
		case errors.Is(err, service.ErrActivityFull), errors.Is(err, service.ErrActivityClosed):
			c.JSON(409, gin.H{"error": err.Error()})
		default:
			c.JSON(500, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(200, v)
}

func (h *Handler) unenroll(c *gin.Context) {
	ctx := c.Request.Context()
	tid := cpmctx.TenantID(ctx)
	if tid == 0 {
		tid = 1
	}
	uid := cpmctx.UserID(ctx)
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	v, err := h.Svc.Unenroll(ctx, tid, uid, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(404, gin.H{"error": "活动不存在"})
			return
		}
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, v)
}
