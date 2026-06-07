// roomtest 是一次性的真钉钉智能会议室手验脚本：列会议室 + 建一个带会议室的日程。
// 用法：仓库根目录 `go run ./cmd/roomtest`，读 ./configs/config.yaml（需 mode:real）。
// 验证点：建完日程后到钉钉日历查看，会议室名称应已自动附在日程上。
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
	ctx := context.Background()

	// 组织者 unionId：优先用配置 calendar_organizer_unionid；否则用下面这个 corp userid 解析。
	organizerUserID := "083221474637757281" // 按需替换为你自己的 userid（与 caltest 一致）
	organizerUnion := cfg.DingTalk.CalendarOrganizerUnionID
	if organizerUnion == "" {
		organizerUnion, err = ding.ResolveUnionID(ctx, organizerUserID)
		if err != nil {
			log.Fatalf("ResolveUnionID failed: %v", err)
		}
	}
	fmt.Printf("organizer unionId = %s\n", organizerUnion)

	// 1) 列出可见的智能会议室
	rooms, err := ding.QueryMeetingRooms(ctx, organizerUnion)
	if err != nil {
		log.Fatalf("QueryMeetingRooms failed: %v", err)
	}
	fmt.Printf("共 %d 个可见会议室：\n", len(rooms))
	for _, r := range rooms {
		fmt.Printf("  - %s (id=%s, 容纳%d人, %s)\n", r.RoomName, r.RoomID, r.Capacity, r.Location)
	}
	if len(rooms) == 0 {
		log.Fatal("没有可见会议室：确认企业已登记智能会议室、且应用已授权会议室相关权限")
	}

	// 2) 建一个带会议室的日程（取第一个会议室）
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	now := time.Now().In(loc)
	start := time.Date(now.Year(), now.Month(), now.Day()+1, 16, 0, 0, 0, loc)
	end := start.Add(time.Hour)

	pick := rooms[0]
	eventID, err := ding.CreateCalendarEvent(ctx, dingtalk.CalendarRequest{
		Title:    "文化官 · 会议室预定测试",
		Detail:   "验证通过 API 预定智能会议室，并让日程自动附上会议室名称。",
		StartAt:  start,
		EndAt:    end,
		Location: pick.RoomName,
		UserIDs:  []string{organizerUserID},
		RoomIDs:  []string{pick.RoomID},
	})
	if err != nil {
		log.Fatalf("CreateCalendarEvent(带会议室) failed: %v", err)
	}
	fmt.Printf("✅ 已创建带会议室的日程\n  eventId = %s\n  会议室  = %s (%s)\n  时间    = %s ~ %s\n",
		eventID, pick.RoomName, pick.RoomID, start.Format("2006-01-02 15:04"), end.Format("15:04"))
	fmt.Println("👉 到钉钉日历查看该日程，会议室名称应已自动附上。")
}
