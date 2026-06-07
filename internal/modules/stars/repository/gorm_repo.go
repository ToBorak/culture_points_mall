package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/standardsoftware/culture_points_mall/internal/modules/stars/domain"
)

type GormRepo struct{ DB *gorm.DB }

func New(db *gorm.DB) *GormRepo { return &GormRepo{DB: db} }

func (r *GormRepo) CreateSeason(ctx context.Context, s *domain.Season) error {
	return r.DB.WithContext(ctx).Create(s).Error
}

func (r *GormRepo) GetSeason(ctx context.Context, tenantID, id int64) (*domain.Season, error) {
	var s domain.Season
	err := r.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *GormRepo) GetCurrentSeason(ctx context.Context, tenantID int64) (*domain.Season, error) {
	var s domain.Season
	err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND status IN ?", tenantID, []domain.SeasonStatus{domain.SeasonNominating, domain.SeasonJudging}).
		Order("id DESC").First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *GormRepo) UpdateSeasonStatus(ctx context.Context, tenantID, id int64, status domain.SeasonStatus) error {
	return r.DB.WithContext(ctx).Model(&domain.Season{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Update("status", status).Error
}

func (r *GormRepo) ListSeasons(ctx context.Context, tenantID int64) ([]domain.Season, error) {
	var rows []domain.Season
	err := r.DB.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("id DESC").Find(&rows).Error
	return rows, err
}

func (r *GormRepo) CreateNomination(ctx context.Context, n *domain.Nomination) error {
	return r.DB.WithContext(ctx).Create(n).Error
}

func (r *GormRepo) GetNomination(ctx context.Context, id int64) (*domain.Nomination, error) {
	var n domain.Nomination
	err := r.DB.WithContext(ctx).Where("id = ?", id).First(&n).Error
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func (r *GormRepo) ListNominationsBySeason(ctx context.Context, tenantID, seasonID int64) ([]domain.Nomination, error) {
	var rows []domain.Nomination
	err := r.DB.WithContext(ctx).Where("tenant_id = ? AND season_id = ?", tenantID, seasonID).Order("id DESC").Find(&rows).Error
	return rows, err
}

func (r *GormRepo) ListNominationsByNominator(ctx context.Context, tenantID, userID, seasonID int64) ([]domain.Nomination, error) {
	var rows []domain.Nomination
	err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND nominator_id = ? AND season_id = ?", tenantID, userID, seasonID).
		Order("id DESC").Find(&rows).Error
	return rows, err
}

func (r *GormRepo) ListNominationsByNominee(ctx context.Context, tenantID, userID, seasonID int64) ([]domain.Nomination, error) {
	var rows []domain.Nomination
	err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND nominee_id = ? AND season_id = ?", tenantID, userID, seasonID).
		Order("id DESC").Find(&rows).Error
	return rows, err
}

func (r *GormRepo) CountNominationsByNominatorSince(ctx context.Context, tenantID, nominatorID int64, since time.Time) (int64, error) {
	var cnt int64
	err := r.DB.WithContext(ctx).Model(&domain.Nomination{}).
		Where("tenant_id = ? AND nominator_id = ? AND created_at >= ?", tenantID, nominatorID, since).
		Count(&cnt).Error
	return cnt, err
}

func (r *GormRepo) CountNominationsByNomineeSince(ctx context.Context, tenantID, nomineeID int64, since time.Time) (int64, error) {
	var cnt int64
	err := r.DB.WithContext(ctx).Model(&domain.Nomination{}).
		Where("tenant_id = ? AND nominee_id = ? AND created_at >= ?", tenantID, nomineeID, since).
		Count(&cnt).Error
	return cnt, err
}

func (r *GormRepo) UpdateNominationScore(ctx context.Context, tenantID, id int64, score float64) error {
	return r.DB.WithContext(ctx).Model(&domain.Nomination{}).
		Where("id = ? AND tenant_id = ?", id, tenantID).Update("score", score).Error
}

func (r *GormRepo) UpdateNominationStatus(ctx context.Context, tenantID, id int64, status domain.NominationStatus) error {
	return r.DB.WithContext(ctx).Model(&domain.Nomination{}).
		Where("id = ? AND tenant_id = ?", id, tenantID).Update("status", status).Error
}

func (r *GormRepo) CreateWinnerIfAbsent(ctx context.Context, w *domain.Winner) (bool, error) {
	res := r.DB.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "season_id"}, {Name: "user_id"}, {Name: "dimension_id"}},
			DoNothing: true,
		}).Create(w)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

func (r *GormRepo) ListWinnersBySeason(ctx context.Context, seasonID int64) ([]domain.Winner, error) {
	var rows []domain.Winner
	err := r.DB.WithContext(ctx).Where("season_id = ?", seasonID).Order("id ASC").Find(&rows).Error
	return rows, err
}

func (r *GormRepo) UpdateNominationRefined(ctx context.Context, tenantID, id int64, refined string, tags string) error {
	return r.DB.WithContext(ctx).Model(&domain.Nomination{}).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Updates(map[string]interface{}{"case_refined": refined, "ai_tags": tags}).Error
}

func (r *GormRepo) UserExists(ctx context.Context, tenantID, userID int64) (bool, error) {
	var exists int
	err := r.DB.WithContext(ctx).
		Raw("SELECT 1 FROM users WHERE id = ? AND tenant_id = ? LIMIT 1", userID, tenantID).
		Scan(&exists).Error
	return exists == 1, err
}
