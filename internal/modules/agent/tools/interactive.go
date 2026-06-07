package tools

import (
	"context"
	"fmt"
	"strings"

	achvsvc "github.com/standardsoftware/culture_points_mall/internal/modules/achievements/service"
	usersvc "github.com/standardsoftware/culture_points_mall/internal/modules/users/service"
	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

// interactive.go 提供"对话式收集信息"的工具：
//   - ask_user           缺 1-3 个必填项时，弹可点击的选择题/填空逐项问
//   - open_points_form   弹"积分加减"整卡让用户一次填全
//   - open_mall_item_form 弹"新增商品"整卡
//   - open_award_badge_form 弹"颁发徽章"整卡
//   - list_users / list_badges  把人名/徽章名解析成 ID（避免臆造）
//
// ask_user 与 open_*_form 都不执行业务，只返回 {form:"slot_form", ...} 信号；前端用统一的
// SlotForm 组件渲染，用户提交后把答案拼成一段结构化文本作为新一轮消息发回，LLM 据此调真正的执行工具。

type InteractiveDeps struct {
	Users        *usersvc.Service
	Achievements *achvsvc.Service
}

// ---- ask_user ----

type AskUserTool struct{}

func (AskUserTool) Name() string { return "ask_user" }
func (AskUserTool) Description() string {
	return "当你缺少完成某操作所必需的信息、或需要用户在若干候选里做选择时调用本工具，会在对话里渲染一组可点击的选择题/填空让用户直接作答。一次最多问 1-3 个关键问题；能列出候选就用 choice 并给 options，尽量减少用户打字。调用后只回一句话提示用户在下面作答，不要臆造答案、也不要在同一轮里继续调用执行工具。"
}
func (AskUserTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"intent": map[string]any{"type": "string", "description": "本次要完成的操作（如 add_points / create_mall_item / award_badge），收齐信息后据此执行"},
			"title":  map[string]any{"type": "string", "description": "可选，提问卡片标题"},
			"questions": map[string]any{
				"type":        "array",
				"description": "1-3 个问题，缺什么问什么",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"field": map[string]any{"type": "string", "description": "对应执行工具的参数名，如 dimension_code / amount / user_id"},
						"label": map[string]any{"type": "string", "description": "问题文案"},
						"type":  map[string]any{"type": "string", "enum": []string{"choice", "text", "number"}, "description": "choice=单/多选，text=填空，number=数字"},
						"options": map[string]any{
							"type":        "array",
							"description": "type=choice 时的候选项",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"value": map[string]any{"type": "string", "description": "回填给参数的真实值，如维度 code、user_id"},
									"label": map[string]any{"type": "string", "description": "展示给用户的文案"},
								},
							},
						},
						"multi":       map[string]any{"type": "boolean", "description": "choice 是否允许多选"},
						"placeholder": map[string]any{"type": "string"},
					},
					"required": []string{"field", "label", "type"},
				},
			},
		},
		"required": []string{"questions"},
	}
}
func (AskUserTool) Execute(_ context.Context, in map[string]any) (map[string]any, error) {
	title := anyString(in["title"])
	if title == "" {
		title = "请补充以下信息"
	}
	return map[string]any{
		"form":   "slot_form",
		"source": "ask",
		"intent": anyString(in["intent"]),
		"title":  title,
		"fields": in["questions"],
	}, nil
}

// ---- open_points_form ----

type OpenPointsFormTool struct{}

func (OpenPointsFormTool) Name() string { return "open_points_form" }
func (OpenPointsFormTool) Description() string {
	return "当要给员工加分/扣分、但需要用户一次性填全（或用户说想自己填表单）时调用，弹出一张积分加减表单。可把已识别到的 user_id/amount/dimension_code/reason 作为预填项传入。调用后只回一句话提示用户在下面填写，不要再自己调用 add_points。"
}
func (OpenPointsFormTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"prefill": map[string]any{"type": "object", "description": "预填项，可含 user_id / amount / dimension_code / reason"},
		},
	}
}
func (OpenPointsFormTool) Execute(_ context.Context, in map[string]any) (map[string]any, error) {
	return map[string]any{
		"form":   "slot_form",
		"source": "open",
		"intent": "add_points",
		"title":  "积分加减",
		"fields": []map[string]any{
			{"field": "user_id", "label": "给谁加/扣分", "type": "choice", "source": "users", "required": true},
			{"field": "amount", "label": "分值（正数加分，负数扣分）", "type": "number", "required": true, "placeholder": "如 100 或 -50"},
			{"field": "dimension_code", "label": "价值观维度", "type": "choice", "source": "dimensions", "required": true},
			{"field": "reason", "label": "事由（可选）", "type": "text", "placeholder": "如：季度优秀表现"},
		},
		"prefill": in["prefill"],
	}, nil
}

