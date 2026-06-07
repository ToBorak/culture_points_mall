package handler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"

	activitiesdomain "github.com/standardsoftware/culture_points_mall/internal/modules/activities/domain"
	activitiessvc "github.com/standardsoftware/culture_points_mall/internal/modules/activities/service"
	mallsvc "github.com/standardsoftware/culture_points_mall/internal/modules/mall/service"
	pointssvc "github.com/standardsoftware/culture_points_mall/internal/modules/points/service"
	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

// UndoHandler 实现「回撤」：把某次修改类操作精确逆转。不经 LLM，按 action 确定性分发到对应服务。
// 各修改类工具在结果里返回 _undo:{label,action,params}，前端「回撤」按钮原样回传到这里执行。
type UndoDeps struct {
	Points     *pointssvc.Service
	Mall       *mallsvc.Service
	Activities *activitiessvc.Service // 活动回撤（删活动 / 还原状态），尽力而为，可为 nil
}

type UndoHandler struct{ Deps UndoDeps }

func NewUndo(d UndoDeps) *UndoHandler { return &UndoHandler{Deps: d} }

func (h *UndoHandler) Register(rg *gin.RouterGroup) { rg.POST("/admin/agent/undo", h.undo) }

type undoReq struct {
	Action string         `json:"action" binding:"required"`
	Params map[string]any `json:"params"`
	Label  string         `json:"label"`
}

func (h *UndoHandler) undo(c *gin.Context) {
	var req undoReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Writer.Flush()
	emit := func(step map[string]any) {
		raw, _ := json.Marshal(step)
		_, _ = c.Writer.Write([]byte("event: step\ndata: "))
		_, _ = c.Writer.Write(raw)
		_, _ = c.Writer.Write([]byte("\n\n"))
		c.Writer.Flush()
	}

	emit(map[string]any{"kind": "tool_use", "toolName": "undo", "input": map[string]any{"action": req.Action}})
	note, err := h.apply(c.Request.Context(), req.Action, req.Params)
	label := req.Label
	if label == "" {
		label = "上一步操作"
	}
	if err != nil {
		emit(map[string]any{"kind": "tool_result", "toolName": "undo", "error": err.Error()})
		emit(map[string]any{"kind": "llm_text", "text": "⚠️ 回撤失败：" + err.Error()})
	} else {
		emit(map[string]any{"kind": "tool_result", "toolName": "undo", "output": map[string]any{"ok": true}})
		msg := "↩️ 已回撤：" + label
		if note != "" {
			msg += "（" + note + "）"
		}
		emit(map[string]any{"kind": "llm_text", "text": msg})
	}
	emit(map[string]any{"kind": "done"})
}

func (h *UndoHandler) apply(ctx context.Context, action string, params map[string]any) (string, error) {
	tid := cpmctx.TenantID(ctx)
	if tid == 0 {
		tid = 1
	}
	switch action {
	case "points_reverse":
		_, err := h.Deps.Points.AddPoints(ctx, pointssvc.AddPointsCmd{
			TenantID: tid, UserID: int64(toInt(params["user_id"])), Amount: -toInt(params["amount"]),
			DimCode: toStr(params["dimension_code"]), Reason: "撤销上一次积分变动",
		})
		return "", err
	case "mall_set_status":
		_, err := h.Deps.Mall.SetItemStatus(ctx, tid, int64(toInt(params["item_id"])), toStr(params["status"]))
		return "", err
	case "mall_delete":
		return "", h.Deps.Mall.DeleteItem(ctx, tid, int64(toInt(params["item_id"])))
	case "mall_restore":
		name := toStr(params["name"])
		cost := toInt(params["cost"])
		img := toStr(params["image_url"])
		cmd := mallsvc.UpdateItemCmd{
			TenantID: tid, ItemID: int64(toInt(params["item_id"])),
			Name: &name, Cost: &cost, ImageURL: &img, StockSet: true,
		}
		if params["stock"] != nil { // nil 表示原本就是不限量
			n := toInt(params["stock"])
			cmd.Stock = &n
		}
		_, _, err := h.Deps.Mall.UpdateItem(ctx, cmd)
		return "", err
	case "batch":
		arr, _ := params["undos"].([]any)
		failed := 0
		for _, u := range arr {
			m, _ := u.(map[string]any)
			if m == nil {
				continue
			}
			if _, err := h.apply(ctx, toStr(m["action"]), toMap(m["params"])); err != nil {
				failed++
			}
		}
		if failed > 0 {
			return fmt.Sprintf("%d 项还原失败", failed), nil
		}
		return "", nil
	case "activity_delete":
		if h.Deps.Activities == nil {
			return "", fmt.Errorf("活动回撤未启用")
		}
		if err := h.Deps.Activities.Delete(ctx, tid, int64(toInt(params["activity_id"]))); err != nil {
			return "", err
		}
		return "已删除活动记录；已发出的钉钉日程/群通知无法自动撤回，请在钉钉中手动处理", nil
	case "points_batch_reverse":
		amount := toInt(params["amount"])
		dim := toStr(params["dimension_code"])
		arr, _ := params["user_ids"].([]any)
		for _, v := range arr {
			_, _ = h.Deps.Points.AddPoints(ctx, pointssvc.AddPointsCmd{
				TenantID: tid, UserID: int64(toInt(v)), Amount: -amount, DimCode: dim, Reason: "撤销批量积分",
			})
		}
		return "", nil
	case "activity_batch_restore":
		if h.Deps.Activities == nil {
			return "", fmt.Errorf("活动回撤未启用")
		}
		arr, _ := params["items"].([]any)
		for _, it := range arr {
			m, _ := it.(map[string]any)
			if m == nil {
				continue
			}
			_, _ = h.Deps.Activities.SetStatus(ctx, tid, int64(toInt(m["activity_id"])), activitiesdomain.Status(toStr(m["status"])))
		}
		return "", nil
	}
	return "", fmt.Errorf("未知回撤操作: %s", action)
}

// JSON 数字默认 float64，这里统一收口成 int / string / map。
func toInt(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case int64:
		return int(x)
	case json.Number:
		n, _ := x.Int64()
		return int(n)
	default:
		return 0
	}
}

func toStr(v any) string {
	s, _ := v.(string)
	return s
}

func toMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}
