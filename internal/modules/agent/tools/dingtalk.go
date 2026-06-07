package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/standardsoftware/culture_points_mall/internal/platform/dingtalk"
)

type DingDeps struct {
	Client dingtalk.Client
}

// ---- send_dingtalk_card ----

type SendDingtalkCardTool struct{ Deps DingDeps }

func (SendDingtalkCardTool) Name() string        { return "send_dingtalk_card" }
func (SendDingtalkCardTool) Description() string { return "向某用户发送互动卡片" }
func (SendDingtalkCardTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"target": map[string]any{"type": "string", "description": "钉钉 userId"},
			"title":  map[string]any{"type": "string"},
			"detail": map[string]any{"type": "string"},
		},
		"required": []string{"target", "title"},
	}
}

func (t SendDingtalkCardTool) Execute(ctx context.Context, in map[string]any) (map[string]any, error) {
	inst, err := t.Deps.Client.SendInteractiveCard(ctx,
		anyString(in["target"]), "default",
		map[string]any{"title": anyString(in["title"]), "detail": anyString(in["detail"])})
	if err != nil {
		return nil, err
	}
	return map[string]any{"card_instance_id": inst.InstanceID}, nil
}

// ---- bot_broadcast ----

type BotBroadcastTool struct{ Deps DingDeps }

func (BotBroadcastTool) Name() string        { return "dingtalk_bot_broadcast" }
func (BotBroadcastTool) Description() string { return "用群机器人在指定群播报消息" }
func (BotBroadcastTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"group_id": map[string]any{"type": "string"},
			"title":    map[string]any{"type": "string"},
			"detail":   map[string]any{"type": "string"},
		},
		"required": []string{"group_id", "title"},
	}
}

func (t BotBroadcastTool) Execute(ctx context.Context, in map[string]any) (map[string]any, error) {
	if err := t.Deps.Client.BotBroadcast(ctx, anyString(in["group_id"]),
		dingtalk.Card{Title: anyString(in["title"]), Detail: anyString(in["detail"])}); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// ---- create_dingtalk_calendar ----

type CreateDingtalkCalendarTool struct{ Deps DingDeps }

func (CreateDingtalkCalendarTool) Name() string        { return "create_dingtalk_calendar" }
func (CreateDingtalkCalendarTool) Description() string { return "为目标员工创建钉钉日程（接受=报名）" }
func (CreateDingtalkCalendarTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title":    map[string]any{"type": "string"},
			"start_at": map[string]any{"type": "string", "description": "RFC3339"},
			"end_at":   map[string]any{"type": "string"},
			"user_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"location": map[string]any{"type": "string"},
			"detail":   map[string]any{"type": "string"},
			"room_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "钉钉智能会议室 roomId 列表（可空）；表单「会议室」选了就带上，会自动预定会议室"},
		},
		"required": []string{"title", "start_at", "end_at", "user_ids"},
	}
}

func (t CreateDingtalkCalendarTool) Execute(ctx context.Context, in map[string]any) (map[string]any, error) {
	startStr := anyString(in["start_at"])
	endStr := anyString(in["end_at"])
	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		return nil, fmt.Errorf("start_at parse: %w", err)
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		return nil, fmt.Errorf("end_at parse: %w", err)
	}
	var userIDs []string
	if arr, ok := in["user_ids"].([]any); ok {
		for _, v := range arr {
			userIDs = append(userIDs, anyString(v))
		}
	}
	// room_ids 容错：LLM 可能传数组，也可能（单选会议室时）传单个字符串
	var roomIDs []string
	switch v := in["room_ids"].(type) {
	case []any:
		for _, x := range v {
			if s := anyString(x); s != "" {
				roomIDs = append(roomIDs, s)
			}
		}
	case string:
		if v != "" {
			roomIDs = append(roomIDs, v)
		}
	}
	id, err := t.Deps.Client.CreateCalendarEvent(ctx, dingtalk.CalendarRequest{
		Title: anyString(in["title"]), StartAt: start, EndAt: end, UserIDs: userIDs,
		Location: anyString(in["location"]), Detail: anyString(in["detail"]), RoomIDs: roomIDs,
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"event_id": id}, nil
}

func RegisterDingtalk(r *Registry, deps DingDeps) {
	r.MustRegister(SendDingtalkCardTool{deps})
	r.MustRegister(BotBroadcastTool{deps})
	r.MustRegister(CreateDingtalkCalendarTool{deps})
}
