package tools

import (
	"context"
	"fmt"

	activitiesdomain "github.com/standardsoftware/culture_points_mall/internal/modules/activities/domain"
	activitiessvc "github.com/standardsoftware/culture_points_mall/internal/modules/activities/service"
	pointssvc "github.com/standardsoftware/culture_points_mall/internal/modules/points/service"
	usersvc "github.com/standardsoftware/culture_points_mall/internal/modules/users/service"
	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

// batch.go：通用「批量管理」机制。open_*_batch 工具发出统一的 {form:"batch_form", ...} 信号，
// 前端用一个通用 BatchCard 渲染（勾选 + 操作 + 可选参数字段），与「批量管理商品」同款交互。
// 提交后回填给对应 batch_* 执行工具，执行结果带 _undo 支持回撤。

type BatchDeps struct {
	Points     *pointssvc.Service
	Users      *usersvc.Service
	Activities *activitiessvc.Service
}

func tenantOf(ctx context.Context) int64 {
	tid := cpmctx.TenantID(ctx)
	if tid == 0 {
		tid = 1
	}
	return tid
}

var activityStatusLabel = map[string]string{
	"draft": "草稿", "published": "已发布", "running": "进行中", "closed": "已关闭",
}

// ---- open_points_batch（信号：批量管理积分）----

type OpenPointsBatchTool struct{ Deps BatchDeps }

func (OpenPointsBatchTool) Name() string { return "open_points_batch" }
func (OpenPointsBatchTool) Description() string {
	return "当用户想批量给多位员工加分/扣分时调用，弹出员工批量表格供勾选，并填写分值/维度/事由后一次性执行。调用后只回一句话提示用户在表格里勾选员工、填写分值与维度，不要自己逐个加分。"
}
func (OpenPointsBatchTool) InputSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}
func (t OpenPointsBatchTool) Execute(ctx context.Context, _ map[string]any) (map[string]any, error) {
	tid := tenantOf(ctx)
	rows, err := t.Deps.Users.List(ctx, tid)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(rows))
	for _, u := range rows {
		bound := ""
		if u.DingUserID == "" {
			bound = "（未绑定钉钉）"
		}
		items = append(items, map[string]any{"id": u.ID, "name": u.Name + bound})
	}
	return map[string]any{
		"form": "batch_form", "intent": "batch_add_points", "idField": "user_ids", "title": "批量管理积分",
		"columns": []map[string]any{{"key": "name", "label": "员工"}},
		"items":   items,
		"actions": []map[string]any{
			{"value": "add", "label": "批量加/扣分", "fields": []map[string]any{
				{"field": "amount", "label": "分值（正数加分，负数扣分）", "type": "number", "placeholder": "如 50 或 -20", "required": true},
				{"field": "dimension_code", "label": "价值观维度", "type": "choice", "source": "dimensions", "required": true},
				{"field": "reason", "label": "事由（可选）", "type": "text", "placeholder": "如：季度优秀表现"},
			}},
		},
	}, nil
}

// ---- batch_add_points（执行）----

type BatchAddPointsTool struct{ Deps BatchDeps }

func (BatchAddPointsTool) Name() string { return "batch_add_points" }
func (BatchAddPointsTool) Description() string {
	return "给多位员工批量加/扣分：user_ids 数组 + amount（正加负扣）+ dimension_code，reason 可选。"
}
func (BatchAddPointsTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"user_ids":       map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
			"amount":         map[string]any{"type": "integer"},
			"dimension_code": map[string]any{"type": "string"},
			"reason":         map[string]any{"type": "string"},
		},
		"required": []string{"user_ids", "amount", "dimension_code"},
	}
}
func (t BatchAddPointsTool) Execute(ctx context.Context, in map[string]any) (map[string]any, error) {
	tid := tenantOf(ctx)
	amount := anyInt(in["amount"])
	dim := anyString(in["dimension_code"])
	reason := anyString(in["reason"])
	var uids []int64
	if arr, ok := in["user_ids"].([]any); ok {
		for _, v := range arr {
			uids = append(uids, int64(anyInt(v)))
		}
	}
	done := 0
	for _, uid := range uids {
		_, err := t.Deps.Points.AddPoints(ctx, pointssvc.AddPointsCmd{TenantID: tid, UserID: uid, Amount: amount, DimCode: dim, Reason: reason})
		if err == nil {
			done++
		}
	}
	return map[string]any{
		"action": "add", "count": done,
		"_undo": map[string]any{
			"label": fmt.Sprintf("撤销批量积分（%d 人）", done), "action": "points_batch_reverse",
			"params": map[string]any{"user_ids": uids, "amount": amount, "dimension_code": dim},
		},
	}, nil
}

