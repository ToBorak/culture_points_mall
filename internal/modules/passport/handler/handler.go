package handler

import (
	"github.com/gin-gonic/gin"

	achvsvc "github.com/standardsoftware/culture_points_mall/internal/modules/achievements/service"
	pointssvc "github.com/standardsoftware/culture_points_mall/internal/modules/points/service"
	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

type Handler struct {
	Points *pointssvc.Service
	Achv   *achvsvc.Service
}

func New(p *pointssvc.Service, a *achvsvc.Service) *Handler { return &Handler{Points: p, Achv: a} }

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/api/v1/me/passport", h.summary)
}

func (h *Handler) summary(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	uid := cpmctx.UserID(c.Request.Context())

	scores, dims, total, err := h.Points.GetUserScores(c.Request.Context(), tid, uid)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	badges, owned, err := h.Achv.ListMyBadges(c.Request.Context(), tid, uid)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	badgeCount := 0
	for _, b := range badges {
		if owned[b.ID] {
			badgeCount++
		}
	}

	type dimResp struct {
		DimensionID   int64  `json:"dimensionId"`
		DimensionCode string `json:"dimensionCode"`
		DimensionName string `json:"dimensionName"`
		TotalScore    int    `json:"totalScore"`
		QuarterScore  int    `json:"quarterScore"`
		YearScore     int    `json:"yearScore"`
	}
	out := make([]dimResp, 0, len(dims))
	for _, d := range dims {
		var ds dimResp
		ds.DimensionID = d.ID
		ds.DimensionCode = d.Code
		ds.DimensionName = d.Name
		for _, s := range scores {
			if s.DimensionID == d.ID {
				ds.TotalScore = s.TotalScore
				ds.QuarterScore = s.QuarterScore
				ds.YearScore = s.YearScore
				break
			}
		}
		out = append(out, ds)
	}
	c.JSON(200, gin.H{
		"totalScore":        total,
		"scoresByDimension": out,
		"badgeCount":        badgeCount,
		"dimensions":        dims,
	})
}
