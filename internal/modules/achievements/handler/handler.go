package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/standardsoftware/culture_points_mall/internal/modules/achievements/service"
	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

type Handler struct{ Svc *service.Service }

func New(s *service.Service) *Handler { return &Handler{Svc: s} }

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/api/v1/me/badges", h.list)
}

type badgeItem struct {
	ID              int64  `json:"id"`
	DimensionID     int64  `json:"dimensionId"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	Rarity          string `json:"rarity"`
	IconURL         string `json:"iconUrl"` // emblem 代码
	Earned          bool   `json:"earned"`
	ProgressCurrent int    `json:"progressCurrent"`
	ProgressTarget  int    `json:"progressTarget"`
}

func (h *Handler) list(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	uid := cpmctx.UserID(c.Request.Context())
	views, err := h.Svc.ListMyBadgeViews(c.Request.Context(), tid, uid)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	out := make([]badgeItem, 0, len(views))
	for _, v := range views {
		out = append(out, badgeItem{
			ID: v.Badge.ID, DimensionID: v.Badge.DimensionID, Name: v.Badge.Name,
			Description: v.Badge.Description, Rarity: string(v.Badge.Rarity), IconURL: v.Badge.IconURL,
			Earned: v.Earned, ProgressCurrent: v.ProgressCurrent, ProgressTarget: v.ProgressTarget,
		})
	}
	c.JSON(200, gin.H{"items": out})
}
