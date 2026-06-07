package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/standardsoftware/culture_points_mall/internal/platform/llm"

	pointsdomain "github.com/standardsoftware/culture_points_mall/internal/modules/points/domain"
	pointssvc "github.com/standardsoftware/culture_points_mall/internal/modules/points/service"
	valuesdomain "github.com/standardsoftware/culture_points_mall/internal/modules/values/domain"
	valuessvc "github.com/standardsoftware/culture_points_mall/internal/modules/values/service"
)

// Service 把 4 个 AI 智能洞察封装在一起。所有 LLM 调用都做 Redis 缓存避免重复消耗 token。
type Service struct {
	LLM    llm.Client
	Points *pointssvc.Service
	Values *valuessvc.Service
	DB     *gorm.DB
	Redis  *redis.Client
}

func New(llmC llm.Client, p *pointssvc.Service, v *valuessvc.Service, db *gorm.DB, r *redis.Client) *Service {
	return &Service{LLM: llmC, Points: p, Values: v, DB: db, Redis: r}
}

// ===== 1. 文化 DNA 年报 =====

type DNAReport struct {
	Title       string    `json:"title"`        // "你的 Q2 文化 DNA"
	Period      string    `json:"period"`       // "quarter" / "year"
	Highlights  []string  `json:"highlights"`   // 3-5 条数据要点
	Personality []string  `json:"personality"`  // 3 个关键词
	Story       string    `json:"story"`        // 100-150 字温暖叙事
	Advice      string    `json:"advice"`       // 下阶段建议
	TopDimCode  string    `json:"topDimCode"`   // 主导维度 code
	TopDimColor string    `json:"topDimColor"`  // 主导维度色
	Stats       DNAStats  `json:"stats"`        // 关键数字
	GeneratedAt time.Time `json:"generatedAt"`
}

type DNAStats struct {
	TotalScore       int            `json:"totalScore"`
	BadgesEarned     int            `json:"badgesEarned"`
	ActivitiesJoined int            `json:"activitiesJoined"`
	ScoresByDim      map[string]int `json:"scoresByDim"`
}

