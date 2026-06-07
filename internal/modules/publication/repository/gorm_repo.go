package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/standardsoftware/culture_points_mall/internal/modules/publication/domain"
)

type GormRepo struct{ DB *gorm.DB }

func New(db *gorm.DB) *GormRepo { return &GormRepo{DB: db} }

func (r *GormRepo) CreatePublication(ctx context.Context, p *domain.Publication) error {
	return r.DB.WithContext(ctx).Create(p).Error
}

func (r *GormRepo) GetPublication(ctx context.Context, tenantID, id int64) (*domain.Publication, error) {
	var p domain.Publication
	err := r.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *GormRepo) ListPublished(ctx context.Context, tenantID int64) ([]domain.Publication, error) {
	var rows []domain.Publication
	err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND status = ?", tenantID, domain.PubPublished).
		Order("published_at DESC").Find(&rows).Error
	return rows, err
}

func (r *GormRepo) GetCurrentPublished(ctx context.Context, tenantID int64) (*domain.Publication, error) {
	var p domain.Publication
	err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND status = ?", tenantID, domain.PubPublished).
		Order("published_at DESC").First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *GormRepo) UpdatePublicationStatus(ctx context.Context, tenantID, id int64, status domain.PublicationStatus, publishedAt *time.Time) error {
	updates := map[string]interface{}{"status": status}
	if publishedAt != nil {
		updates["published_at"] = *publishedAt
	}
	return r.DB.WithContext(ctx).Model(&domain.Publication{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).Updates(updates).Error
}

// ReplaceSections 用事务整体替换某刊的栏目集合（先删后插），保证排序/可见性一次定稿。
func (r *GormRepo) ReplaceSections(ctx context.Context, publicationID int64, sections []domain.Section) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("publication_id = ?", publicationID).Delete(&domain.Section{}).Error; err != nil {
			return err
		}
		if len(sections) == 0 {
			return nil
		}
		return tx.Create(&sections).Error
	})
}

func (r *GormRepo) ListSections(ctx context.Context, publicationID int64) ([]domain.Section, error) {
	var rows []domain.Section
	err := r.DB.WithContext(ctx).
		Where("publication_id = ?", publicationID).Order("sort_order ASC, id ASC").Find(&rows).Error
	return rows, err
}

func (r *GormRepo) CreateArticle(ctx context.Context, a *domain.Article) error {
	return r.DB.WithContext(ctx).Create(a).Error
}

func (r *GormRepo) UpdateArticle(ctx context.Context, tenantID int64, a *domain.Article) error {
	return r.DB.WithContext(ctx).Model(&domain.Article{}).
		Where("tenant_id = ? AND id = ?", tenantID, a.ID).
		Updates(map[string]interface{}{
			"title": a.Title, "summary": a.Summary, "content_html": a.ContentHTML,
			"cover_image_url": a.CoverImageURL, "value_dimension_id": a.ValueDimensionID,
			"status": a.Status,
		}).Error
}

func (r *GormRepo) ListArticlesByPublication(ctx context.Context, tenantID, publicationID int64) ([]domain.Article, error) {
	var rows []domain.Article
	err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND publication_id = ?", tenantID, publicationID).
		Order("id ASC").Find(&rows).Error
	return rows, err
}

func (r *GormRepo) UpsertSnapshot(ctx context.Context, s *domain.Snapshot) error {
	return r.DB.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "publication_id"}, {Name: "section_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"data_json", "created_at"}),
		}).Create(s).Error
}

func (r *GormRepo) ListSnapshots(ctx context.Context, publicationID int64) ([]domain.Snapshot, error) {
	var rows []domain.Snapshot
	err := r.DB.WithContext(ctx).Where("publication_id = ?", publicationID).Find(&rows).Error
	return rows, err
}

// applyWindow 给带 created/earned/start 时间列的聚合加可选时间窗。
func applyWindow(q *gorm.DB, col string, start, end *time.Time) *gorm.DB {
	if start != nil {
		q = q.Where(col+" >= ?", *start)
	}
	if end != nil {
		q = q.Where(col+" <= ?", *end)
	}
	return q
}

