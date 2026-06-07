package handler

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/standardsoftware/culture_points_mall/internal/modules/agent/repository"
	"github.com/standardsoftware/culture_points_mall/internal/modules/agent/service"
	"github.com/standardsoftware/culture_points_mall/internal/platform/llm"
	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

type Handler struct {
	Orchestrator *service.Orchestrator
	Sessions     *repository.Repo
}

func New(o *service.Orchestrator, s *repository.Repo) *Handler {
	return &Handler{Orchestrator: o, Sessions: s}
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.POST("/admin/agent/chat", h.chat)
	rg.GET("/admin/agent/sessions", h.listSessions)
	rg.GET("/admin/agent/sessions/:id", h.sessionDetail)
	rg.POST("/admin/agent/sessions/:id/end", h.endSession)
	rg.GET("/admin/agent/suggestions", h.suggestions)
	rg.GET("/admin/agent/search", h.search)
}

// adminFeature 后台功能目录（覆盖全部一级菜单与核心能力），供「AI 智能搜索」把自然语言映射到功能并跳转。
type adminFeature struct {
	ID    string
	Label string
	Route string
	Icon  string
	Hint  string // 关键词/说明，喂给 LLM 与关键词兜底匹配
}

var adminFeatures = []adminFeature{
	{"home", "首页", "/", "✦", "后台首页 概览 仪表盘 主页 总览"},
	{"chat", "HR-Agent 智能助理", "/chat", "⚡", "AI 助理 对话 会话 聊天 加分 扣分 积分加减 批量加分 批量操作 颁发徽章 发布活动 查排行榜 记忆"},
	{"activities", "活动管理", "/activities", "◐", "活动 发布活动 活动列表 签到码 文化活动 关闭活动 报名"},
	{"values", "价值观维度", "/values", "✧", "价值观 维度 客户第一 坦诚沟通 一号位 敢于创新 维度配置 权重 标签"},
	{"mall", "商城/盲盒", "/mall", "◈", "积分商城 商品 盲盒 上架 下架 库存 兑换 礼品 新增商品 改库存 商品管理"},
	{"insights", "数据洞察", "/insights", "⌬", "统计 数据 分析 报表 积分统计 趋势 洞察 排行 谁积分最高 概览数据 看板"},
	{"dingtalk", "钉钉推送", "/dingtalk/mock-outbox", "⊕", "钉钉 推送 消息 通知 发送记录 群机器人 卡片 outbox"},
	{"schedules", "日程发布", "/schedules", "📅", "日程 钉钉日程 日历 会议 安排 创建日程 发布日程"},
}

// search 接受自然语言，AI 识别后返回后台最匹配的功能按钮（前端点击即跳转），覆盖全部功能。
func (h *Handler) search(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		c.JSON(200, gin.H{"items": []any{}})
		return
	}
	tid := cpmctx.TenantID(c.Request.Context())
	uid := cpmctx.UserID(c.Request.Context())
	out := make([]map[string]any, 0, 6)
	for _, f := range h.matchFeatures(c.Request.Context(), q) {
		out = append(out, map[string]any{"label": f.Label, "route": f.Route, "icon": f.Icon})
	}
	// 命中具体历史会话（含已结束的，方便回看）→ 点击跳 /chat 并加载该会话
	if sessions, err := h.Sessions.SearchSessions(c.Request.Context(), tid, uid, q, 3); err == nil {
		for _, s := range sessions {
			label := s.Title
			if label == "" {
				label = s.Summary
			}
			if label == "" {
				label = "未命名会话"
			}
			out = append(out, map[string]any{"label": "会话 · " + label, "route": "/chat", "icon": "💬", "sessionId": s.ID})
		}
	}
	c.JSON(200, gin.H{"items": out})
}

func (h *Handler) matchFeatures(c context.Context, q string) []adminFeature {
	if h.Orchestrator != nil && h.Orchestrator.LLM != nil {
		var sb strings.Builder
		for _, f := range adminFeatures {
			sb.WriteString(f.ID + "|" + f.Label + "|" + f.Hint + "\n")
		}
		resp, err := h.Orchestrator.LLM.Messages(c, llm.MessagesRequest{
			System: "你是后台功能搜索助手。根据用户输入的意图，从下面功能清单里挑出最匹配的 1-4 个，按相关度从高到低排序，只输出它们的 id 组成的 JSON 数组（例如 [\"mall\",\"insights\"]），不要任何多余文字。功能清单（id|名称|说明）：\n" + sb.String(),
			Messages:  []llm.Message{{Role: llm.RoleUser, Content: []llm.Block{{Type: "text", Text: q}}}},
			MaxTokens: 60,
		})
		if err == nil {
			var txt string
			for _, b := range resp.Content {
				if b.Type == "text" {
					txt += b.Text
				}
			}
			if feats := featuresByIDs(parseIDArray(txt)); len(feats) > 0 {
				return feats
			}
		}
	}
	return keywordMatch(q)
}

