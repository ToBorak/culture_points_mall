package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

func injectRoles(roles []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(cpmctx.WithRoles(c.Request.Context(), roles))
		c.Next()
	}
}

func TestRequireRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	newSrv := func(roles []string) *httptest.Server {
		r := gin.New()
		r.GET("/admin/x", injectRoles(roles), RequireRole("admin"), func(c *gin.Context) {
			c.JSON(200, gin.H{"ok": true})
		})
		return httptest.NewServer(r)
	}

	srvAdmin := newSrv([]string{"admin"})
	defer srvAdmin.Close()
	resp, err := http.Get(srvAdmin.URL + "/admin/x")
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	srvNone := newSrv(nil)
	defer srvNone.Close()
	resp2, err := http.Get(srvNone.URL + "/admin/x")
	require.NoError(t, err)
	require.Equal(t, 403, resp2.StatusCode)
}
