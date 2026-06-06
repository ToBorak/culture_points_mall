package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func routeSet(register func(*gin.RouterGroup)) map[string]bool {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	register(e.Group("/"))
	m := map[string]bool{}
	for _, ri := range e.Routes() {
		m[ri.Method+" "+ri.Path] = true
	}
	return m
}

func TestLayoutsAdminRoutesGated(t *testing.T) {
	h := New(nil)
	pub := routeSet(h.Register)
	require.True(t, pub["GET /api/v1/layout"])
	require.False(t, pub["PUT /api/v1/admin/layout"], "保存接口不应在公开/authed组")
	require.False(t, pub["GET /api/v1/admin/layout"], "admin读接口不应在公开/authed组")

	adm := routeSet(h.RegisterAdmin)
	require.True(t, adm["GET /api/v1/admin/layout"])
	require.True(t, adm["PUT /api/v1/admin/layout"])
}
