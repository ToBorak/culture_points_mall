package domain

import (
	"context"
	"time"
)

type Repository interface {
	// publications
	CreatePublication(ctx context.Context, p *Publication) error
	GetPublication(ctx context.Context, tenantID, id int64) (*Publication, error)
	ListPublished(ctx context.Context, tenantID int64) ([]Publication, error)
	ListAllForAdmin(ctx context.Context, tenantID int64) ([]Publication, error)
	GetCurrentPublished(ctx context.Context, tenantID int64) (*Publication, error)
	UpdatePublicationStatus(ctx context.Context, tenantID, id int64, status PublicationStatus, publishedAt *time.Time) error

	// sections
	ReplaceSections(ctx context.Context, publicationID int64, sections []Section) error
	ListSections(ctx context.Context, publicationID int64) ([]Section, error)

	// articles
	CreateArticle(ctx context.Context, a *Article) error
	UpdateArticle(ctx context.Context, tenantID int64, a *Article) error
	ListArticlesByPublication(ctx context.Context, tenantID, publicationID int64) ([]Article, error)

	// snapshots
	UpsertSnapshot(ctx context.Context, s *Snapshot) error
	ListSnapshots(ctx context.Context, publicationID int64) ([]Snapshot, error)

	// AI 写回
	UpdatePublicationIntro(ctx context.Context, tenantID, id int64, intro string) error
	UpdateSectionAICopy(ctx context.Context, sectionID int64, aiCopy string) error

	// AI② 案例文章：只读查 stars 表（跨模块只读，不写 stars）
	ListSelectedNominations(ctx context.Context, tenantID, seasonID int64) ([]SelectedNominationRow, error)
	ExistsArticleFromNomination(ctx context.Context, tenantID, publicationID, nominationID int64) (bool, error)

	// AI④ 文化官问答：读价值观上下文
	ListValueDimensions(ctx context.Context, tenantID int64) ([]ValueRow, error)

	// 聚合（只读查源表）
	AggStarWinners(ctx context.Context, tenantID, seasonID int64) ([]StarWinnerRow, error)
	AggValues(ctx context.Context, tenantID, seasonID int64) ([]ValueRow, error)
	AggHonors(ctx context.Context, tenantID int64, start, end *time.Time, limit int) ([]HonorRow, error)
	AggLottery(ctx context.Context, tenantID int64, start, end *time.Time, limit int) ([]LotteryRow, error)
	AggActivities(ctx context.Context, tenantID int64, start, end *time.Time, limit int) ([]ActivityRow, error)
	AggLeaderboard(ctx context.Context, tenantID int64, limit int) ([]LeaderRow, error)
}
