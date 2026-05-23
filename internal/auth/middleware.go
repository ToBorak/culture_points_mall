package auth

import (
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

// RequireJWT 仅校验 token 签名。
func RequireJWT(s *Signer) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := authenticate(c, s)
		if !ok {
			return
		}
		attachContext(c, claims)
		c.Next()
	}
}

// RequireJWTWithUser 在 token 校验之外，再验证用户在数据库里仍然存在。
// 用于 DB 重置后老 token 失效的场景：前端拿到 401 后清 token 重新登录。
func RequireJWTWithUser(s *Signer, db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := authenticate(c, s)
		if !ok {
			return
		}
		var exists int
		if err := db.WithContext(c.Request.Context()).
			Raw("SELECT 1 FROM users WHERE id = ? AND tenant_id = ? LIMIT 1", claims.UserID, claims.TenantID).
			Scan(&exists).Error; err != nil || exists != 1 {
			c.AbortWithStatusJSON(401, gin.H{"error": "user not found", "code": "user_gone"})
			return
		}
		attachContext(c, claims)
		c.Next()
	}
}

func authenticate(c *gin.Context, s *Signer) (*Claims, bool) {
	h := c.GetHeader("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		c.AbortWithStatusJSON(401, gin.H{"error": "missing bearer"})
		return nil, false
	}
	token := strings.TrimPrefix(h, "Bearer ")
	claims, err := s.Parse(token)
	if err != nil {
		c.AbortWithStatusJSON(401, gin.H{"error": "invalid token"})
		return nil, false
	}
	return claims, true
}

func attachContext(c *gin.Context, claims *Claims) {
	ctx := c.Request.Context()
	ctx = cpmctx.WithTenant(ctx, claims.TenantID)
	ctx = cpmctx.WithUser(ctx, claims.UserID)
	ctx = cpmctx.WithRoles(ctx, claims.Roles)
	c.Request = c.Request.WithContext(ctx)
}
