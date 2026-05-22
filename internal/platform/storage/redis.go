package storage

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/standardsoftware/culture_points_mall/internal/config"
)

func NewRedis(cfg config.RedisCfg) (*redis.Client, error) {
	c := redis.NewClient(&redis.Options{Addr: cfg.Addr, DB: cfg.DB})
	if err := c.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return c, nil
}
