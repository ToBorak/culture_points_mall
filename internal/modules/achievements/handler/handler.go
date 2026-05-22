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
	ID          int64  `json:"id"`
	DimensionID int64  `json:"dimensionId"`
	Name        string `json:"name"`
	Rarity      string `json:"rarity"`
	IconURL     string `json:"iconUrl"`
	Earned      bool   `json:"earned"`
}

func (h *Handler) list(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	uid := cpmctx.UserID(c.Request.Context())
	badges, owned, err := h.Svc.ListMyBadges(c.Request.Context(), tid, uid)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	out := make([]badgeItem, 0, len(badges))
	for _, b := range badges {
		out = append(out, badgeItem{
			ID: b.ID, DimensionID: b.DimensionID, Name: b.Name,
			Rarity: string(b.Rarity), IconURL: b.IconURL, Earned: owned[b.ID],
		})
	}
	c.JSON(200, gin.H{"items": out})
}
