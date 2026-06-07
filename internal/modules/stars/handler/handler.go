package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/standardsoftware/culture_points_mall/internal/modules/stars/domain"
	starssvc "github.com/standardsoftware/culture_points_mall/internal/modules/stars/service"
	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

type Handler struct{ Svc *starssvc.Service }

func New(s *starssvc.Service) *Handler { return &Handler{Svc: s} }

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/api/v1/stars/seasons/current", h.currentSeason)
	rg.POST("/api/v1/stars/nominations", h.nominate)
	rg.GET("/api/v1/stars/nominations/mine", h.myNominations)
}

func (h *Handler) RegisterAdmin(rg *gin.RouterGroup) {
	rg.POST("/admin/stars/seasons", h.createSeason)
	rg.PUT("/admin/stars/seasons/:id/status", h.advanceStatus)
	rg.GET("/admin/stars/seasons/:id/nominations", h.listNominations)
	rg.POST("/admin/stars/nominations/:id/score", h.score)
	rg.POST("/admin/stars/seasons/:id/select", h.selectWinners)
}

func (h *Handler) currentSeason(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	uid := cpmctx.UserID(c.Request.Context())
	if tid == 0 || uid == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	q, err := h.Svc.CurrentSeasonWithQuota(c.Request.Context(), tid, uid)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"season": nil})
		return
	}
	c.JSON(http.StatusOK, q)
}

func (h *Handler) nominate(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	uid := cpmctx.UserID(c.Request.Context())
	if tid == 0 || uid == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req struct {
		SeasonID    int64  `json:"seasonId" binding:"required"`
		NomineeID   int64  `json:"nomineeId"`
		DimensionID int64  `json:"dimensionId" binding:"required"`
		CaseText    string `json:"caseText" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	n, err := h.Svc.Nominate(c.Request.Context(), starssvc.NominateCmd{
		TenantID: tid, SeasonID: req.SeasonID, NominatorID: uid,
		NomineeID: req.NomineeID, DimensionID: req.DimensionID, CaseText: req.CaseText,
	})
	if err != nil {
		switch {
		case errors.Is(err, starssvc.ErrDuplicateNomination):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, starssvc.ErrSeasonNotOpen):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, starssvc.ErrNomineeNotFound):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, n)
}

func (h *Handler) myNominations(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	uid := cpmctx.UserID(c.Request.Context())
	seasonID, _ := strconv.ParseInt(c.Query("seasonId"), 10, 64)
	submitted, received, err := h.Svc.MyNominations(c.Request.Context(), tid, uid, seasonID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"submitted": submitted, "received": received})
}

func (h *Handler) createSeason(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	var req struct {
		Name        string `json:"name" binding:"required"`
		QuarterCode string `json:"quarterCode" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sn := &domain.Season{TenantID: tid, Name: req.Name, QuarterCode: req.QuarterCode}
	if err := h.Svc.CreateSeason(c.Request.Context(), sn); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sn)
}

func (h *Handler) advanceStatus(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.Svc.AdvanceStatus(c.Request.Context(), tid, id, domain.SeasonStatus(req.Status)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) listNominations(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	rows, err := h.Svc.ListNominations(c.Request.Context(), tid, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

func (h *Handler) score(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	nominationID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		SeasonID int64   `json:"seasonId" binding:"required"`
		Score    float64 `json:"score"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.Svc.Score(c.Request.Context(), tid, req.SeasonID, nominationID, req.Score); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) selectWinners(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	seasonID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Picks []struct {
			UserID             int64  `json:"userId" binding:"required"`
			DimensionID        int64  `json:"dimensionId" binding:"required"`
			SourceNominationID *int64 `json:"sourceNominationId"`
			Citation           string `json:"citation"`
		} `json:"picks" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	picks := make([]starssvc.Pick, 0, len(req.Picks))
	for _, p := range req.Picks {
		picks = append(picks, starssvc.Pick{
			UserID: p.UserID, DimensionID: p.DimensionID,
			SourceNominationID: p.SourceNominationID, Citation: p.Citation,
		})
	}
	if err := h.Svc.SelectWinners(c.Request.Context(), tid, seasonID, picks); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
