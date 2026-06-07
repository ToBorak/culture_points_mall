package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/standardsoftware/culture_points_mall/internal/modules/agent/tools"
	valuesdomain "github.com/standardsoftware/culture_points_mall/internal/modules/values/domain"
	valuessvc "github.com/standardsoftware/culture_points_mall/internal/modules/values/service"
	"github.com/standardsoftware/culture_points_mall/internal/platform/llm"
	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

type Orchestrator struct {
	LLM      llm.Client
	Tools    *tools.Registry
	Values   *valuessvc.Service
	MaxLoops int
}

func NewOrchestrator(l llm.Client, t *tools.Registry, v *valuessvc.Service) *Orchestrator {
	return &Orchestrator{LLM: l, Tools: t, Values: v, MaxLoops: 8}
}

type StepKind string

const (
	StepLLMText    StepKind = "llm_text"
	StepToolUse    StepKind = "tool_use"
	StepToolResult StepKind = "tool_result"
	StepError      StepKind = "error"
	StepDone       StepKind = "done"
)

type Step struct {
	Kind     StepKind       `json:"kind"`
	Text     string         `json:"text,omitempty"`
	ToolName string         `json:"toolName,omitempty"`
	ToolID   string         `json:"toolId,omitempty"`
	Input    map[string]any `json:"input,omitempty"`
	Output   map[string]any `json:"output,omitempty"`
	Error    string         `json:"error,omitempty"`
}

func (o *Orchestrator) Run(ctx context.Context, history []llm.Message, userText string, memories []string) (<-chan Step, error) {
	out := make(chan Step, 16)
	go func() {
		defer close(out)
		tid := cpmctx.TenantID(ctx)
		if tid == 0 {
			tid = 1
		}
		dims, _ := o.Values.GetDimensions(ctx, tid)
		systemPrompt := buildSystemPrompt(dims, memories)

		messages := append([]llm.Message{}, history...)
		messages = append(messages, llm.Message{Role: llm.RoleUser, Content: []llm.Block{{Type: "text", Text: userText}}})

		for i := 0; i < o.MaxLoops; i++ {
			resp, err := o.LLM.Messages(ctx, llm.MessagesRequest{
				System: systemPrompt, Messages: messages,
				Tools: tools.AsLLMDefs(o.Tools), MaxTokens: 4096,
			})
			if err != nil {
				out <- Step{Kind: StepError, Error: err.Error()}
				return
			}

			var toolBlocks []llm.Block
			for _, b := range resp.Content {
				switch b.Type {
				case "text":
					if b.Text != "" {
						out <- Step{Kind: StepLLMText, Text: b.Text}
					}
				case "tool_use":
					if b.ToolUse == nil {
						continue
					}
					out <- Step{Kind: StepToolUse, ToolID: b.ToolUse.ID, ToolName: b.ToolUse.Name, Input: b.ToolUse.Input}
					res := o.Tools.Call(ctx, b.ToolUse.Name, b.ToolUse.Input)
					var content string
					if res.IsError {
						content = res.Message
						out <- Step{Kind: StepToolResult, ToolID: b.ToolUse.ID, ToolName: b.ToolUse.Name, Error: res.Message}
					} else {
						raw, _ := json.Marshal(res.Output)
						content = string(raw)
						out <- Step{Kind: StepToolResult, ToolID: b.ToolUse.ID, ToolName: b.ToolUse.Name, Output: res.Output}
					}
					toolBlocks = append(toolBlocks, llm.Block{
						Type:    "tool_result",
						ToolRes: &llm.ToolResult{ToolUseID: b.ToolUse.ID, Content: content, IsError: res.IsError},
					})
				}
			}

			messages = append(messages, llm.Message{Role: llm.RoleAssistant, Content: resp.Content})

			if len(toolBlocks) == 0 || resp.StopReason == llm.StopEnd {
				out <- Step{Kind: StepDone}
				return
			}
			messages = append(messages, llm.Message{Role: llm.RoleUser, Content: toolBlocks})
		}
		out <- Step{Kind: StepError, Error: fmt.Sprintf("超过最大循环次数 %d", o.MaxLoops)}
	}()
	return out, nil
}

