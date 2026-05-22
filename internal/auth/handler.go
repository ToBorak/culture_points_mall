package auth

import (
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/standardsoftware/culture_points_mall/internal/config"
	"github.com/standardsoftware/culture_points_mall/internal/platform/dingtalk"
)

type Handler struct {
	DB     *gorm.DB
	Cfg    *config.Config
	Signer *Signer
	Ding   dingtalk.Client
}

func NewHandler(db *gorm.DB, cfg *config.Config, ding dingtalk.Client) *Handler {
	return &Handler{
		DB:     db,
		Cfg:    cfg,
		Signer: &Signer{Secret: []byte(cfg.JWT.Secret), TTL: time.Duration(cfg.JWT.TTLHours) * time.Hour},
		Ding:   ding,
	}
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.POST("/auth/dingtalk/login", h.dingLogin)
	rg.POST("/auth/dev/login", h.devLogin)
}

type dingLoginReq struct {
	Code string `json:"code" binding:"required"`
}

type loginResp struct {
	Token    string `json:"token"`
	UserID   int64  `json:"userId"`
	TenantID int64  `json:"tenantId"`
	Name     string `json:"name"`
}

func (h *Handler) dingLogin(c *gin.Context) {
	var req dingLoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	user, err := h.Ding.GetUserByCode(c.Request.Context(), req.Code)
	if err != nil {
		c.JSON(401, gin.H{"error": err.Error()})
		return
	}
	tid := h.Cfg.Seed.DefaultTenantID
	userID, name, err := h.upsertUser(c, tid, user)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	tok, err := h.Signer.Issue(userID, tid, nil)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, loginResp{Token: tok, UserID: userID, TenantID: tid, Name: name})
}

type devLoginReq struct {
	UserID int64 `json:"userId" binding:"required"`
}

func (h *Handler) devLogin(c *gin.Context) {
	var req devLoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	tid := h.Cfg.Seed.DefaultTenantID
	var name string
	if err := h.DB.Raw("SELECT name FROM users WHERE id = ? AND tenant_id = ?", req.UserID, tid).Scan(&name).Error; err != nil {
		c.JSON(404, gin.H{"error": "user not found"})
		return
	}
	tok, _ := h.Signer.Issue(req.UserID, tid, nil)
	c.JSON(200, loginResp{Token: tok, UserID: req.UserID, TenantID: tid, Name: name})
}

func (h *Handler) upsertUser(c *gin.Context, tid int64, du dingtalk.User) (int64, string, error) {
	var existing struct {
		ID   int64
		Name string
	}
	err := h.DB.WithContext(c.Request.Context()).
		Raw("SELECT id, name FROM users WHERE tenant_id = ? AND ding_user_id = ? LIMIT 1", tid, du.DingUserID).
		Scan(&existing).Error
	if err == nil && existing.ID > 0 {
		return existing.ID, existing.Name, nil
	}
	res := h.DB.WithContext(c.Request.Context()).
		Exec("INSERT INTO users (tenant_id, ding_user_id, name, avatar_url) VALUES (?, ?, ?, ?)", tid, du.DingUserID, du.Name, du.AvatarURL)
	if res.Error != nil {
		return 0, "", res.Error
	}
	var id int64
	h.DB.WithContext(c.Request.Context()).
		Raw("SELECT LAST_INSERT_ID()").Scan(&id)
	return id, du.Name, nil
}
