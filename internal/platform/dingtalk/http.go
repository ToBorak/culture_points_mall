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
	_ = json.Unmarshal(raw, &env)
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

// do 发 POST，返回响应体；headerToken 非空时设钉钉新接口 header，并对非 2xx 报错。
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
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("dingtalk %s status=%d body=%s", fullURL, resp.StatusCode, string(raw))
	}
	return raw, nil
}