func parseIDArray(s string) []string {
	i := strings.IndexByte(s, '[')
	j := strings.LastIndexByte(s, ']')
	if i < 0 || j <= i {
		return nil
	}
	var ids []string
	if json.Unmarshal([]byte(s[i:j+1]), &ids) != nil {
		return nil
	}
	return ids
}

func featuresByIDs(ids []string) []adminFeature {
	idx := make(map[string]adminFeature, len(adminFeatures))
	for _, f := range adminFeatures {
		idx[f.ID] = f
	}
	out := make([]adminFeature, 0, 4)
	seen := map[string]bool{}
	for _, id := range ids {
		if f, ok := idx[id]; ok && !seen[id] {
			seen[id] = true
			out = append(out, f)
			if len(out) >= 4 {
				break
			}
		}
	}
	return out
}

// keywordMatch LLM 不可用时的子串兜底；都不中则默认指向智能助理（它能完成大部分操作）。
func keywordMatch(q string) []adminFeature {
	out := make([]adminFeature, 0, 4)
	for _, f := range adminFeatures {
		if strings.Contains(f.Label, q) || strings.Contains(f.Hint, q) {
			out = append(out, f)
		}
	}
	if len(out) == 0 {
		for _, f := range adminFeatures {
			if f.ID == "chat" {
				out = append(out, f)
			}
		}
	}
	if len(out) > 4 {
		out = out[:4]
	}
	return out
}

// 工具 → 操作类目；类目 → 给前端的快捷操作 chip（label + 点击后发送的话 + 图标）。
var toolCategory = map[string]string{
	"add_points": "points", "batch_add_points": "points",
	"create_activity": "activity", "open_activity_form": "activity",
	"create_mall_item": "mall", "update_mall_item": "mall", "delist_mall_item": "mall",
	"relist_mall_item": "mall", "open_mall_batch": "mall", "batch_update_mall": "mall", "list_mall_items": "mall",
	"award_badge": "badge", "open_award_badge_form": "badge",
	"get_leaderboard":     "leaderboard",
	"open_activity_batch": "actbatch", "batch_update_activities": "actbatch",
	"create_dingtalk_calendar": "schedule", "open_schedule_form": "schedule",
}

type suggestChip struct {
	Label string `json:"label"`
	Send  string `json:"send"`
	Icon  string `json:"icon"`
}

var categoryChip = map[string]suggestChip{
	"points":      {"批量加分", "我要批量管理积分", "⭐"},
	"activity":    {"发布活动", "发布一个活动", "📣"},
	"mall":        {"管理商品", "我要批量管理商品", "🛍️"},
	"badge":       {"颁发徽章", "给员工颁发徽章", "🏅"},
	"leaderboard": {"查看排行榜", "查看总积分排行榜", "🏆"},
	"actbatch":    {"批量管理活动", "我要批量管理活动", "🗂️"},
	"schedule":    {"安排日程", "创建一个钉钉日程", "📅"},
}

// suggestions 按该 HR 的历史高频操作返回个性化快捷按钮；无历史则给通用默认项。
func (h *Handler) suggestions(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	uid := cpmctx.UserID(c.Request.Context())
	msgs, _ := h.Sessions.OperatorAssistantMessages(c.Request.Context(), tid, uid, 400)
	counts := map[string]int{}
	for _, m := range msgs {
		var blocks []llm.Block
		_ = json.Unmarshal(m.Content, &blocks)
		for _, b := range blocks {
			if b.Type == "tool_use" && b.ToolUse != nil {
				if cat := toolCategory[b.ToolUse.Name]; cat != "" {
					counts[cat]++
				}
			}
		}
	}
	c.JSON(200, gin.H{"items": buildSuggestions(counts)})
}

