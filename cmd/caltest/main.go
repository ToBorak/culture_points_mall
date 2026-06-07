// caltest 是一次性的真钉钉日历创建脚本（手动验证 CreateCalendarEvent）。
// 用法：在仓库根目录 `go run ./cmd/caltest`，读 ./configs/config.yaml（需 mode:real）。
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/standardsoftware/culture_points_mall/internal/config"
	"github.com/standardsoftware/culture_points_mall/internal/platform/dingtalk"
	"github.com/standardsoftware/culture_points_mall/internal/platform/storage"
)

func main() {
	cfg, err := config.Load("./configs")
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	rdb, err := storage.NewRedis(cfg.Redis)
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
	ding := dingtalk.NewReal(cfg.DingTalk, rdb)

	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	now := time.Now().In(loc)
	start := time.Date(now.Year(), now.Month(), now.Day()+1, 15, 0, 0, 0, loc)
	end := start.Add(time.Hour)

	eventID, err := ding.CreateCalendarEvent(context.Background(), dingtalk.CalendarRequest{
		Title:    "文化官 · 团队日程测试",
		Detail:   "这是通过文化官系统创建的第一个真实钉钉日程。",
		StartAt:  start,
		EndAt:    end,
		Location: "线上会议",
		UserIDs:  []string{"083221474637757281", "02562843492437802982"},
	})
	if err != nil {
		log.Fatalf("CreateCalendarEvent failed: %v", err)
	}
	fmt.Printf("✅ 日程已创建\n  eventId = %s\n  时间    = %s ~ %s\n", eventID, start.Format("2006-01-02 15:04"), end.Format("15:04"))
}
