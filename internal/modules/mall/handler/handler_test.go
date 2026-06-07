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

func TestMallAdminRoutesGated(t *testing.T) {
	h := New(nil, nil, "")
	pub := routeSet(h.Register)
	require.True(t, pub["GET /api/v1/mall/items"])
	require.True(t, pub["POST /api/v1/mall/blindbox/draw"])
	require.False(t, pub["POST /api/v1/admin/mall/items"], "创建接口不应在公开/authed组")
	require.False(t, pub["DELETE /api/v1/admin/mall/items/:id"], "删除接口不应在公开/authed组")
	require.False(t, pub["PUT /api/v1/admin/mall/blindbox/:id/config"], "盲盒配置不应在公开/authed组")

	adm := routeSet(h.RegisterAdmin)
	require.True(t, adm["POST /api/v1/admin/mall/items"])
	require.True(t, adm["DELETE /api/v1/admin/mall/items/:id"])
	require.True(t, adm["POST /api/v1/admin/mall/upload"])
	require.True(t, adm["GET /api/v1/admin/mall/blindbox/:id/config"])
	require.True(t, adm["PUT /api/v1/admin/mall/blindbox/:id/config"])
}