func buildSuggestions(counts map[string]int) []suggestChip {
	type kv struct {
		cat string
		n   int
	}
	ranked := make([]kv, 0, len(counts))
	for cat, n := range counts {
		if n > 0 {
			ranked = append(ranked, kv{cat, n})
		}
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].n > ranked[j].n })

	out := make([]suggestChip, 0, 4)
	seen := map[string]bool{}
	add := func(cat string) {
		if chip, ok := categoryChip[cat]; ok && !seen[cat] {
			seen[cat] = true
			out = append(out, chip)
		}
	}
	for _, e := range ranked {
		add(e.cat)
		if len(out) >= 4 {
			return out
		}
	}
	// 历史不足 4 项时，用通用默认项补齐
	for _, cat := range []string{"activity", "points", "leaderboard", "mall"} {
		add(cat)
		if len(out) >= 4 {
			break
		}
	}
	return out
}

type chatReq struct {
	SessionID int64  `json:"sessionId"`
	Text      string `json:"text" binding:"required"`
}

func (h *Handler) chat(c *gin.Context) {
	var req chatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	tid := cpmctx.TenantID(c.Request.Context())
	uid := cpmctx.UserID(c.Request.Context())

	var history []llm.Message
	if req.SessionID > 0 {
		msgs, err := h.Sessions.ListMessages(c.Request.Context(), req.SessionID)
		if err == nil {
			for _, m := range msgs {
				var blocks []llm.Block
				_ = json.Unmarshal(m.Content, &blocks)
				history = append(history, llm.Message{Role: llm.Role(m.Role), Content: blocks})
			}
		}
	} else {
		s := &repository.Session{TenantID: tid, OperatorID: uid, Title: truncate(req.Text, 50)}
		if err := h.Sessions.CreateSession(c.Request.Context(), s); err == nil {
			req.SessionID = s.ID
			// 异步用 AI 把首句精炼成简洁标题（不阻塞首条回复；侧栏下次刷新即更新）
			sid, firstText := s.ID, req.Text
			go func() {
				if title := h.generateTitle(firstText); title != "" {
					_ = h.Sessions.UpdateTitle(context.Background(), sid, title)
				}
			}()
		}
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Writer.Flush()

	if req.SessionID > 0 {
		userBlocks := []llm.Block{{Type: "text", Text: req.Text}}
		raw, _ := json.Marshal(userBlocks)
		_ = h.Sessions.AppendMessage(c.Request.Context(), &repository.Message{
			SessionID: req.SessionID, Role: string(llm.RoleUser), Content: raw,
		})
	}

	_, _ = c.Writer.Write([]byte("event: session\ndata: " + jsonStr(map[string]any{"sessionId": req.SessionID}) + "\n\n"))
	c.Writer.Flush()

	memories := h.recentMemories(c.Request.Context(), tid, uid, req.SessionID)
	steps, _ := h.Orchestrator.Run(c.Request.Context(), history, req.Text, memories)
	for step := range steps {
		raw, _ := json.Marshal(step)
		_, _ = c.Writer.Write([]byte("event: step\ndata: "))
		_, _ = c.Writer.Write(raw)
		_, _ = c.Writer.Write([]byte("\n\n"))
		c.Writer.Flush()

		if req.SessionID > 0 {
			if role, blocks, ok := stepToBlocks(step); ok {
				msgRaw, _ := json.Marshal(blocks)
				_ = h.Sessions.AppendMessage(c.Request.Context(), &repository.Message{
					SessionID: req.SessionID, Role: string(role), Content: msgRaw,
				})
			}
		}
	}
}

func (h *Handler) listSessions(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	uid := cpmctx.UserID(c.Request.Context())
	limit, _ := strconv.Atoi(c.Query("limit"))
	rows, err := h.Sessions.ListSessions(c.Request.Context(), tid, uid, limit)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"items": rows})
}

// sessionDetail 重建某会话的对话气泡，供前端点开历史会话后继续。
func (h *Handler) sessionDetail(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	tid := cpmctx.TenantID(c.Request.Context())
	uid := cpmctx.UserID(c.Request.Context())
	sess, err := h.Sessions.GetSession(c.Request.Context(), id)
	if err != nil {
		c.JSON(404, gin.H{"error": "session not found"})
		return
	}
	if sess.TenantID != tid || sess.OperatorID != uid {
		c.JSON(403, gin.H{"error": "forbidden"})
		return
	}
	msgs, _ := h.Sessions.ListMessages(c.Request.Context(), id)
	c.JSON(200, gin.H{"sessionId": id, "title": sess.Title, "summary": sess.Summary, "turns": reconstructTurns(msgs)})
}

