package domain

import "context"

type Repository interface {
	ListByTenant(ctx context.Context, tenantID int64) ([]Dimension, error)
	GetByCode(ctx context.Context, tenantID int64, code string) (*Dimension, error)
	Upsert(ctx context.Context, d *Dimension) error
	SetEnabled(ctx context.Context, tenantID, id int64, enabled bool) error
}
