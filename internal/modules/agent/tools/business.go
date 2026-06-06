package tools

import (
	"context"
	"time"

	activitiesdomain "github.com/standardsoftware/culture_points_mall/internal/modules/activities/domain"
	activitiessvc "github.com/standardsoftware/culture_points_mall/internal/modules/activities/service"
	achvsvc "github.com/standardsoftware/culture_points_mall/internal/modules/achievements/service"
	lbsvc "github.com/standardsoftware/culture_points_mall/internal/modules/leaderboard/service"
	pointssvc "github.com/standardsoftware/culture_points_mall/internal/modules/points/service"
	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

type BusinessDeps struct {
	Activities   *activitiessvc.Service
	Points       *pointssvc.Service
	Leaderboard  *lbsvc.Service
	Achievements *achvsvc.Service
}

// ---- create_activity ----

type CreateActivityTool struct{ Deps BusinessDeps }

func (CreateActivityTool) Name() string        { return "create_activity" }
func (CreateActivityTool) Description() string { return "创建一个文化运营活动，必须绑定一个价值观维度。" }
func (CreateActivityTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"dimension_code": map[string]any{"type": "string", "description": "价值观维度代码，例如 team_collab"},
			"title":          map[string]any{"type": "string"},
			"start_at":       map[string]any{"type": "string", "description": "RFC3339 时间"},
			"end_at":         map[string]any{"type": "string"},
			"capacity":       map[string]any{"type": "integer"},
			"points_reward":  map[string]any{"type": "integer"},
		},
		"required": []string{"dimension_code", "title"},
	}
}

func (t CreateActivityTool) Execute(ctx context.Context, in map[string]any) (map[string]any, error) {
	tid := cpmctx.TenantID(ctx)
	if tid == 0 {
		tid = 1
	}
	cmd := activitiessvc.CreateCmd{
		TenantID:      tid,
		DimensionCode: anyString(in["dimension_code"]),
		Title:         anyString(in["title"]),
		PointsReward:  anyInt(in["points_reward"]),
	}
	if s := anyString(in["start_at"]); s != "" {
		if ts, err := time.Parse(time.RFC3339, s); err == nil {
			cmd.StartAt = &ts
		}
	}
	if s := anyString(in["end_at"]); s != "" {
		if ts, err := time.Parse(time.RFC3339, s); err == nil {
			cmd.EndAt = &ts
		}
	}
	if cap := anyInt(in["capacity"]); cap > 0 {
		cmd.Capacity = &cap
	}
	a, err := t.Deps.Activities.Create(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return map[string]any{"activity_id": a.ID, "title": a.Title, "status": string(a.Status), "dimension_id": a.DimensionID}, nil
}

// ---- open_activity_form ----

// OpenActivityFormTool 不直接建活动，而是给前端发"渲染日程表单"的信号。
// HR 表达发布/创建活动意图时，LLM 调它弹表单；用户在表单里选时间/会议室/人员后由前端走发布接口执行。
type OpenActivityFormTool struct{}

func (OpenActivityFormTool) Name() string { return "open_activity_form" }
func (OpenActivityFormTool) Description() string {
	return "当 HR 想发布或创建一个活动时调用本工具，会在对话里弹出一张日程表单，让用户选择时间、会议室、参与人员（默认全员）。调用后只需用一句话提示用户填写表单，不要再自己调用 create_activity。可把已从用户话里识别到的标题/维度/时间作为预填项传入。"
}
func (OpenActivityFormTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title":          map[string]any{"type": "string", "description": "活动标题（若用户已提到）"},
			"dimension_code": map[string]any{"type": "string", "description": "价值观维度代码（若能判断）"},
			"start_at":       map[string]any{"type": "string", "description": "RFC3339 开始时间（若用户已提到）"},
			"end_at":         map[string]any{"type": "string", "description": "RFC3339 结束时间（若用户已提到）"},
			"points_reward":  map[string]any{"type": "integer", "description": "奖励积分（若用户已提到）"},
		},
	}
}

func (OpenActivityFormTool) Execute(_ context.Context, in map[string]any) (map[string]any, error) {
	prefill := map[string]any{}
	for _, k := range []string{"title", "dimension_code", "start_at", "end_at", "points_reward"} {
		if v, ok := in[k]; ok {
			prefill[k] = v
		}
	}
	return map[string]any{"form": "activity_schedule", "prefill": prefill}, nil
}

// ---- list_activities ----

type ListActivitiesTool struct{ Deps BusinessDeps }

func (ListActivitiesTool) Name() string        { return "list_activities" }
func (ListActivitiesTool) Description() string { return "列出当前租户的活动，支持按 status 过滤" }
func (ListActivitiesTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"status": map[string]any{"type": "string", "enum": []string{"", "draft", "published", "running", "closed"}},
		},
	}
}

