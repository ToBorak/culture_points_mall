//go:build integration

package storage

import (
	"testing"

	"github.com/standardsoftware/culture_points_mall/internal/config"
	"github.com/stretchr/testify/require"
)

func TestRedisConnect_Integration(t *testing.T) {
	c, err := NewRedis(config.RedisCfg{Addr: "127.0.0.1:36379"})
	require.NoError(t, err)
	defer c.Close()
}