func (s *Service) DNAReport(ctx context.Context, tenantID, userID int64, period string) (*DNAReport, error) {
	if period != "quarter" && period != "year" {
		period = "quarter"
	}
	cacheKey := fmt.Sprintf("insights:dna:%d:%d:%s", tenantID, userID, period)
	if cached, ok := s.getCached(ctx, cacheKey); ok {
		var r DNAReport
		if json.Unmarshal([]byte(cached), &r) == nil {
			return &r, nil
		}
	}

	stats, err := s.collectStats(ctx, tenantID, userID, period)
	if err != nil {
		return nil, err
	}
	dims, _ := s.Values.GetDimensions(ctx, tenantID)
	topCode, topColor := topDimension(stats.ScoresByDim, dims)

	periodLabel := "本季度"
	if period == "year" {
		periodLabel = "本年度"
	}
	system := `你是一位企业文化分析师，擅长把员工的积分行为数据转化为温暖且有洞察的"文化 DNA 报告"。语气积极、具体，避免空话套话。`
	user := fmt.Sprintf(`基于以下员工 %s 数据生成"文化 DNA 报告"。

数据：
- 总积分：%d
- 各维度分数：%s
- 主导维度：%s
- 获得徽章：%d 枚
- 参与活动：%d 场

要求严格输出 JSON（不要有其它任何文字，不要包代码块）：
{
  "title": "你的%s文化 DNA",
  "highlights": ["3-5 条具体数据要点，每条 20 字内"],
  "personality": ["3 个性格关键词"],
  "story": "100-150 字温暖叙事，体现该员工的文化贡献",
  "advice": "下阶段成长建议，1 句话"
}`, periodLabel,
		stats.TotalScore,
		mapToString(stats.ScoresByDim),
		topCode,
		stats.BadgesEarned,
		stats.ActivitiesJoined,
		periodLabel)

	raw, err := s.callLLMJSON(ctx, system, user)
	if err != nil {
		return nil, err
	}

	var parsed struct {
		Title       string   `json:"title"`
		Highlights  []string `json:"highlights"`
		Personality []string `json:"personality"`
		Story       string   `json:"story"`
		Advice      string   `json:"advice"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("parse dna report: %w (raw: %s)", err, raw)
	}
	report := &DNAReport{
		Title:       parsed.Title,
		Period:      period,
		Highlights:  parsed.Highlights,
		Personality: parsed.Personality,
		Story:       parsed.Story,
		Advice:      parsed.Advice,
		TopDimCode:  topCode,
		TopDimColor: topColor,
		Stats:       *stats,
		GeneratedAt: time.Now(),
	}
	s.cache(ctx, cacheKey, mustMarshal(report), 12*time.Hour)
	return report, nil
}

// ===== 2. AI 成长教练 =====

type CoachAdvice struct {
	FocusDimCode  string    `json:"focusDimCode"`
	FocusDimName  string    `json:"focusDimName"`
	FocusDimColor string    `json:"focusDimColor"`
	Title         string    `json:"title"`        // "本周聚焦：XXX"
	Reason        string    `json:"reason"`       // 为什么聚焦这个维度
	ActionItems   []string  `json:"actionItems"`  // 3 个具体行动
	ExpectedGain  string    `json:"expectedGain"` // "30-50 分"
	GeneratedAt   time.Time `json:"generatedAt"`
}

func (s *Service) Coach(ctx context.Context, tenantID, userID int64) (*CoachAdvice, error) {
	cacheKey := fmt.Sprintf("insights:coach:%d:%d", tenantID, userID)
	if cached, ok := s.getCached(ctx, cacheKey); ok {
		var c CoachAdvice
		if json.Unmarshal([]byte(cached), &c) == nil {
			return &c, nil
		}
	}

	scores, dims, _, err := s.Points.GetUserScores(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	w := weakestDimension(scores, dims)
	if w == nil {
		return nil, errors.New("no dimensions configured")
	}

	system := `你是企业文化成长教练，给员工的下周提供具体可执行的行动建议。语气积极、具体到细节，避免空话。`
	user := fmt.Sprintf(`员工当前在「%s」（%s）维度得分较弱（%d 分）。
公司在这个维度强调：%s

请生成下周聚焦此维度的具体行动建议，严格输出 JSON：
{
  "title": "本周聚焦：%s",
  "reason": "1 句话说明为什么聚焦这个维度",
  "actionItems": ["3 个具体行动，每个 15 字内，可在 1 周内完成"],
  "expectedGain": "预期获得多少分，例如 30-50 分"
}`, w.Name, w.Code, w.TotalScore, w.Keywords, w.Name)

	raw, err := s.callLLMJSON(ctx, system, user)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Title        string   `json:"title"`
		Reason       string   `json:"reason"`
		ActionItems  []string `json:"actionItems"`
		ExpectedGain string   `json:"expectedGain"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("parse coach: %w (raw: %s)", err, raw)
	}
	c := &CoachAdvice{
		FocusDimCode:  w.Code,
		FocusDimName:  w.Name,
		FocusDimColor: w.Color,
		Title:         parsed.Title,
		Reason:        parsed.Reason,
		ActionItems:   parsed.ActionItems,
		ExpectedGain:  parsed.ExpectedGain,
		GeneratedAt:   time.Now(),
	}
	s.cache(ctx, cacheKey, mustMarshal(c), 6*time.Hour)
	return c, nil
}

// ===== 3. 每周/每日文化挑战 =====

type Challenge struct {
	ID                 string    `json:"id"`
	DimensionCode      string    `json:"dimensionCode"`
	DimensionName      string    `json:"dimensionName"`
	DimensionColor     string    `json:"dimensionColor"`
	Title              string    `json:"title"`
	Description        string    `json:"description"`
	EstimatedMinutes   int       `json:"estimatedMinutes"`
	VerificationPrompt string    `json:"verificationPrompt"` // 让用户提交什么作为证明
	Points             int       `json:"points"`
	GeneratedAt        time.Time `json:"generatedAt"`
}

