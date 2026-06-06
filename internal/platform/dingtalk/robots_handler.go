package dingtalk

import (
	"github.com/gin-gonic/gin"

	"github.com/standardsoftware/culture_points_mall/internal/config"
)

// RobotsHandler 暴露已配置的群机器人列表（只回 id+name，绝不回 webhook/secret）。
type RobotsHandler struct{ robots []config.RobotCfg }

func NewRobotsHandler(robots []config.RobotCfg) *RobotsHandler {
	return &RobotsHandler{robots: robots}
}

func (h *RobotsHandler) RegisterAdmin(rg *gin.RouterGroup) {
	rg.GET("/admin/dingtalk/robots", h.list)
}

func (h *RobotsHandler) list(c *gin.Context) {
	out := make([]gin.H, 0, len(h.robots))
	for _, r := range h.robots {
		out = append(out, gin.H{"id": r.ID, "name": r.Name})
	}
	c.JSON(200, gin.H{"items": out})
}