// endSession 结束会话：用 LLM 把本次核心内容提炼成一句摘要存库（侧栏预览 + 跨会话记忆）。
func (h *Handler) endSession(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	tid := cpmctx.TenantID(c.Request.Context())
	uid := cpmctx.UserID(c.Request.Context())
	sess, err := h.Sessions.GetSession(c.Request.Context(), id)
	if err != nil {
		c.JSON(404, gin.H{"error": "session not found"})
		return
	}
	if sess.TenantID != tid || sess.OperatorID != uid {
		c.JSON(403, gin.H{"error": "forbidden"})
		return
	}
	msgs, _ := h.Sessions.ListMessages(c.Request.Context(), id)
	summary := h.summarize(c.Request.Context(), msgs)
	if summary != "" {
		_ = h.Sessions.UpdateSummary(c.Request.Context(), id, summary)
	}
	_ = h.Sessions.MarkEnded(c.Request.Context(), id) // 归档：从历史列表移除（摘要仍留作记忆）
	c.JSON(200, gin.H{"sessionId": id, "summary": summary})
}

type histStep struct {
	Kind     string         `json:"kind"`
	Text     string         `json:"text,omitempty"`
	ToolName string         `json:"toolName,omitempty"`
	ToolID   string         `json:"toolId,omitempty"`
	Input    map[string]any `json:"input,omitempty"`
	Output   map[string]any `json:"output,omitempty"`
	Error    string         `json:"error,omitempty"`
}
type histTurn struct {
	UserText string     `json:"userText"`
	Steps    []histStep `json:"steps"`
	Done     bool       `json:"done"`
}

// reconstructTurns 把存库的 []llm.Block 消息序列还原成前端对话气泡（user 文本起新一轮；
// tool_result 借 tool_use 的 id 补回 toolName；去掉 _undo 以免历史误触回撤）。
func reconstructTurns(msgs []repository.Message) []histTurn {
	turns := make([]histTurn, 0)
	toolName := map[string]string{}
	var cur *histTurn
	push := func() {
		if cur != nil {
			cur.Done = true
			turns = append(turns, *cur)
		}
	}
	for _, m := range msgs {
		var blocks []llm.Block
		_ = json.Unmarshal(m.Content, &blocks)
		switch llm.Role(m.Role) {
		case llm.RoleUser:
			var txt string
			for _, b := range blocks {
				if b.Type == "text" {
					txt += b.Text
				}
			}
			push()
			cur = &histTurn{UserText: txt}
		case llm.RoleAssistant:
			if cur == nil {
				cur = &histTurn{}
			}
			for _, b := range blocks {
				if b.Type == "text" && b.Text != "" {
					cur.Steps = append(cur.Steps, histStep{Kind: "llm_text", Text: b.Text})
				} else if b.Type == "tool_use" && b.ToolUse != nil {
					toolName[b.ToolUse.ID] = b.ToolUse.Name
					cur.Steps = append(cur.Steps, histStep{Kind: "tool_use", ToolName: b.ToolUse.Name, ToolID: b.ToolUse.ID, Input: b.ToolUse.Input})
				}
			}
		case llm.RoleTool:
			if cur == nil {
				cur = &histTurn{}
			}
			for _, b := range blocks {
				if b.Type != "tool_result" || b.ToolRes == nil {
					continue
				}
				st := histStep{Kind: "tool_result", ToolID: b.ToolRes.ToolUseID, ToolName: toolName[b.ToolRes.ToolUseID]}
				if b.ToolRes.IsError {
					st.Error = b.ToolRes.Content
				} else {
					var out map[string]any
					if json.Unmarshal([]byte(b.ToolRes.Content), &out) == nil {
						delete(out, "_undo")
						st.Output = out
					}
				}
				cur.Steps = append(cur.Steps, st)
			}
		}
	}
	push()
	return turns
}

