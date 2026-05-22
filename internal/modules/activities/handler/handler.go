package handler

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/standardsoftware/culture_points_mall/internal/modules/activities/domain"
	"github.com/standardsoftware/culture_points_mall/internal/modules/activities/service"
	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

type Handler struct{ Svc *service.Service }

func New(s *service.Service) *Handler { return &Handler{Svc: s} }

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.POST("/admin/activities", h.create)
	rg.GET("/api/v1/activities", h.list)
}

type createReq struct {
	DimensionCode string     `json:"dimensionCode" binding:"required"`
	Title         string     `json:"title" binding:"required"`
	StartAt       *time.Time `json:"startAt"`
	EndAt         *time.Time `json:"endAt"`
	Capacity      *int       `json:"capacity"`
	LocationLat   *float64   `json:"locationLat"`
	LocationLng   *float64   `json:"locationLng"`
	RadiusM       *int       `json:"radiusM"`
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
		LocationLat: req.LocationLat, LocationLng: req.LocationLng, RadiusM: req.RadiusM,
		PointsReward: req.PointsReward,
	})
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"id": a.ID, "title": a.Title, "status": a.Status, "dimensionId": a.DimensionID})
}

func (h *Handler) list(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	if tid == 0 {
		tid = 1
	}
	rows, err := h.Svc.List(c.Request.Context(), tid, domain.Status(c.Query("status")))
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"items": rows})
}