func (t ListActivitiesTool) Execute(ctx context.Context, in map[string]any) (map[string]any, error) {
	tid := cpmctx.TenantID(ctx)
	if tid == 0 {
		tid = 1
	}
	rows, err := t.Deps.Activities.List(ctx, tid, activitiesdomain.Status(anyString(in["status"])))
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"id": r.ID, "title": r.Title, "status": string(r.Status), "dimension_id": r.DimensionID,
		})
	}
	return map[string]any{"items": out, "total": len(out)}, nil
}

// ---- get_user_points ----

type GetUserPointsTool struct{ Deps BusinessDeps }

func (GetUserPointsTool) Name() string        { return "get_user_points" }
func (GetUserPointsTool) Description() string { return "查询某员工的总积分与按维度的分值" }
func (GetUserPointsTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{"user_id": map[string]any{"type": "integer"}},
		"required":   []string{"user_id"},
	}
}

func (t GetUserPointsTool) Execute(ctx context.Context, in map[string]any) (map[string]any, error) {
	tid := cpmctx.TenantID(ctx)
	if tid == 0 {
		tid = 1
	}
	uid := int64(anyInt(in["user_id"]))
	scores, _, total, err := t.Deps.Points.GetUserScores(ctx, tid, uid)
	if err != nil {
		return nil, err
	}
	return map[string]any{"user_id": uid, "total": total, "scores": scores}, nil
}

// ---- add_points ----

type AddPointsTool struct{ Deps BusinessDeps }

func (AddPointsTool) Name() string        { return "add_points" }
func (AddPointsTool) Description() string { return "为某员工加分（必须指定维度）" }
func (AddPointsTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"user_id":        map[string]any{"type": "integer"},
			"amount":         map[string]any{"type": "integer", "description": "正数加分，负数扣分"},
			"dimension_code": map[string]any{"type": "string"},
			"reason":         map[string]any{"type": "string"},
		},
		"required": []string{"user_id", "amount", "dimension_code"},
	}
}

func (t AddPointsTool) Execute(ctx context.Context, in map[string]any) (map[string]any, error) {
	tid := cpmctx.TenantID(ctx)
	if tid == 0 {
		tid = 1
	}
	tx, err := t.Deps.Points.AddPoints(ctx, pointssvc.AddPointsCmd{
		TenantID: tid, UserID: int64(anyInt(in["user_id"])), Amount: anyInt(in["amount"]),
		DimCode: anyString(in["dimension_code"]), Reason: anyString(in["reason"]),
	})
	if err != nil {
		return nil, err
	}
	newBadges, _ := t.Deps.Achievements.CheckTriggers(ctx, tid, tx.UserID, tx.DimensionID)
	return map[string]any{"transaction_id": tx.ID, "new_badges": newBadges}, nil
}

// ---- get_leaderboard ----

type GetLeaderboardTool struct{ Deps BusinessDeps }

func (GetLeaderboardTool) Name() string        { return "get_leaderboard" }
func (GetLeaderboardTool) Description() string { return "查询排行榜：scope=total/dim/dept" }
func (GetLeaderboardTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"scope":        map[string]any{"type": "string", "enum": []string{"total", "dim", "dept"}},
			"dimension_id": map[string]any{"type": "integer"},
			"limit":        map[string]any{"type": "integer"},
		},
		"required": []string{"scope"},
	}
}

func (t GetLeaderboardTool) Execute(ctx context.Context, in map[string]any) (map[string]any, error) {
	tid := cpmctx.TenantID(ctx)
	if tid == 0 {
		tid = 1
	}
	rows, err := t.Deps.Leaderboard.List(ctx, lbsvc.ListParams{
		TenantID: tid, Scope: anyString(in["scope"]),
		DimensionID: int64(anyInt(in["dimension_id"])),
		Limit:       anyInt(in["limit"]),
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"entries": rows}, nil
}

// ---- award_badge ----

type AwardBadgeTool struct{ Deps BusinessDeps }

func (AwardBadgeTool) Name() string        { return "award_badge" }
func (AwardBadgeTool) Description() string { return "直接为员工颁发指定徽章" }
func (AwardBadgeTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"user_id":  map[string]any{"type": "integer"},
			"badge_id": map[string]any{"type": "integer"},
		},
		"required": []string{"user_id", "badge_id"},
	}
}

func (t AwardBadgeTool) Execute(ctx context.Context, in map[string]any) (map[string]any, error) {
	uid := int64(anyInt(in["user_id"]))
	bid := int64(anyInt(in["badge_id"]))
	if err := t.Deps.Achievements.AwardBadge(ctx, uid, bid); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

func RegisterBusiness(r *Registry, deps BusinessDeps) {
	r.MustRegister(CreateActivityTool{deps})
	r.MustRegister(OpenActivityFormTool{})
	r.MustRegister(ListActivitiesTool{deps})
	r.MustRegister(GetUserPointsTool{deps})
	r.MustRegister(AddPointsTool{deps})
	r.MustRegister(GetLeaderboardTool{deps})
	r.MustRegister(AwardBadgeTool{deps})
}

func anyString(v any) string {
	s, _ := v.(string)
	return s
}

func anyInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	default:
		return 0
	}
}
