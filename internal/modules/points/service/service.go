package service

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/standardsoftware/culture_points_mall/internal/modules/points/domain"
	valuesdomain "github.com/standardsoftware/culture_points_mall/internal/modules/values/domain"
	valuessvc "github.com/standardsoftware/culture_points_mall/internal/modules/values/service"
)

type Service struct {
	DB     *gorm.DB
	Repo   domain.Repository
	Values *valuessvc.Service
}

func New(db *gorm.DB, repo domain.Repository, values *valuessvc.Service) *Service {
	return &Service{DB: db, Repo: repo, Values: values}
}

type AddPointsCmd struct {
	TenantID    int64
	UserID      int64
	Amount      int
	DimensionID int64
	DimCode     string
	ActivityID  *int64
	Reason      string
	OperatorID  *int64
}

func (s *Service) AddPoints(ctx context.Context, cmd AddPointsCmd) (*domain.Transaction, error) {
	if cmd.Amount == 0 {
		return nil, errors.New("amount must be non-zero")
	}
	dimID, err := s.resolveDimension(ctx, cmd.TenantID, cmd.DimensionID, cmd.DimCode)
	if err != nil {
		return nil, err
	}

	tx := &domain.Transaction{
		TenantID:    cmd.TenantID,
		UserID:      cmd.UserID,
		DimensionID: dimID,
		Amount:      cmd.Amount,
		ActivityID:  cmd.ActivityID,
		Reason:      cmd.Reason,
		OperatorID:  cmd.OperatorID,
	}

	err = s.DB.WithContext(ctx).Transaction(func(db *gorm.DB) error {
		if err := s.Repo.InsertTransaction(ctx, tx); err != nil {
			return fmt.Errorf("insert tx: %w", err)
		}
		if err := s.Repo.IncrementSnapshot(ctx, cmd.TenantID, cmd.UserID, dimID, cmd.Amount); err != nil {
			return fmt.Errorf("inc snapshot: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func (s *Service) resolveDimension(ctx context.Context, tenantID, dimID int64, code string) (int64, error) {
	if dimID > 0 {
		return dimID, nil
	}
	if code == "" {
		return 0, errors.New("dimension_id or dimension_code required")
	}
	rows, err := s.Values.GetDimensions(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	for _, d := range rows {
		if d.Code == code {
			return d.ID, nil
		}
	}
	return 0, fmt.Errorf("dimension code not found: %s", code)
}

func (s *Service) GetUserScores(ctx context.Context, tenantID, userID int64) ([]domain.DimensionScore, []valuesdomain.Dimension, int, error) {
	scores, err := s.Repo.GetSnapshotsByUser(ctx, tenantID, userID)
	if err != nil {
		return nil, nil, 0, err
	}
	dims, err := s.Values.GetDimensions(ctx, tenantID)
	if err != nil {
		return nil, nil, 0, err
	}
	total, err := s.Repo.GetTotalScore(ctx, tenantID, userID)
	if err != nil {
		return nil, nil, 0, err
	}
	return scores, dims, total, nil
}

func (s *Service) ListTransactions(ctx context.Context, tenantID, userID, cursor int64, limit int) ([]domain.Transaction, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.Repo.ListTransactions(ctx, tenantID, userID, cursor, limit)
}
