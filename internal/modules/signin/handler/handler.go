package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/standardsoftware/culture_points_mall/internal/modules/signin/service"
	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

type Handler struct{ Svc *service.Service }

func New(s *service.Service) *Handler { return &Handler{Svc: s} }

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.POST("/api/v1/signin/check", h.check)
	rg.GET("/admin/activities/:id/signin-code", h.currentCode)
}

type checkReq struct {
	ActivityID int64    `json:"activityId" binding:"required"`
	Code       string   `json:"code" binding:"required"`
	GPSLat     *float64 `json:"gpsLat"`
	GPSLng     *float64 `json:"gpsLng"`
	QuizExpect string   `json:"quizExpect"`
	QuizAnswer string   `json:"quizAnswer"`
}

func (h *Handler) check(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	uid := cpmctx.UserID(c.Request.Context())
	var req checkReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	res, err := h.Svc.Check(c.Request.Context(), service.CheckCmd{
		TenantID: tid, UserID: uid, ActivityID: req.ActivityID,
		Code: req.Code, GPSLat: req.GPSLat, GPSLng: req.GPSLng,
		QuizExpect: req.QuizExpect, QuizAnswer: req.QuizAnswer,
	})
	if err != nil {
		if errors.Is(err, service.ErrAlreadySignedIn) {
			c.JSON(409, gin.H{"error": err.Error()})
			return
		}
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if !res.OK {
		c.JSON(400, gin.H{"ok": false, "reason": res.Reason})
		return
	}
	c.JSON(200, gin.H{"ok": true, "transactionId": res.TransactionID, "newBadges": res.NewBadges})
}

func (h *Handler) currentCode(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	c.JSON(200, gin.H{"code": h.Svc.CurrentCode(id), "windowSecs": h.Svc.WindowSecs})
}
