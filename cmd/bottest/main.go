// bottest 一次性验证：通过群自定义机器人推一条 markdown 消息。
// 用法：仓库根目录 `go run ./cmd/bottest [群id]`（默认 culture），读 ./configs/config.yaml。
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/standardsoftware/culture_points_mall/internal/config"
	"github.com/standardsoftware/culture_points_mall/internal/platform/dingtalk"
)

func main() {
	cfg, err := config.Load("./configs")
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	groupID := "culture"
	if len(os.Args) > 1 {
		groupID = os.Args[1]
	}
	// BotBroadcast 只用 webhook+加签，不需要 Redis/企业 token，故 rdb 传 nil。
	ding := dingtalk.NewReal(cfg.DingTalk, nil)

	err = ding.BotBroadcast(context.Background(), groupID, dingtalk.Card{
		Title:  "文化官 · 日程提醒",
		Detail: "**团队周会**\n\n- 时间：明天 15:00 ~ 16:00\n- 地点：线上会议\n\n> 这是文化官系统通过群机器人推送的第一条消息 🎉",
	})
	if err != nil {
		log.Fatalf("BotBroadcast failed: %v", err)
	}
	fmt.Printf("✅ 已推送到群（id=%s）\n", groupID)
}
