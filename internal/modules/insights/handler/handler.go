package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/standardsoftware/culture_points_mall/internal/modules/insights/service"
	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

type Handler struct {
	Svc *service.Service
}

func New(s *service.Service) *Handler { return &Handler{Svc: s} }

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/api/v1/me/dna-report", h.dnaReport)
	rg.GET("/api/v1/me/coach", h.coach)
	rg.GET("/api/v1/me/challenge/today", h.todayChallenge)
	rg.POST("/api/v1/me/challenge/submit", h.submitChallenge)
	rg.GET("/api/v1/me/leaderboard-insight", h.leaderboardInsight)
}

func (h *Handler) dnaReport(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	uid := cpmctx.UserID(c.Request.Context())
	period := c.DefaultQuery("period", "quarter")
	r, err := h.Svc.DNAReport(c.Request.Context(), tid, uid, period)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, r)
}

func (h *Handler) coach(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	uid := cpmctx.UserID(c.Request.Context())
	r, err := h.Svc.Coach(c.Request.Context(), tid, uid)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, r)
}

func (h *Handler) todayChallenge(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	uid := cpmctx.UserID(c.Request.Context())
	r, err := h.Svc.TodayChallenge(c.Request.Context(), tid, uid)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, r)
}

type submitReq struct {
	Proof string `json:"proof" binding:"required"`
}

func (h *Handler) submitChallenge(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	uid := cpmctx.UserID(c.Request.Context())
	var req submitReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	r, err := h.Svc.SubmitChallenge(c.Request.Context(), tid, uid, req.Proof)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, r)
}

func (h *Handler) leaderboardInsight(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	uid := cpmctx.UserID(c.Request.Context())
	r, err := h.Svc.LeaderboardInsight(c.Request.Context(), tid, uid)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, r)
}