func (r *GormRepo) AggStarWinners(ctx context.Context, tenantID, seasonID int64) ([]domain.StarWinnerRow, error) {
	var rows []domain.StarWinnerRow
	err := r.DB.WithContext(ctx).
		Table("star_winners w").
		Select("w.user_id, u.name, COALESCE(u.avatar_url,'') as avatar_url, d.name as dimension, COALESCE(w.citation,'') as citation").
		Joins("JOIN users u ON u.id = w.user_id").
		Joins("JOIN value_dimensions d ON d.id = w.dimension_id").
		Where("w.tenant_id = ? AND w.season_id = ?", tenantID, seasonID).
		Order("w.id ASC").Scan(&rows).Error
	return rows, err
}

func (r *GormRepo) AggValues(ctx context.Context, tenantID, seasonID int64) ([]domain.ValueRow, error) {
	var rows []domain.ValueRow
	err := r.DB.WithContext(ctx).
		Table("value_dimensions d").
		Select(`d.id as dimension_id, d.name, d.description, d.icon, d.color,
			(SELECT COUNT(*) FROM star_nominations n WHERE n.tenant_id = d.tenant_id AND n.season_id = ? AND n.dimension_id = d.id) as nomination_count`, seasonID).
		Where("d.tenant_id = ? AND d.enabled = 1", tenantID).
		Order("d.sort_order ASC, d.id ASC").Scan(&rows).Error
	return rows, err
}

func (r *GormRepo) AggHonors(ctx context.Context, tenantID int64, start, end *time.Time, limit int) ([]domain.HonorRow, error) {
	var rows []domain.HonorRow
	q := r.DB.WithContext(ctx).
		Table("user_badges ub").
		Select("ub.user_id, u.name, b.name as badge, b.rarity, COALESCE(b.icon_url,'') as icon_url, DATE_FORMAT(ub.earned_at,'%Y-%m-%d') as earned_at").
		Joins("JOIN badges b ON b.id = ub.badge_id").
		Joins("JOIN users u ON u.id = ub.user_id").
		Where("b.tenant_id = ?", tenantID)
	q = applyWindow(q, "ub.earned_at", start, end)
	err := q.Order("ub.earned_at DESC").Limit(limit).Scan(&rows).Error
	return rows, err
}

func (r *GormRepo) AggLottery(ctx context.Context, tenantID int64, start, end *time.Time, limit int) ([]domain.LotteryRow, error) {
	var rows []domain.LotteryRow
	q := r.DB.WithContext(ctx).
		Table("mall_orders o").
		Select("o.user_id, u.name, COALESCE(p.prize_name,'') as prize, DATE_FORMAT(o.created_at,'%Y-%m-%d') as won_at").
		Joins("JOIN users u ON u.id = o.user_id").
		Joins("LEFT JOIN mall_blindbox_pool p ON p.id = o.prize_id").
		Where("o.tenant_id = ? AND o.prize_id IS NOT NULL", tenantID)
	q = applyWindow(q, "o.created_at", start, end)
	err := q.Order("o.created_at DESC").Limit(limit).Scan(&rows).Error
	return rows, err
}

func (r *GormRepo) AggActivities(ctx context.Context, tenantID int64, start, end *time.Time, limit int) ([]domain.ActivityRow, error) {
	var rows []domain.ActivityRow
	q := r.DB.WithContext(ctx).
		Table("activities a").
		Select("a.id, a.title, COALESCE(DATE_FORMAT(a.start_at,'%Y-%m-%d'),'') as start_at").
		Where("a.tenant_id = ?", tenantID)
	q = applyWindow(q, "a.start_at", start, end)
	err := q.Order("a.start_at DESC").Limit(limit).Scan(&rows).Error
	return rows, err
}

func (r *GormRepo) AggLeaderboard(ctx context.Context, tenantID int64, limit int) ([]domain.LeaderRow, error) {
	var rows []domain.LeaderRow
	err := r.DB.WithContext(ctx).
		Table("user_dimension_scores s").
		Select("s.user_id, u.name, SUM(s.quarter_score) as score").
		Joins("JOIN users u ON u.id = s.user_id").
		Where("s.tenant_id = ?", tenantID).
		Group("s.user_id, u.name").
		Order("score DESC").Limit(limit).Scan(&rows).Error
	return rows, err
}
