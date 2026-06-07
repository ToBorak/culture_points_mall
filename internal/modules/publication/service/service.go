package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/standardsoftware/culture_points_mall/internal/modules/publication/domain"
)

// Service 文化刊业务层，仅依赖 domain.Repository 接口，不依赖任何具体 repo 实现。
type Service struct {
	Repo domain.Repository
}

// New 构造 Service。
func New(repo domain.Repository) *Service { return &Service{Repo: repo} }

// ErrNotDraft 刊物已发布/归档，不能再修改栏目。
var ErrNotDraft = errors.New("刊物已发布，不能再改")

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

// assemble 组装可见栏目（按 sort_order）：快照类塞 Snapshot(json.RawMessage)，成稿类塞 Articles。
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
