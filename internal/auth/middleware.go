package auth

import (
	"strings"

	"github.com/gin-gonic/gin"

	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

func RequireJWT(s *Signer) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			c.AbortWithStatusJSON(401, gin.H{"error": "missing bearer"})
			return
		}
		token := strings.TrimPrefix(h, "Bearer ")
		claims, err := s.Parse(token)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid token"})
			return
		}
		ctx := c.Request.Context()
		ctx = cpmctx.WithTenant(ctx, claims.TenantID)
		ctx = cpmctx.WithUser(ctx, claims.UserID)
		ctx = cpmctx.WithRoles(ctx, claims.Roles)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
