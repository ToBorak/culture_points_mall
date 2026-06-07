package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/standardsoftware/culture_points_mall/internal/modules/publication/domain"
	"github.com/standardsoftware/culture_points_mall/internal/platform/dingtalk"
	"github.com/standardsoftware/culture_points_mall/internal/platform/llm"
)

// Service 文化刊业务层，仅依赖 domain.Repository 接口，不依赖任何具体 repo 实现。
type Service struct {
	Repo domain.Repository
	LLM  llm.Client
	Ding dingtalk.Client
}

// New 构造 Service。llmC/ding 可为 nil（nil 时 AI/推送端点返 503，主流程不受影响）。
func New(repo domain.Repository, llmC llm.Client, ding dingtalk.Client) *Service {
	return &Service{Repo: repo, LLM: llmC, Ding: ding}
}

// ErrNotDraft 刊物已发布/归档，不能再修改栏目。
var ErrNotDraft = errors.New("刊物已发布，不能再改")

// ErrLLMUnavailable AI 能力未配置（LLM 客户端为 nil）。
var ErrLLMUnavailable = errors.New("AI 能力未配置")

// ─── Task 5: 建期 / 配栏目 / 文章 ─────────────────────────────────────────────

// CreateIssueCmd 创建期刊的入参。
type CreateIssueCmd struct {
	TenantID    int64
	SeasonID    *int64
	Title       string
	PeriodCode  string
	PeriodStart *time.Time
	PeriodEnd   *time.Time
}

