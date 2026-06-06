package dingtalk

import (
	"github.com/redis/go-redis/v9"

	"github.com/standardsoftware/culture_points_mall/internal/config"
)

// New 按 mode 选择实现：real 返回 RealClient，其余返回传入的 mock。
func New(cfg config.DingTalkCfg, rdb *redis.Client, mock Client) Client {
	if cfg.Mode == "real" {
		return NewReal(cfg, rdb)
	}
	return mock
}
