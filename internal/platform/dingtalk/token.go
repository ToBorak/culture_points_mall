package dingtalk

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type tokenManager struct {
	api       *caller
	rdb       *redis.Client
	appKey    string
	appSecret string
}

func (t *tokenManager) cacheKey() string { return "dingtalk:corp_token:" + t.appKey }

// corpToken 取企业 access_token，优先 Redis 缓存，miss 则请求钉钉并按 expireIn-300s 回写。
func (t *tokenManager) corpToken(ctx context.Context) (string, error) {
	if v, err := t.rdb.Get(ctx, t.cacheKey()).Result(); err == nil && v != "" {
		return v, nil
	}
	var out struct {
		AccessToken string `json:"accessToken"`
		ExpireIn    int    `json:"expireIn"`
	}
	if err := t.api.apiPost(ctx, "/v1.0/oauth2/accessToken", "", map[string]string{
		"appKey":    t.appKey,
		"appSecret": t.appSecret,
	}, &out); err != nil {
		return "", err
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("dingtalk: empty accessToken")
	}
	ttl := time.Duration(out.ExpireIn-300) * time.Second
	if ttl < time.Minute {
		ttl = time.Minute
	}
	_ = t.rdb.Set(ctx, t.cacheKey(), out.AccessToken, ttl).Err()
	return out.AccessToken, nil
}
