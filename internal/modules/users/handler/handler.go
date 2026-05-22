package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/standardsoftware/culture_points_mall/internal/modules/users/service"
	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

type Handler struct{ Svc *service.Service }

func New(svc *service.Service) *Handler { return &Handler{Svc: svc} }

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/api/v1/me", h.me)
}

func (h *Handler) me(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	uid := cpmctx.UserID(c.Request.Context())
	u, err := h.Svc.GetByID(c.Request.Context(), tid, uid)
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{
		"id": u.ID, "name": u.Name, "avatarUrl": u.AvatarURL, "deptId": u.DeptID,
	})
}