func (s *Service) TodayChallenge(ctx context.Context, tenantID, userID int64) (*Challenge, error) {
	cacheKey := fmt.Sprintf("insights:challenge:%d:%d:%s", tenantID, userID, time.Now().Format("2006-01-02"))
	if cached, ok := s.getCached(ctx, cacheKey); ok {
		var c Challenge
		if json.Unmarshal([]byte(cached), &c) == nil {
			return &c, nil
		}
	}

	scores, dims, _, err := s.Points.GetUserScores(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	w := weakestDimension(scores, dims)
	if w == nil {
		return nil, errors.New("no dimensions configured")
	}

	system := `你是企业文化运营专家，每天给员工出一个 5-10 分钟可完成的"文化小挑战"。要具体、有趣、不老套。`
	user := fmt.Sprintf(`员工在「%s」维度较弱。公司在这个维度强调：%s

请生成今天的小挑战，严格输出 JSON：
{
  "title": "挑战标题（10 字内）",
  "description": "做什么 + 为什么有意义（30 字内）",
  "estimatedMinutes": 5,
  "verificationPrompt": "完成后让用户提交什么作为证明，1 句话",
  "points": 20
}`, w.Name, w.Keywords)

	raw, err := s.callLLMJSON(ctx, system, user)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Title              string `json:"title"`
		Description        string `json:"description"`
		EstimatedMinutes   int    `json:"estimatedMinutes"`
		VerificationPrompt string `json:"verificationPrompt"`
		Points             int    `json:"points"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("parse challenge: %w (raw: %s)", err, raw)
	}
	if parsed.Points <= 0 || parsed.Points > 100 {
		parsed.Points = 20
	}
	c := &Challenge{
		ID:                 fmt.Sprintf("ch-%d-%s", userID, time.Now().Format("20060102")),
		DimensionCode:      w.Code,
		DimensionName:      w.Name,
		DimensionColor:     w.Color,
		Title:              parsed.Title,
		Description:        parsed.Description,
		EstimatedMinutes:   parsed.EstimatedMinutes,
		VerificationPrompt: parsed.VerificationPrompt,
		Points:             parsed.Points,
		GeneratedAt:        time.Now(),
	}
	s.cache(ctx, cacheKey, mustMarshal(c), 24*time.Hour)
	return c, nil
}

type ChallengeSubmitResult struct {
	Pass        bool   `json:"pass"`
	Feedback    string `json:"feedback"`
	PointsAwarded int  `json:"pointsAwarded"`
	TransactionID int64 `json:"transactionId,omitempty"`
}

// SubmitChallenge LLM 校验用户提交的完成证明 → 通过则自动加分
func (s *Service) SubmitChallenge(ctx context.Context, tenantID, userID int64, proof string) (*ChallengeSubmitResult, error) {
	ch, err := s.TodayChallenge(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(proof) == "" {
		return &ChallengeSubmitResult{Pass: false, Feedback: "请提供完成证明再提交"}, nil
	}
	// 防重复：已提交过的当天挑战标记缓存
	doneKey := fmt.Sprintf("insights:challenge-done:%d:%d:%s", tenantID, userID, time.Now().Format("2006-01-02"))
	if cached, ok := s.getCached(ctx, doneKey); ok && cached == "1" {
		return &ChallengeSubmitResult{Pass: false, Feedback: "今天的挑战已经提交过了，明天再来"}, nil
	}

	system := `你是文化挑战的 AI 裁判，宽容但不放水。判断员工的完成证明是否真的体现了挑战要求。`
	user := fmt.Sprintf(`挑战：%s
挑战描述：%s
要求：%s

员工提交：%s

请严格输出 JSON：
{
  "pass": true/false,
  "feedback": "1-2 句温暖反馈，pass=true 时表扬，false 时鼓励重新提交"
}`, ch.Title, ch.Description, ch.VerificationPrompt, proof)

	raw, err := s.callLLMJSON(ctx, system, user)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Pass     bool   `json:"pass"`
		Feedback string `json:"feedback"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("parse submit: %w (raw: %s)", err, raw)
	}

	result := &ChallengeSubmitResult{Pass: parsed.Pass, Feedback: parsed.Feedback}
	if parsed.Pass {
		tx, err := s.Points.AddPoints(ctx, pointssvc.AddPointsCmd{
			TenantID: tenantID, UserID: userID,
			Amount:  ch.Points,
			DimCode: ch.DimensionCode,
			Reason:  "完成每日文化挑战：" + ch.Title,
		})
		if err != nil {
			return nil, err
		}
		result.PointsAwarded = ch.Points
		result.TransactionID = tx.ID
		s.cache(ctx, doneKey, "1", 24*time.Hour)
	}
	return result, nil
}

// ===== 4. 个人排行动态解读 =====

type LeaderboardInsight struct {
	Headline    string    `json:"headline"`    // "你这周 +5 名！"
	KeyDriver   string    `json:"keyDriver"`   // 主要因为啥
	NextGoal    string    `json:"nextGoal"`    // 下一个目标
	Tone        string    `json:"tone"`        // encouraging / proud / steady
	CurrentRank int       `json:"currentRank"`
	TotalScore  int       `json:"totalScore"`
	GeneratedAt time.Time `json:"generatedAt"`
}