// CreateIssue 新建一期文化刊（draft 状态）。
func (s *Service) CreateIssue(ctx context.Context, cmd CreateIssueCmd) (*domain.Publication, error) {
	p := &domain.Publication{
		TenantID:    cmd.TenantID,
		SeasonID:    cmd.SeasonID,
		Title:       cmd.Title,
		PeriodCode:  cmd.PeriodCode,
		PeriodStart: cmd.PeriodStart,
		PeriodEnd:   cmd.PeriodEnd,
		Status:      domain.PubDraft,
	}
	if err := s.Repo.CreatePublication(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// ConfigureSections 整体替换该期刊的栏目集合（仅 draft 状态可改）。
// sections 入参：PublicationID 自动设为 pubID，ID 清零（由 DB 自增）。
func (s *Service) ConfigureSections(ctx context.Context, tenantID, pubID int64, sections []domain.Section) error {
	pub, err := s.Repo.GetPublication(ctx, tenantID, pubID)
	if err != nil {
		return err
	}
	if pub.Status != domain.PubDraft {
		return ErrNotDraft
	}
	for i := range sections {
		sections[i].PublicationID = pubID
		sections[i].ID = 0
	}
	return s.Repo.ReplaceSections(ctx, pubID, sections)
}

// UpsertArticle 新建（ID==0）或更新（ID>0）文章，默认 status=draft、source_type=manual。
func (s *Service) UpsertArticle(ctx context.Context, tenantID int64, a *domain.Article) error {
	a.TenantID = tenantID
	if a.ID == 0 {
		if a.Status == "" {
			a.Status = domain.ArticleDraft
		}
		if a.SourceType == "" {
			a.SourceType = domain.ArticleManual
		}
		return s.Repo.CreateArticle(ctx, a)
	}
	return s.Repo.UpdateArticle(ctx, tenantID, a)
}

// ─── Task 6: 聚合编排 ─────────────────────────────────────────────────────────

const aggLimit = 50

// Aggregate 遍历可见且需快照的栏目，按类型只读查源表，marshal 进 publication_snapshots。
// 幂等：UpsertSnapshot 内置 ON CONFLICT 覆盖。
func (s *Service) Aggregate(ctx context.Context, tenantID, pubID int64) error {
	pub, err := s.Repo.GetPublication(ctx, tenantID, pubID)
	if err != nil {
		return err
	}
	sections, err := s.Repo.ListSections(ctx, pubID)
	if err != nil {
		return err
	}
	for _, sec := range sections {
		if !sec.Visible || !sec.Type.SnapshotBacked() {
			continue
		}
		data, err := s.aggregateSection(ctx, tenantID, pub, sec.Type)
		if err != nil {
			return err
		}
		raw, err := json.Marshal(data)
		if err != nil {
			return err
		}
		if err := s.Repo.UpsertSnapshot(ctx, &domain.Snapshot{
			PublicationID: pubID,
			SectionID:     sec.ID,
			DataJSON:      string(raw),
		}); err != nil {
			return err
		}
	}
	return nil
}

// aggregateSection 按栏目类型分派只读聚合查询。
// star/values 用 pub.SeasonID；honors/lottery/activity 用 pub.PeriodStart/End + aggLimit；leaderboard 用 aggLimit。
func (s *Service) aggregateSection(ctx context.Context, tenantID int64, pub *domain.Publication, t domain.SectionType) (interface{}, error) {
	switch t {
	case domain.SecStar:
		if pub.SeasonID == nil {
			return []domain.StarWinnerRow{}, nil
		}
		return s.Repo.AggStarWinners(ctx, tenantID, *pub.SeasonID)

	case domain.SecValues:
		var seasonID int64
		if pub.SeasonID != nil {
			seasonID = *pub.SeasonID
		}
		return s.Repo.AggValues(ctx, tenantID, seasonID)

	case domain.SecHonors:
		return s.Repo.AggHonors(ctx, tenantID, pub.PeriodStart, pub.PeriodEnd, aggLimit)

	case domain.SecLottery:
		return s.Repo.AggLottery(ctx, tenantID, pub.PeriodStart, pub.PeriodEnd, aggLimit)

	case domain.SecActivity:
		return s.Repo.AggActivities(ctx, tenantID, pub.PeriodStart, pub.PeriodEnd, aggLimit)

	case domain.SecLeaderboard:
		return s.Repo.AggLeaderboard(ctx, tenantID, aggLimit)
	}
	return nil, nil
}

// ─── Task 7: 发布 + H5 读取组装 ───────────────────────────────────────────────

// PublishedView 是 H5 阅读页的组装结果：刊物 + 有序可见栏目（各带快照数据或文章列表）。
type PublishedView struct {
	Publication *domain.Publication `json:"publication"`
	Sections    []SectionView       `json:"sections"`
}

// SectionView 单个栏目视图：快照类栏目带 Snapshot，成稿类带 Articles。
type SectionView struct {
	Section  domain.Section   `json:"section"`
	Snapshot json.RawMessage  `json:"snapshot,omitempty"` // 快照类
	Articles []domain.Article `json:"articles,omitempty"` // 成稿类
}

// Publish 将刊物状态改为 published，记录发布时间。
func (s *Service) Publish(ctx context.Context, tenantID, pubID int64) error {
	now := time.Now()
	return s.Repo.UpdatePublicationStatus(ctx, tenantID, pubID, domain.PubPublished, &now)
}

// ListPublished 列出某租户所有已发布刊物，按发布时间倒序。
func (s *Service) ListPublished(ctx context.Context, tenantID int64) ([]domain.Publication, error) {
	return s.Repo.ListPublished(ctx, tenantID)
}

// GetCurrent 获取最新一期已发布刊物的组装视图。
func (s *Service) GetCurrent(ctx context.Context, tenantID int64) (*PublishedView, error) {
	pub, err := s.Repo.GetCurrentPublished(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return s.assemble(ctx, tenantID, pub)
}

// GetDetail 获取指定 pubID 的刊物组装视图（admin 和员工端均用）。
func (s *Service) GetDetail(ctx context.Context, tenantID, pubID int64) (*PublishedView, error) {
	pub, err := s.Repo.GetPublication(ctx, tenantID, pubID)
	if err != nil {
		return nil, err
	}
	return s.assemble(ctx, tenantID, pub)
}

// ─── Task 4 AI①: 一键编排 ─────────────────────────────────────────────────────

// Compose AI① 一键编排：基于已聚合快照生成刊首语 + 各栏目导语，写回 publications.intro_text / sections.ai_copy。
// 单栏目导语失败不阻断整体。
func (s *Service) Compose(ctx context.Context, tenantID, pubID int64) error {
	if s.LLM == nil {
		return ErrLLMUnavailable
	}
	pub, err := s.Repo.GetPublication(ctx, tenantID, pubID)
	if err != nil {
		return err
	}
	sections, err := s.Repo.ListSections(ctx, pubID)
	if err != nil {
		return err
	}
	snaps, err := s.Repo.ListSnapshots(ctx, pubID)
	if err != nil {
		return err
	}
	snapBySection := make(map[int64]string, len(snaps))
	for _, sn := range snaps {
		snapBySection[sn.SectionID] = sn.DataJSON
	}
	// 1) 刊首语：喂各栏目标题 + 快照摘要
	var ctxBuf strings.Builder
	for _, sec := range sections {
		if !sec.Visible {
			continue
		}
		fmt.Fprintf(&ctxBuf, "栏目【%s】", sec.Title)
		if raw, ok := snapBySection[sec.ID]; ok {
			snippet := raw
			if len(snippet) > 300 {
				snippet = snippet[:300]
			}
			fmt.Fprintf(&ctxBuf, "数据：%s", snippet)
		}
		ctxBuf.WriteString("\n")
	}
	intro, err := llm.MessagesText(ctx,
		s.LLM,
		`你是企业文化刊的主编，写一段温暖有力的刊首语。`,
		fmt.Sprintf("本期《%s》包含以下栏目与数据：\n%s\n请写 120-180 字刊首语，不要标题，直接正文。", pub.Title, ctxBuf.String()),
		800)
	if err != nil {
		return err
	}
	if err := s.Repo.UpdatePublicationIntro(ctx, tenantID, pubID, intro); err != nil {
		return err
	}
	// 2) 各栏目导语
	for _, sec := range sections {
		if !sec.Visible {
			continue
		}
		data := snapBySection[sec.ID]
		cp, copyErr := llm.MessagesText(ctx,
			s.LLM,
			`你是文化刊编辑，为栏目写一句话导语（30 字内），点题、有感染力。`,
			fmt.Sprintf("栏目标题：%s\n栏目数据：%s\n只输出导语正文。", sec.Title, data),
			200)
		if copyErr != nil {
			continue // 单栏目失败不阻断
		}
		_ = s.Repo.UpdateSectionAICopy(ctx, sec.ID, strings.TrimSpace(cp))
	}
	return nil
}

// ─── Task 5 AI②: 发刊生成案例文章 ───────────────────────────────────────────────

// GenerateCaseArticles AI② B：把本季已入选提名生成"践行案例"文章（幂等：按 source_id 去重）。
// 返回本次新建的文章数量。
func (s *Service) GenerateCaseArticles(ctx context.Context, tenantID, pubID int64) (int, error) {
	if s.LLM == nil {
		return 0, ErrLLMUnavailable
	}
	pub, err := s.Repo.GetPublication(ctx, tenantID, pubID)
	if err != nil {
		return 0, err
	}
	if pub.SeasonID == nil {
		return 0, nil
	}
	noms, err := s.Repo.ListSelectedNominations(ctx, tenantID, *pub.SeasonID)
	if err != nil {
		return 0, err
	}
	created := 0
	for _, n := range noms {
		exists, err := s.Repo.ExistsArticleFromNomination(ctx, tenantID, pubID, n.NominationID)
		if err != nil {
			return created, err
		}
		if exists {
			continue
		}
		body := n.CaseRefined
		if strings.TrimSpace(body) == "" {
			// 没有提炼过则现炼一段
			t, llmErr := llm.MessagesText(ctx, s.LLM,
				`你是文化刊编辑，把提名理由写成一篇 100-150 字的践行小故事，第三人称、温暖具体。`,
				fmt.Sprintf("被提名人：%s\n价值观：%s\n提名理由：%s\n只输出正文。", n.NomineeName, n.Dimension, n.CaseText), 600)
			if llmErr != nil {
				continue
			}
			body = strings.TrimSpace(t)
		}
		nomID := n.NominationID
		dimID := n.DimensionID
		title := fmt.Sprintf("%s · %s", n.NomineeName, n.Dimension)
		if err := s.Repo.CreateArticle(ctx, &domain.Article{
			TenantID: tenantID, PublicationID: &pubID,
			Title: title, ContentHTML: body,
			SourceType: domain.ArticleFromNomination, SourceID: &nomID,
			ValueDimensionID: &dimID, Status: domain.ArticleDraft,
		}); err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}

// ─── Task 6 AI④: 文化官问答 ──────────────────────────────────────────────────────

// CultureQA AI④ 文化官问答：无状态，喂公司价值观上下文回答员工提问。
func (s *Service) CultureQA(ctx context.Context, tenantID int64, question string) (string, error) {
	if s.LLM == nil {
		return "", ErrLLMUnavailable
	}
	dims, _ := s.Repo.ListValueDimensions(ctx, tenantID)
	var vb strings.Builder
	for _, d := range dims {
		fmt.Fprintf(&vb, "- %s：%s\n", d.Name, d.Description)
	}
	system := fmt.Sprintf(`你是公司的"AI 文化官"，用亲切口吻解答员工关于企业文化与价值观的问题。
公司核心价值观：
%s
回答简洁（150 字内）、贴合公司语境，不知道就坦诚说不确定。`, vb.String())
	return llm.MessagesText(ctx, s.LLM, system, question, 800)
}

// ─── Task 7: 钉钉发刊推送 ──────────────────────────────────────────────────────

// PushDingtalk 把已发布刊物的摘要推到指定群机器人（groupID=config.dingtalk.robots[].id）。
func (s *Service) PushDingtalk(ctx context.Context, tenantID, pubID int64, groupID string) error {
	pub, err := s.Repo.GetPublication(ctx, tenantID, pubID)
	if err != nil {
		return err
	}
	intro := ""
	if pub.IntroText != nil {
		intro = *pub.IntroText
	}
	detail := intro
	if detail == "" {
		detail = "本期文化刊已发布，快来 H5 查看星标榜、获奖与价值观专区！"
	}
	return s.Ding.BotBroadcast(ctx, groupID, dingtalk.Card{
		Title:  "📖 " + pub.Title + " 已发布",
		Detail: detail,
		Extra:  map[string]any{"publicationId": pub.ID, "periodCode": pub.PeriodCode},
	})
}

// assemble 组装可见栏目（按 sort_order ASC）。
// - SnapshotBacked 栏目（star/values/honors/lottery/activity/leaderboard）：从 publication_snapshots 取 json.RawMessage 塞入 Snapshot。
// - 成稿类栏目（editorial/innovation/custom 等）：从 publication_articles 按 section_id 聚合塞入 Articles。
// - 两者可同时存在（快照 + 文章），由调用方决定前端展示顺序。
func (s *Service) assemble(ctx context.Context, tenantID int64, pub *domain.Publication) (*PublishedView, error) {
	sections, err := s.Repo.ListSections(ctx, pub.ID)
	if err != nil {
		return nil, err
	}

	snaps, err := s.Repo.ListSnapshots(ctx, pub.ID)
	if err != nil {
		return nil, err
	}
	snapBySection := make(map[int64]string, len(snaps))
	for _, sn := range snaps {
		snapBySection[sn.SectionID] = sn.DataJSON
	}

	articles, err := s.Repo.ListArticlesByPublication(ctx, tenantID, pub.ID)
	if err != nil {
		return nil, err
	}
	artBySection := make(map[int64][]domain.Article)
	for _, a := range articles {
		if a.SectionID != nil {
			artBySection[*a.SectionID] = append(artBySection[*a.SectionID], a)
		}
	}

	views := make([]SectionView, 0, len(sections))
	for _, sec := range sections {
		if !sec.Visible {
			continue
		}
		sv := SectionView{Section: sec}
		if raw, ok := snapBySection[sec.ID]; ok {
			sv.Snapshot = json.RawMessage(raw)
		}
		if arts, ok := artBySection[sec.ID]; ok {
			sv.Articles = arts
		}
		views = append(views, sv)
	}

	return &PublishedView{Publication: pub, Sections: views}, nil
}