// ---- open_activity_batch（信号：批量管理活动）----

type OpenActivityBatchTool struct{ Deps BatchDeps }

func (OpenActivityBatchTool) Name() string { return "open_activity_batch" }
func (OpenActivityBatchTool) Description() string {
	return "当用户想批量管理活动（批量关闭/重新发布）时调用，弹出活动批量表格供勾选。调用后只回一句话提示用户在表格里勾选活动并选择操作，不要自己逐个改。"
}
func (OpenActivityBatchTool) InputSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}
func (t OpenActivityBatchTool) Execute(ctx context.Context, _ map[string]any) (map[string]any, error) {
	tid := tenantOf(ctx)
	rows, err := t.Deps.Activities.List(ctx, tid, "")
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		label := activityStatusLabel[string(a.Status)]
		if label == "" {
			label = string(a.Status)
		}
		items = append(items, map[string]any{"id": a.ID, "title": a.Title, "status": label})
	}
	return map[string]any{
		"form": "batch_form", "intent": "batch_update_activities", "idField": "activity_ids", "title": "批量管理活动",
		"columns": []map[string]any{{"key": "title", "label": "活动"}, {"key": "status", "label": "状态"}},
		"items":   items,
		"actions": []map[string]any{
			{"value": "close", "label": "批量关闭"},
			{"value": "publish", "label": "批量重新发布"},
		},
	}, nil
}

// ---- batch_update_activities（执行）----

type BatchUpdateActivitiesTool struct{ Deps BatchDeps }

func (BatchUpdateActivitiesTool) Name() string { return "batch_update_activities" }
func (BatchUpdateActivitiesTool) Description() string {
	return "批量改活动状态：activity_ids 数组 + action（close 批量关闭 / publish 批量重新发布）。"
}
func (BatchUpdateActivitiesTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"activity_ids": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
			"action":       map[string]any{"type": "string", "enum": []string{"close", "publish"}},
		},
		"required": []string{"activity_ids", "action"},
	}
}
func (t BatchUpdateActivitiesTool) Execute(ctx context.Context, in map[string]any) (map[string]any, error) {
	tid := tenantOf(ctx)
	action := anyString(in["action"])
	target := activitiesdomain.StatusClosed
	if action == "publish" {
		target = activitiesdomain.StatusPublished
	}
	var ids []int64
	if arr, ok := in["activity_ids"].([]any); ok {
		for _, v := range arr {
			ids = append(ids, int64(anyInt(v)))
		}
	}
	undos := make([]map[string]any, 0, len(ids))
	done := 0
	for _, id := range ids {
		prev, err := t.Deps.Activities.SetStatus(ctx, tid, id, target)
		if err != nil {
			continue
		}
		undos = append(undos, map[string]any{"activity_id": id, "status": string(prev)})
		done++
	}
	return map[string]any{
		"action": action, "count": done,
		"_undo": map[string]any{
			"label": fmt.Sprintf("撤销批量活动操作（%d 个）", done), "action": "activity_batch_restore",
			"params": map[string]any{"items": undos},
		},
	}, nil
}

func RegisterBatch(r *Registry, deps BatchDeps) {
	r.MustRegister(OpenPointsBatchTool{deps})
	r.MustRegister(BatchAddPointsTool{deps})
	r.MustRegister(OpenActivityBatchTool{deps})
	r.MustRegister(BatchUpdateActivitiesTool{deps})
}