// ---- open_mall_item_form ----

type OpenMallItemFormTool struct{}

func (OpenMallItemFormTool) Name() string { return "open_mall_item_form" }
func (OpenMallItemFormTool) Description() string {
	return "当要新增积分商城商品、但需要用户一次性填全（或用户说想自己填表单）时调用，弹出一张新增商品表单。可把已识别到的 type/name/cost/stock 作为预填项传入。调用后只回一句话提示用户在下面填写，不要再自己调用 create_mall_item。"
}
func (OpenMallItemFormTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"prefill": map[string]any{"type": "object", "description": "预填项，可含 type / name / cost / stock / image_url"},
		},
	}
}
func (OpenMallItemFormTool) Execute(_ context.Context, in map[string]any) (map[string]any, error) {
	return map[string]any{
		"form":   "slot_form",
		"source": "open",
		"intent": "create_mall_item",
		"title":  "新增积分商品",
		"fields": []map[string]any{
			{"field": "type", "label": "商品类型", "type": "choice", "required": true, "options": []map[string]any{
				{"value": "item", "label": "普通兑换商品"},
				{"value": "blindbox", "label": "盲盒"},
			}},
			{"field": "name", "label": "商品名称", "type": "text", "required": true, "placeholder": "如：定制保温杯"},
			{"field": "cost", "label": "兑换所需积分", "type": "number", "required": true, "placeholder": "必须大于 0"},
			{"field": "stock", "label": "库存（留空=不限量）", "type": "number"},
			{"field": "image_url", "label": "图片 URL（可选）", "type": "text"},
		},
		"prefill": in["prefill"],
	}, nil
}

// ---- open_schedule_form ----

type OpenScheduleFormTool struct{}

func (OpenScheduleFormTool) Name() string { return "open_schedule_form" }
func (OpenScheduleFormTool) Description() string {
	return "当用户想创建/安排一个钉钉日程或日历（不是文化活动）时调用，弹出日程表单：标题、开始/结束时间（时间选择器）、参与人员（成员多选，默认全体人员）、会议室（可选，下拉单选）。调用后只回一句话提示用户在下面填写，不要再自己调用 create_dingtalk_calendar。可把已识别到的 title/start_at/end_at 作为预填项传入。"
}
func (OpenScheduleFormTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"prefill": map[string]any{"type": "object", "description": "预填项，可含 title / start_at / end_at"},
		},
	}
}
func (OpenScheduleFormTool) Execute(_ context.Context, in map[string]any) (map[string]any, error) {
	return map[string]any{
		"form":   "slot_form",
		"source": "open",
		"intent": "create_dingtalk_calendar",
		"title":  "创建钉钉日程",
		"fields": []map[string]any{
			{"field": "title", "label": "日程标题", "type": "text", "required": true, "placeholder": "如：季度复盘会"},
			{"field": "start_at", "label": "开始时间", "type": "datetime", "required": true},
			{"field": "end_at", "label": "结束时间", "type": "datetime", "required": true},
			{"field": "user_ids", "label": "参与人员", "type": "choice", "source": "users", "multi": true, "valueKey": "ding", "allowAll": true, "required": true},
			{"field": "room_ids", "label": "会议室（可选）", "type": "choice", "source": "rooms"},
		},
		"prefill": in["prefill"],
	}, nil
}

// ---- open_award_badge_form ----

type OpenAwardBadgeFormTool struct{ Deps InteractiveDeps }

