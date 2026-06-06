package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/standardsoftware/culture_points_mall/internal/modules/points/domain"
	valuesdomain "github.com/standardsoftware/culture_points_mall/internal/modules/values/domain"
	valuessvc "github.com/standardsoftware/culture_points_mall/internal/modules/values/service"
)

type Service struct {
	DB     *gorm.DB
	Repo   domain.Repository
	Values *valuessvc.Service
	Redis  *redis.Client
}

func New(db *gorm.DB, repo domain.Repository, values *valuessvc.Service, rdb *redis.Client) *Service {
	return &Service{DB: db, Repo: repo, Values: values, Redis: rdb}
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

// TryFreeze 冻结用户的积分（不真扣分，只占位）。返回 txID。
func (s *Service) TryFreeze(ctx context.Context, tenantID, userID int64, amount int, ttl time.Duration) (string, error) {
	total, err := s.Repo.GetTotalScore(ctx, tenantID, userID)
	if err != nil {
		return "", err
	}
	if total < amount {
		return "", fmt.Errorf("积分不足")
	}
	txID := fmt.Sprintf("tx-%d-%d-%d", userID, time.Now().UnixNano(), amount)
	if s.Redis != nil {
		ok, err := s.Redis.SetNX(ctx, "freeze:"+txID, amount, ttl).Result()
		if err != nil {
			return "", err
		}
		if !ok {
			return "", fmt.Errorf("redis freeze conflict")
		}
	}
	return txID, nil
}

// Confirm 真扣分（事务）
func (s *Service) Confirm(ctx context.Context, tenantID, userID int64, amount int, dimID int64, reason string) error {
	tx := &domain.Transaction{
		TenantID: tenantID, UserID: userID, DimensionID: dimID,
		Amount: -amount, Reason: reason,
	}
	return s.DB.WithContext(ctx).Transaction(func(_ *gorm.DB) error {
		if err := s.Repo.InsertTransaction(ctx, tx); err != nil {
			return err
		}
		if err := s.Repo.IncrementSnapshot(ctx, tenantID, userID, dimID, -amount); err != nil {
			return err
		}
		return nil
	})
}

// CancelByTxID 释放 Redis 冻结
func (s *Service) CancelByTxID(ctx context.Context, txID string) error {
	if s.Redis != nil {
		return s.Redis.Del(ctx, "freeze:"+txID).Err()
	}
	return nil
}

// GetEarnedTotal 累计赚取积分（正流水之和，排除一次性"新员工欢迎积分"）。
func (s *Service) GetEarnedTotal(ctx context.Context, tenantID, userID int64) (int, error) {
	var total int64
	err := s.DB.WithContext(ctx).
		Table("point_transactions").
		Where("tenant_id = ? AND user_id = ? AND amount > 0 AND reason <> ?", tenantID, userID, "新员工欢迎积分").
		Select("COALESCE(SUM(amount),0)").
		Scan(&total).Error
	return int(total), err
}

// GetSpentTotal 累计消费积分（负流水绝对值之和）。
func (s *Service) GetSpentTotal(ctx context.Context, tenantID, userID int64) (int, error) {
	var total int64
	err := s.DB.WithContext(ctx).
		Table("point_transactions").
		Where("tenant_id = ? AND user_id = ? AND amount < 0", tenantID, userID).
		Select("COALESCE(-SUM(amount),0)").
		Scan(&total).Error
	return int(total), err
}

// HasActivityParticipation 是否参加过活动（存在 activity_id 非空的正流水）。
func (s *Service) HasActivityParticipation(ctx context.Context, tenantID, userID int64) (bool, error) {
	var cnt int64
	err := s.DB.WithContext(ctx).
		Table("point_transactions").
		Where("tenant_id = ? AND user_id = ? AND amount > 0 AND activity_id IS NOT NULL", tenantID, userID).
		Count(&cnt).Error
	return cnt > 0, err
}
