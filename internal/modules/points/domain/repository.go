package domain

import "context"

type Repository interface {
	InsertTransaction(ctx context.Context, tx *Transaction) error
	IncrementSnapshot(ctx context.Context, tenantID, userID, dimID int64, amount int) error
	GetSnapshotsByUser(ctx context.Context, tenantID, userID int64) ([]DimensionScore, error)
	ListTransactions(ctx context.Context, tenantID, userID int64, cursor int64, limit int) ([]Transaction, error)
	GetTotalScore(ctx context.Context, tenantID, userID int64) (int, error)
}
