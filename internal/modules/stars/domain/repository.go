package domain

import (
	"context"
	"time"
)

type Repository interface {
	CreateSeason(ctx context.Context, s *Season) error
	GetSeason(ctx context.Context, tenantID, id int64) (*Season, error)
	GetCurrentSeason(ctx context.Context, tenantID int64) (*Season, error)
	UpdateSeasonStatus(ctx context.Context, tenantID, id int64, status SeasonStatus) error
	ListSeasons(ctx context.Context, tenantID int64) ([]Season, error)

	CreateNomination(ctx context.Context, n *Nomination) error
	GetNomination(ctx context.Context, id int64) (*Nomination, error)
	ListNominationsBySeason(ctx context.Context, seasonID int64) ([]Nomination, error)
	ListNominationsByNominator(ctx context.Context, tenantID, userID, seasonID int64) ([]Nomination, error)
	ListNominationsByNominee(ctx context.Context, tenantID, userID, seasonID int64) ([]Nomination, error)
	CountNominationsByNominatorSince(ctx context.Context, tenantID, nominatorID int64, since time.Time) (int64, error)
	CountNominationsByNomineeSince(ctx context.Context, tenantID, nomineeID int64, since time.Time) (int64, error)
	UpdateNominationScore(ctx context.Context, id int64, score float64) error
	UpdateNominationStatus(ctx context.Context, id int64, status NominationStatus) error

	// CreateWinnerIfAbsent 命中 uk_season_user_dim 时不报错，返回是否新建。
	CreateWinnerIfAbsent(ctx context.Context, w *Winner) (created bool, err error)
	ListWinnersBySeason(ctx context.Context, seasonID int64) ([]Winner, error)

	// UserExists 校验被提名人真实存在。
	UserExists(ctx context.Context, tenantID, userID int64) (bool, error)
}
