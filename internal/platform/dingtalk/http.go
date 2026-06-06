package dingtalk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// caller 封装钉钉两套域名的请求风格。
// oapi.dingtalk.com：token 走 ?access_token= 查询，错误读 errcode。
// api.dingtalk.com：token 走 header x-acs-dingtalk-access-token，错误读 HTTP 状态。
type caller struct {
	hc       *http.Client
	oapiBase string
	apiBase  string
}

func newCaller() *caller {
	return &caller{
		hc:       &http.Client{Timeout: 10 * time.Second},
		oapiBase: "https://oapi.dingtalk.com",
		apiBase:  "https://api.dingtalk.com",
	}
}

func (c *caller) oapiPost(ctx context.Context, path, token string, in, out any) error {
	u := c.oapiBase + path + "?access_token=" + url.QueryEscape(token)
	raw, err := c.do(ctx, u, "", in)
	if err != nil {
		return err
	}
	var env struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("dingtalk oapi %s: invalid json response: %w", path, err)
	}
	if env.ErrCode != 0 {
		return fmt.Errorf("dingtalk oapi %s errcode=%d errmsg=%s", path, env.ErrCode, env.ErrMsg)
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

func (c *caller) apiPost(ctx context.Context, path, token string, in, out any) error {
	raw, err := c.do(ctx, c.apiBase+path, token, in)
	if err != nil {
		return err
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// apiGet 发 GET 到 api.dingtalk.com 新接口（参数走 query，token 走 header），用于会议室列表等只读接口。
func (c *caller) apiGet(ctx context.Context, path, token string, query url.Values, out any) error {
	full := c.apiBase + path
	if len(query) > 0 {
		full += "?" + query.Encode()
	}
	raw, err := c.doGet(ctx, full, token)
	if err != nil {
		return err
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// do 发 POST 并返回响应体。headerToken 非空时设置钉钉新接口 header；任何非 2xx 响应都返回错误。
func (c *caller) do(ctx context.Context, fullURL, headerToken string, in any) ([]byte, error) {
	body, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if headerToken != "" {
		req.Header.Set("x-acs-dingtalk-access-token", headerToken)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("dingtalk %s read body: %w", redactToken(fullURL), err)
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("dingtalk %s status=%d body=%s", redactToken(fullURL), resp.StatusCode, string(raw))
	}
	return raw, nil
}

// doGet 发 GET 并返回响应体。语义与 do 一致，仅方法不同、无请求体。
func (c *caller) doGet(ctx context.Context, fullURL, headerToken string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}
	if headerToken != "" {
		req.Header.Set("x-acs-dingtalk-access-token", headerToken)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("dingtalk %s read body: %w", redactToken(fullURL), err)
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("dingtalk %s status=%d body=%s", redactToken(fullURL), resp.StatusCode, string(raw))
	}
	return raw, nil
}

// redactToken 掩码 URL 中的 access_token 查询值，避免错误信息把 token 泄露到日志。
func redactToken(u string) string {
	parsed, err := url.Parse(u)
	if err != nil {
		return u
	}
	q := parsed.Query()
	if q.Has("access_token") {
		q.Set("access_token", "***")
		parsed.RawQuery = q.Encode()
	}
	return parsed.String()
}
