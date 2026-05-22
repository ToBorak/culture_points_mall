package service

import (
	"context"
	"testing"

	"github.com/standardsoftware/culture_points_mall/internal/modules/values/domain"
	"github.com/stretchr/testify/require"
)

type memRepo struct{ rows []domain.Dimension }

func (m *memRepo) ListByTenant(_ context.Context, tenantID int64) ([]domain.Dimension, error) {
	var out []domain.Dimension
	for _, r := range m.rows {
		if r.TenantID == tenantID {
			out = append(out, r)
		}
	}
	return out, nil
}
func (m *memRepo) GetByCode(_ context.Context, tenantID int64, code string) (*domain.Dimension, error) {
	for i := range m.rows {
		if m.rows[i].TenantID == tenantID && m.rows[i].Code == code {
			return &m.rows[i], nil
		}
	}
	return nil, nil
}
func (m *memRepo) Upsert(_ context.Context, d *domain.Dimension) error {
	for i := range m.rows {
		if m.rows[i].TenantID == d.TenantID && m.rows[i].Code == d.Code {
			m.rows[i] = *d
			return nil
		}
	}
	m.rows = append(m.rows, *d)
	return nil
}
func (m *memRepo) SetEnabled(_ context.Context, tenantID, id int64, enabled bool) error {
	for i := range m.rows {
		if m.rows[i].TenantID == tenantID && m.rows[i].ID == id {
			m.rows[i].Enabled = enabled
		}
	}
	return nil
}

func TestService_Cache(t *testing.T) {
	r := &memRepo{}
	s := New(r)
	ctx := context.Background()
	require.NoError(t, s.Upsert(ctx, &domain.Dimension{TenantID: 1, Code: "x", Name: "X", Enabled: true}))
	rows, err := s.GetDimensions(ctx, 1)
	require.NoError(t, err)
	require.Len(t, rows, 1)
}
