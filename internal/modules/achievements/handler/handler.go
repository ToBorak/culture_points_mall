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
	rg.POST("/api/v1/me/badges/celebrated", h.markCelebrated)
}

// check 结算授予并返回所有「尚未庆祝」的已得勋章，供前端全局庆祝弹窗逐枚展示。
// 返回的勋章需前端展示后调 /me/badges/celebrated 回执落定，否则下次仍会返回（零丢失）。
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

// markCelebrated 前端在勋章弹窗展示后回执，落定「已庆祝」，之后不再返回这些勋章。
func (h *Handler) markCelebrated(c *gin.Context) {
	uid := cpmctx.UserID(c.Request.Context())
	var req struct {
		BadgeIDs []int64 `json:"badgeIds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if err := h.Svc.MarkCelebrated(c.Request.Context(), uid, req.BadgeIDs); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
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
	ProgressUnit    string `json:"progressUnit"`
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
			ProgressUnit: v.ProgressUnit,
		})
	}
	c.JSON(200, gin.H{"items": out})
}