// summarize 用 LLM 把会话转录浓缩成一句记忆。
func (h *Handler) summarize(c context.Context, msgs []repository.Message) string {
	if h.Orchestrator == nil || h.Orchestrator.LLM == nil {
		return ""
	}
	var sb strings.Builder
	for _, m := range msgs {
		var blocks []llm.Block
		_ = json.Unmarshal(m.Content, &blocks)
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				role := "HR"
				if llm.Role(m.Role) == llm.RoleAssistant {
					role = "助理"
				}
				sb.WriteString(role + "：" + b.Text + "\n")
			} else if b.Type == "tool_use" && b.ToolUse != nil {
				sb.WriteString("[执行操作] " + b.ToolUse.Name + "\n")
			}
		}
	}
	transcript := strings.TrimSpace(sb.String())
	if transcript == "" {
		return ""
	}
	resp, err := h.Orchestrator.LLM.Messages(c, llm.MessagesRequest{
		System:    "你是会话归档助手。请用一句不超过 40 字的中文，概括本次会话里 HR 实际做了哪些操作与关键结论，作为日后回忆的记忆。只输出这句话本身，不要前后缀、不要解释。",
		Messages:  []llm.Message{{Role: llm.RoleUser, Content: []llm.Block{{Type: "text", Text: transcript}}}},
		MaxTokens: 200,
	})
	if err != nil {
		return ""
	}
	var out string
	for _, b := range resp.Content {
		if b.Type == "text" {
			out += b.Text
		}
	}
	return strings.TrimSpace(out)
}

// generateTitle 用 LLM 把会话首句精炼成简洁标题（≤16 字），替代"直接拿首句当标题"。
func (h *Handler) generateTitle(text string) string {
	if h.Orchestrator == nil || h.Orchestrator.LLM == nil {
		return ""
	}
	resp, err := h.Orchestrator.LLM.Messages(context.Background(), llm.MessagesRequest{
		System:    "给用户这句话起一个不超过12个汉字、概括其意图的简洁会话标题。只输出标题本身，不要任何标点、引号或解释。",
		Messages:  []llm.Message{{Role: llm.RoleUser, Content: []llm.Block{{Type: "text", Text: text}}}},
		MaxTokens: 60,
	})
	if err != nil {
		return ""
	}
	var out string
	for _, b := range resp.Content {
		if b.Type == "text" {
			out += b.Text
		}
	}
	out = strings.Trim(strings.TrimSpace(out), "《》\"'「」 　")
	if r := []rune(out); len(r) > 16 {
		out = string(r[:16])
	}
	return out
}

// recentMemories 取该 HR 最近若干条"已结束(有摘要)"会话的摘要，作为跨会话记忆注入新会话。
func (h *Handler) recentMemories(c context.Context, tenantID, operatorID, excludeID int64) []string {
	rows, err := h.Sessions.ListMemories(c, tenantID, operatorID, 20)
	if err != nil {
		return nil
	}
	mem := make([]string, 0, 5)
	for _, s := range rows {
		if s.ID == excludeID || strings.TrimSpace(s.Summary) == "" {
			continue
		}
		mem = append(mem, s.Summary)
		if len(mem) >= 5 {
			break
		}
	}
	return mem
}

// stepToBlocks 把流式 Step 转成可持久化的 llm.Block（与重载时 json.Unmarshal 成 []llm.Block 对称）。
// 修复此前"按 Step 形状存、按 Block 形状取"字段名不匹配，导致第二轮起助手发言/工具调用全部丢失的问题。
// 存储顺序即流顺序（tool_use 紧跟其 tool_result），ListMessages 按 id ASC 回放，配对保持合法。
func stepToBlocks(step service.Step) (llm.Role, []llm.Block, bool) {
	switch step.Kind {
	case service.StepLLMText:
		if step.Text == "" {
			return "", nil, false
		}
		return llm.RoleAssistant, []llm.Block{{Type: "text", Text: step.Text}}, true
	case service.StepToolUse:
		return llm.RoleAssistant, []llm.Block{{
			Type:    "tool_use",
			ToolUse: &llm.ToolUse{ID: step.ToolID, Name: step.ToolName, Input: step.Input},
		}}, true
	case service.StepToolResult:
		isErr := step.Error != ""
		content := step.Error
		if !isErr {
			raw, _ := json.Marshal(step.Output)
			content = string(raw)
		}
		return llm.RoleTool, []llm.Block{{
			Type:    "tool_result",
			ToolRes: &llm.ToolResult{ToolUseID: step.ToolID, Content: content, IsError: isErr},
		}}, true
	}
	return "", nil, false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func jsonStr(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}