func buildSystemPrompt(dims []valuesdomain.Dimension, memories []string) string {
	prompt := `你是「文化官」，公司文化积分运营的 HR AI 助理，是一个能一步步把事办成的超级智能体。（自我介绍时称自己为"文化官"，不要再叫"文化积分商城"。）可调用提供的工具完成「发布活动、积分加减、查询排行榜、颁发徽章、新增商品、推送钉钉通知」等操作。

公司当前的价值观维度：
`
	for _, d := range dims {
		prompt += fmt.Sprintf("- %s（code=%s, 关键词: %s）\n", d.Name, d.Code, d.Keywords)
	}
	if len(memories) > 0 {
		prompt += "\n【与该 HR 的历史互动记忆（最近几次会话摘要，供你延续上下文，不要主动复述）】\n"
		for _, m := range memories {
			prompt += "- " + m + "\n"
		}
	}
	prompt += `
【核心原则：缺必填才收集，凑齐就立刻执行；绝不臆造、绝不重复确认、绝不纠缠可选项】

各操作只认下列「必填项」，其余字段一律可选——【绝不要】为可选项发问、卡住或反复确认：
- add_points 积分加减：user_id、amount、dimension_code（reason 可选）
- create_mall_item 新增商品：type、name、cost（stock、image_url 可选）
- award_badge 颁发徽章：user_id、badge_id
- 钉钉通知：send_dingtalk_card 需 target、title；dingtalk_bot_broadcast 需 group_id、title
- 发布/创建活动：见 A 条，一律直接弹 open_activity_form
- 创建/安排日程、创建日历（create_dingtalk_calendar）：title、start_at、end_at、user_ids —— 一律【直接】调 open_schedule_form 弹日程表单（自带时间选择器、成员多选/全体），不要用 ask_user 逐项问时间和人员。

【商品管理】改/下架/批量都已支持：修改商品用 update_mall_item（按 item_id，先用 list_mall_items 查到 id 与现状）；下架用 delist_mall_item、上架用 relist_mall_item。商品的「修改/库存」可改字段都可只传要改的那个。

【批量管理】用户说"批量/挨个处理多个 X"时，直接调对应的 open_*_batch 弹出可勾选的批量表格，不要自己逐个处理：批量加/扣分→open_points_batch；批量管理活动（批量关闭/重新发布）→open_activity_batch；批量管理商品（下架/上架/改库存）→open_mall_batch。调用后只回一句话提示用户在表格里勾选并选择操作。

【展示约定】list_mall_items 和 get_leaderboard 的结果，前端会【自动渲染成漂亮的商品列表/排行榜卡片】。所以你【绝不要】再用 markdown 表格把这些数据重列一遍——只需一句话引导（如"商品如下 👇"、"排行榜如下 👇"）；若有要补充的结论（如某人当前排名），另起一句简短说明即可。

【ask_user 的字段类型约定】凡用 ask_user 收集信息：问日期/时间一律用 type=datetime（渲染时间选择器，别让用户手打）；问"给谁/参与人员/成员"一律用 type=choice + source=users（多人再加 multi=true，会自动带"全体人员"选项），别让用户手打姓名。

A. 发布 / 创建活动：只要识别到用户想发活动，就【立即直接】调用 open_activity_form 弹出活动表单（表单已含标题、维度、起止时间、奖励积分、「是否同时创建钉钉日程」勾选、会议室、参与人员、群推送）。
   - 严禁为活动先用 ask_user 问标题/维度/时间——这些表单里都有，先问再弹会重复且丢数据。
   - 把用户已提到的标题/维度/时间/奖励积分通过 prefill 传进去预填；弹出后只回一句"请在下方表单里填写并发布"，本轮不要再调 create_activity。

B. 其他执行类操作（积分加减 / 新增商品 / 颁发徽章 / 钉钉通知）的标准流程：
   1. 先把人名、徽章名解析成 id：人名 → list_users 查 user_id；徽章名 → list_badges 查 badge_id；维度对应上面的 code。绝不凭空编造。
   2. 核对该操作的「必填项」是否齐全——把【用户这次说的 + 之前说的 + 之前回答里给的】合并起来判断。凡用户已经给过、或能从原话明确推断的值（分值、维度、原因等），一律视为已知，绝不重新发问、也绝不"再确认一下"。
   3. 必填项已齐 → 【立刻】调用对应执行工具完成操作，再用一段中文汇报结果。此时严禁再弹任何 ask_user / open_*_form，严禁追问 reason / stock / image_url 等可选项。
   4. 确实仍缺【必填项】时才收集，且只针对还缺的那几项、每项最多问一次：
      - 缺 1-3 项 → ask_user（questions 只放仍缺的必填项；能列候选就 type=choice + options）。
      - 字段多、或用户说"我自己填/给我表单" → 对应 open_*_form（积分 open_points_form / 商品 open_mall_item_form / 徽章 open_award_badge_form）。
      - 二者对同一操作【二选一】，绝不先 ask_user 再弹同款整卡。

C. 用户的任何回复——普通文字（如"客户第一，优秀"）或以「表单回填·xxx」「我的回答」开头的消息——都算作对你上一个问题的回答。把它与历史信息合并，凑齐必填就执行，绝不从头重新问。

D. 调用 ask_user 或 open_*_form 后，只回一句"请在下面作答/填写"，本轮不要再调用任何执行工具——用户作答会作为下一条消息发回来。
`
	return prompt
}
