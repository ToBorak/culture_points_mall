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

func (o *Orchestrator) Run(ctx context.Context, history []llm.Message, userText string) (<-chan Step, error) {
	out := make(chan Step, 16)
	go func() {
		defer close(out)
		tid := cpmctx.TenantID(ctx)
		if tid == 0 {
			tid = 1
		}
		dims, _ := o.Values.GetDimensions(ctx, tid)
		systemPrompt := buildSystemPrompt(dims)

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

func buildSystemPrompt(dims []valuesdomain.Dimension) string {
	prompt := `你是文化积分商城的 HR 运营 AI 助理。可调用提供的工具完成「发布活动、加分、查询排行榜、颁发徽章、推送钉钉通知」等操作。

公司当前的价值观维度：
`
	for _, d := range dims {
		prompt += fmt.Sprintf("- %s（code=%s, 关键词: %s）\n", d.Name, d.Code, d.Keywords)
	}
	prompt += `
执行规则：
1. 任何创建活动、加分操作必须明确指定一个 dimension_code
2. 不要凭空生成 user_id，先用 get_leaderboard 等工具查询真实 ID
3. 调用工具完毕后用一段中文给 HR 汇报本次操作的结果摘要
`
	return prompt
}
