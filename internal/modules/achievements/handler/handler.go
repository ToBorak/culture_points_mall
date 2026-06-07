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
	rg.POST("/api/v1/me/badges/check", h.check)
}

// check 结算并返回本次「新解锁」的勋章，供前端全局庆祝弹窗。无新勋章时 items 为空数组。
func (h *Handler) check(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	uid := cpmctx.UserID(c.Request.Context())
	newly, err := h.Svc.CheckNew(c.Request.Context(), tid, uid)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	out := make([]badgeItem, 0, len(newly))
	for _, b := range newly {
		out = append(out, badgeItem{
			ID: b.ID, DimensionID: b.DimensionID, Name: b.Name,
			Description: b.Description, Rarity: string(b.Rarity), IconURL: b.IconURL, Earned: true,
		})
	}
	c.JSON(200, gin.H{"items": out})
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
