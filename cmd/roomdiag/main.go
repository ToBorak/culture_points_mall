// roomdiag 一次性诊断：为什么会议室列表为空。直接打印钉钉原始响应，不做任何解析假设。
// 用法：仓库根目录 `go run ./cmd/roomdiag`，读 ./configs/config.yaml（mode:real）。
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/standardsoftware/culture_points_mall/internal/config"
)

// 两个测试 userid（与 caltest 一致）
var userIDs = []string{"083221474637757281", "02562843492437802982"}

func main() {
	cfg, err := config.Load("./configs")
	if err != nil {
		log.Fatal(err)
	}
	tok := getToken(cfg.DingTalk.AppKey, cfg.DingTalk.AppSecret)
	fmt.Printf("AppKey=%s... token 已获取(len=%d)\n", safePrefix(cfg.DingTalk.AppKey), len(tok))

	// 1) 会议室分组（看企业到底有没有在本应用可见的会议室配置）
	dumpGet("会议室分组列表 /v1.0/rooms/groups", "https://api.dingtalk.com/v1.0/rooms/groups?maxResults=100", tok)

	// 2) 逐个 userid：先换 unionId，再查它可见的会议室（打印原始 JSON）
	for _, uid := range userIDs {
		union := resolveUnion(tok, uid)
		fmt.Printf("\n#### userid=%s -> unionId=%s ####\n", uid, union)
		q := url.Values{}
		q.Set("unionId", union)
		q.Set("maxResults", "100")
		dumpGet("会议室列表 /v1.0/rooms/meetingRoomLists", "https://api.dingtalk.com/v1.0/rooms/meetingRoomLists?"+q.Encode(), tok)
	}

	// 3) 不带 unionId 调一次，看钉钉怎么回（确认 unionId 是不是可见性关键）
	dumpGet("会议室列表(不带unionId)", "https://api.dingtalk.com/v1.0/rooms/meetingRoomLists?maxResults=100", tok)
}

func getToken(ak, as string) string {
	body, _ := json.Marshal(map[string]string{"appKey": ak, "appSecret": as})
	resp, err := http.Post("https://api.dingtalk.com/v1.0/oauth2/accessToken", "application/json", bytes.NewReader(body))
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out struct {
		AccessToken string `json:"accessToken"`
	}
	_ = json.Unmarshal(raw, &out)
	if out.AccessToken == "" {
		log.Fatalf("拿不到 token: %s", raw)
	}
	return out.AccessToken
}

func resolveUnion(tok, userid string) string {
	body, _ := json.Marshal(map[string]any{"userid": userid, "language": "zh_CN"})
	u := "https://oapi.dingtalk.com/topapi/v2/user/get?access_token=" + url.QueryEscape(tok)
	resp, err := http.Post(u, "application/json", bytes.NewReader(body))
	if err != nil {
		return "ERR:" + err.Error()
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out struct {
		Result struct {
			UnionID string `json:"unionid"`
		} `json:"result"`
	}
	_ = json.Unmarshal(raw, &out)
	if out.Result.UnionID == "" {
		return "EMPTY(raw=" + string(raw) + ")"
	}
	return out.Result.UnionID
}

func dumpGet(label, fullURL, tok string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	req.Header.Set("x-acs-dingtalk-access-token", tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("\n== %s ==\nERR %v\n", label, err)
		return
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	fmt.Printf("\n== %s ==\nHTTP %d\n%s\n", label, resp.StatusCode, string(raw))
}

func safePrefix(s string) string {
	if len(s) <= 6 {
		return "***"
	}
	return s[:6]
}