func (OpenAwardBadgeFormTool) Name() string { return "open_award_badge_form" }
func (OpenAwardBadgeFormTool) Description() string {
	return "当要给员工颁发徽章、但需要用户在成员/徽章里挑选（或用户说想自己填表单）时调用，弹出一张颁发徽章表单，徽章选项已自动列好。调用后只回一句话提示用户在下面填写，不要再自己调用 award_badge。"
}
func (OpenAwardBadgeFormTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"prefill": map[string]any{"type": "object", "description": "预填项，可含 user_id / badge_id"},
		},
	}
}
func (t OpenAwardBadgeFormTool) Execute(ctx context.Context, in map[string]any) (map[string]any, error) {
	tid := cpmctx.TenantID(ctx)
	if tid == 0 {
		tid = 1
	}
	badgeOpts := []map[string]any{}
	if t.Deps.Achievements != nil {
		badges, _, err := t.Deps.Achievements.ListMyBadges(ctx, tid, 0)
		if err == nil {
			for _, b := range badges {
				badgeOpts = append(badgeOpts, map[string]any{"value": fmt.Sprintf("%d", b.ID), "label": b.Name})
			}
		}
	}
	return map[string]any{
		"form":   "slot_form",
		"source": "open",
		"intent": "award_badge",
		"title":  "颁发徽章",
		"fields": []map[string]any{
			{"field": "user_id", "label": "颁发给谁", "type": "choice", "source": "users", "required": true},
			{"field": "badge_id", "label": "选择徽章", "type": "choice", "options": badgeOpts, "required": true},
		},
		"prefill": in["prefill"],
	}, nil
}

// ---- list_users ----

type ListUsersTool struct{ Deps InteractiveDeps }

func (ListUsersTool) Name() string { return "list_users" }
func (ListUsersTool) Description() string {
	return "列出当前公司全部成员（含 user_id、姓名、是否已绑定钉钉），用于把人名解析成 user_id，不要臆造 user_id。可传 name 做姓名模糊过滤。"
}
func (ListUsersTool) InputSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{"name": map[string]any{"type": "string", "description": "可选，姓名模糊过滤"}},
	}
}
func (t ListUsersTool) Execute(ctx context.Context, in map[string]any) (map[string]any, error) {
	tid := cpmctx.TenantID(ctx)
	if tid == 0 {
		tid = 1
	}
	rows, err := t.Deps.Users.List(ctx, tid)
	if err != nil {
		return nil, err
	}
	kw := strings.TrimSpace(anyString(in["name"]))
	out := make([]map[string]any, 0, len(rows))
	for _, u := range rows {
		if kw != "" && !strings.Contains(u.Name, kw) {
			continue
		}
		out = append(out, map[string]any{"id": u.ID, "name": u.Name, "ding_user_id": u.DingUserID})
	}
	return map[string]any{"items": out, "total": len(out)}, nil
}

// ---- list_badges ----

type ListBadgesTool struct{ Deps InteractiveDeps }

func (ListBadgesTool) Name() string { return "list_badges" }
func (ListBadgesTool) Description() string {
	return "列出当前公司的徽章（含 badge_id、名称、描述、稀有度），用于把徽章名解析成 badge_id，不要臆造 badge_id。"
}
func (ListBadgesTool) InputSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}
func (t ListBadgesTool) Execute(ctx context.Context, _ map[string]any) (map[string]any, error) {
	tid := cpmctx.TenantID(ctx)
	if tid == 0 {
		tid = 1
	}
	badges, _, err := t.Deps.Achievements.ListMyBadges(ctx, tid, 0)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(badges))
	for _, b := range badges {
		out = append(out, map[string]any{"id": b.ID, "name": b.Name, "description": b.Description, "rarity": string(b.Rarity)})
	}
	return map[string]any{"items": out, "total": len(out)}, nil
}

func RegisterInteractive(r *Registry, deps InteractiveDeps) {
	r.MustRegister(AskUserTool{})
	r.MustRegister(OpenPointsFormTool{})
	r.MustRegister(OpenMallItemFormTool{})
	r.MustRegister(OpenScheduleFormTool{})
	r.MustRegister(OpenAwardBadgeFormTool{deps})
	r.MustRegister(ListUsersTool{deps})
	r.MustRegister(ListBadgesTool{deps})
}
