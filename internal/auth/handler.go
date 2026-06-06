package auth

import (
	"context"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/standardsoftware/culture_points_mall/internal/config"
	"github.com/standardsoftware/culture_points_mall/internal/platform/dingtalk"
)

// WelcomeBonusGranter 抽象积分注入接口，避免 auth 直接依赖 points/values
type WelcomeBonusGranter interface {
	GrantWelcomeBonus(ctx context.Context, tenantID, userID int64, amount int) error
}

type Handler struct {
	DB      *gorm.DB
	Cfg     *config.Config
	Signer  *Signer
	Ding    dingtalk.Client
	Granter WelcomeBonusGranter
}

func NewHandler(db *gorm.DB, cfg *config.Config, ding dingtalk.Client) *Handler {
	return &Handler{
		DB:     db,
		Cfg:    cfg,
		Signer: &Signer{Secret: []byte(cfg.JWT.Secret), TTL: time.Duration(cfg.JWT.TTLHours) * time.Hour},
		Ding:   ding,
	}
}

func (h *Handler) WithGranter(g WelcomeBonusGranter) *Handler {
	h.Granter = g
	return h
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
	roles := h.rolesFor(user.DingUserID, user.IsAdmin)
	tok, err := h.Signer.Issue(userID, tid, roles)
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
	var row struct {
		ID         int64
		Name       string
		DingUserID string
		IsAdmin    bool
	}
	if err := h.DB.WithContext(c.Request.Context()).
		Raw("SELECT id, name, ding_user_id, is_admin FROM users WHERE id = ? AND tenant_id = ?", req.UserID, tid).
		Scan(&row).Error; err != nil || row.ID == 0 {
		c.JSON(404, gin.H{"error": "user not found"})
		return
	}
	roles := h.rolesFor(row.DingUserID, row.IsAdmin)
	tok, _ := h.Signer.Issue(req.UserID, tid, roles)
	c.JSON(200, loginResp{Token: tok, UserID: req.UserID, TenantID: tid, Name: row.Name})
}

func (h *Handler) upsertUser(c *gin.Context, tid int64, du dingtalk.User) (int64, string, error) {
	ctx := c.Request.Context()
	var existing struct {
		ID   int64
		Name string
	}
	err := h.DB.WithContext(ctx).
		Raw("SELECT id, name FROM users WHERE tenant_id = ? AND ding_user_id = ? LIMIT 1", tid, du.DingUserID).
		Scan(&existing).Error
	if err == nil && existing.ID > 0 {
		if err := h.DB.WithContext(ctx).Exec(
			"UPDATE users SET union_id = ?, is_admin = ? WHERE id = ?",
			nullable(du.UnionID), boolToInt(du.IsAdmin), existing.ID).Error; err != nil {
			return 0, "", err
		}
		h.maybeGrantWelcome(ctx, tid, existing.ID)
		return existing.ID, existing.Name, nil
	}
	res := h.DB.WithContext(ctx).Exec(
		"INSERT INTO users (tenant_id, ding_user_id, name, avatar_url, union_id, is_admin) VALUES (?, ?, ?, ?, ?, ?)",
		tid, du.DingUserID, du.Name, du.AvatarURL, nullable(du.UnionID), boolToInt(du.IsAdmin))
	if res.Error != nil {
		return 0, "", res.Error
	}
	var id int64
	h.DB.WithContext(ctx).Raw("SELECT LAST_INSERT_ID()").Scan(&id)
	h.maybeGrantWelcome(ctx, tid, id)
	return id, du.Name, nil
}

func (h *Handler) rolesFor(dingUserID string, isAdmin bool) []string {
	if isAdmin {
		return []string{"admin"}
	}
	for _, id := range h.Cfg.DingTalk.AdminUserIDs {
		if id == dingUserID {
			return []string{"admin"}
		}
	}
	return nil
}

// nullable 把空串转 NULL，避免 union_id 空串撞 uk_tenant_union 唯一键。
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (h *Handler) maybeGrantWelcome(ctx context.Context, tid, uid int64) {
	if h.Granter == nil {
		return
	}
	bonus := h.Cfg.Seed.WelcomeBonus
	if bonus <= 0 {
		return
	}
	// 只在用户总积分 = 0 时补发
	var total int
	if err := h.DB.WithContext(ctx).
		Raw("SELECT COALESCE(SUM(total_score), 0) FROM user_dimension_scores WHERE tenant_id = ? AND user_id = ?", tid, uid).
		Scan(&total).Error; err != nil {
		log.Printf("welcome-bonus: read total failed uid=%d err=%v", uid, err)
		return
	}
	if total > 0 {
		return
	}
	if err := h.Granter.GrantWelcomeBonus(ctx, tid, uid, bonus); err != nil {
		log.Printf("welcome-bonus: grant failed uid=%d err=%v", uid, err)
	}
}