func (s *Service) LeaderboardInsight(ctx context.Context, tenantID, userID int64) (*LeaderboardInsight, error) {
	cacheKey := fmt.Sprintf("insights:lb:%d:%d", tenantID, userID)
	if cached, ok := s.getCached(ctx, cacheKey); ok {
		var l LeaderboardInsight
		if json.Unmarshal([]byte(cached), &l) == nil {
			return &l, nil
		}
	}

	// 查当前 rank 和总分
	type row struct {
		UID   int64
		Total int
		Name  string
	}
	var all []row
	if err := s.DB.Raw(`
		SELECT u.id AS uid, u.name, COALESCE(SUM(s.total_score), 0) AS total
		FROM users u
		LEFT JOIN user_dimension_scores s ON s.user_id = u.id AND s.tenant_id = u.tenant_id
		WHERE u.tenant_id = ?
		GROUP BY u.id, u.name
		ORDER BY total DESC
	`, tenantID).Scan(&all).Error; err != nil {
		return nil, err
	}
	var myRank, myScore int
	var leader string
	for i, r := range all {
		if r.UID == userID {
			myRank = i + 1
			myScore = r.Total
		}
		if i == 0 {
			leader = r.Name
		}
	}
	if myRank == 0 {
		myRank = len(all) + 1
	}
	beats := 0
	if len(all) > 0 {
		beats = (len(all) - myRank) * 100 / len(all)
	}

	// 最近 7 天新增积分
	var recentScore int
	s.DB.Raw(`
		SELECT COALESCE(SUM(amount), 0) FROM point_transactions
		WHERE tenant_id = ? AND user_id = ? AND created_at >= ? AND amount > 0
	`, tenantID, userID, time.Now().AddDate(0, 0, -7)).Scan(&recentScore)

	// 最近 1 次加分原因
	var recentReason string
	s.DB.Raw(`
		SELECT reason FROM point_transactions
		WHERE tenant_id = ? AND user_id = ? AND amount > 0
		ORDER BY created_at DESC LIMIT 1
	`, tenantID, userID).Scan(&recentReason)

	system := `你是排行榜解说员，用 2-3 句话给员工解读 ta 当前的位置变化，语气有节奏感、积极但不浮夸。`
	user := fmt.Sprintf(`员工当前排名：第 %d / %d 名
总积分：%d
超越 %d%% 同事
最近 7 天新增积分：%d
最近一笔加分原因：%s
榜首是：%s

严格输出 JSON：
{
  "headline": "首行口播，10 字内，体现位置或趋势",
  "keyDriver": "最近表现的关键驱动，1 句话",
  "nextGoal": "下一步建议或目标，1 句话",
  "tone": "encouraging / proud / steady 之一"
}`, myRank, len(all), myScore, beats, recentScore, recentReason, leader)

	raw, err := s.callLLMJSON(ctx, system, user)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Headline  string `json:"headline"`
		KeyDriver string `json:"keyDriver"`
		NextGoal  string `json:"nextGoal"`
		Tone      string `json:"tone"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("parse lb insight: %w (raw: %s)", err, raw)
	}
	insight := &LeaderboardInsight{
		Headline:    parsed.Headline,
		KeyDriver:   parsed.KeyDriver,
		NextGoal:    parsed.NextGoal,
		Tone:        parsed.Tone,
		CurrentRank: myRank,
		TotalScore:  myScore,
		GeneratedAt: time.Now(),
	}
	s.cache(ctx, cacheKey, mustMarshal(insight), time.Hour)
	return insight, nil
}

// ===== 公共辅助 =====

func (s *Service) collectStats(ctx context.Context, tenantID, userID int64, period string) (*DNAStats, error) {
	var startAt time.Time
	if period == "year" {
		startAt = time.Now().AddDate(-1, 0, 0)
	} else {
		startAt = time.Now().AddDate(0, -3, 0)
	}

	scores, dims, total, err := s.Points.GetUserScores(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	dimByID := map[int64]valuesdomain.Dimension{}
	for _, d := range dims {
		dimByID[d.ID] = d
	}
	scoresByDim := map[string]int{}
	for _, s := range scores {
		if d, ok := dimByID[s.DimensionID]; ok {
			scoresByDim[d.Code] = s.TotalScore
		}
	}

	var badges int64
	s.DB.Raw(`SELECT COUNT(*) FROM user_badges WHERE user_id = ? AND created_at >= ?`, userID, startAt).Scan(&badges)

	var activities int64
	s.DB.Raw(`SELECT COUNT(DISTINCT activity_id) FROM point_transactions WHERE user_id = ? AND created_at >= ? AND activity_id IS NOT NULL`, userID, startAt).Scan(&activities)

	return &DNAStats{
		TotalScore:       total,
		BadgesEarned:     int(badges),
		ActivitiesJoined: int(activities),
		ScoresByDim:      scoresByDim,
	}, nil
}

func (s *Service) callLLMJSON(ctx context.Context, system, userPrompt string) (string, error) {
	resp, err := s.LLM.Messages(ctx, llm.MessagesRequest{
		System: system + "\n\n严格输出 JSON，不要包代码块，不要多余文字。",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: []llm.Block{{Type: "text", Text: userPrompt}}},
		},
		MaxTokens: 1024,
	})
	if err != nil {
		return "", err
	}
	var text string
	for _, b := range resp.Content {
		if b.Type == "text" {
			text += b.Text
		}
	}
	text = strings.TrimSpace(text)
	// 防御：如果包了 ```json ... ``` 代码块，剥掉
	if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)
	}
	// 进一步：只保留首个 { 到最后一个 } 之间的内容
	if first := strings.Index(text, "{"); first >= 0 {
		if last := strings.LastIndex(text, "}"); last > first {
			text = text[first : last+1]
		}
	}
	if text == "" {
		return "", errors.New("llm returned empty json")
	}
	return text, nil
}

func (s *Service) getCached(ctx context.Context, key string) (string, bool) {
	if s.Redis == nil {
		return "", false
	}
	v, err := s.Redis.Get(ctx, key).Result()
	if err != nil {
		return "", false
	}
	return v, true
}

func (s *Service) cache(ctx context.Context, key, val string, ttl time.Duration) {
	if s.Redis == nil {
		return
	}
	_ = s.Redis.Set(ctx, key, val, ttl).Err()
}

func mapToString(m map[string]int) string {
	parts := make([]string, 0, len(m))
	for k, v := range m {
		parts = append(parts, fmt.Sprintf("%s=%d", k, v))
	}
	return strings.Join(parts, ", ")
}

func mustMarshal(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

func topDimension(scores map[string]int, dims []valuesdomain.Dimension) (code, color string) {
	var max int
	for _, d := range dims {
		if s, ok := scores[d.Code]; ok && s > max {
			max = s
			code = d.Code
		}
	}
	if code == "" && len(dims) > 0 {
		code = dims[0].Code
	}
	// hardcoded color map（与前端 dimColor 保持一致）
	colorMap := map[string]string{
		"customer_first": "#f97316",
		"candor":    "#0ea5e9",
		"innovation":     "#ec4899",
		"ownership":      "#10b981",
	}
	color = colorMap[code]
	if color == "" {
		color = "#7c3aed"
	}
	return
}

// WeakDimInfo 弱势维度信息（聚合 score 和元数据）
type WeakDimInfo struct {
	DimensionID   int64
	Code          string
	Name          string
	Color         string
	Keywords      string
	TotalScore    int
}

func weakestDimension(scores []pointsdomain.DimensionScore, dims []valuesdomain.Dimension) *WeakDimInfo {
	if len(dims) == 0 {
		return nil
	}
	scoreByDim := map[int64]int{}
	for _, s := range scores {
		scoreByDim[s.DimensionID] = s.TotalScore
	}
	colorMap := map[string]string{
		"customer_first": "#f97316",
		"candor":    "#0ea5e9",
		"innovation":     "#ec4899",
		"ownership":      "#10b981",
	}
	// 找最弱（含 0 分）
	var weakest *WeakDimInfo
	for _, d := range dims {
		sc := scoreByDim[d.ID]
		color := colorMap[d.Code]
		if color == "" {
			color = "#7c3aed"
		}
		info := &WeakDimInfo{
			DimensionID: d.ID, Code: d.Code, Name: d.Name, Color: color,
			Keywords: d.Keywords, TotalScore: sc,
		}
		if weakest == nil || info.TotalScore < weakest.TotalScore {
			weakest = info
		}
	}
	return weakest
}
