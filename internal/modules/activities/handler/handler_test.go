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

func TestActivitiesAdminRoutesGated(t *testing.T) {
	h := New(nil)
	pub := routeSet(h.Register)
	require.True(t, pub["GET /api/v1/activities"])
	require.False(t, pub["POST /admin/activities"], "写接口不应出现在公开组")

	adm := routeSet(h.RegisterAdmin)
	require.True(t, adm["POST /admin/activities"])
}
