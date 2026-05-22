package service

import (
	"context"
	"sync"
	"time"

	"github.com/standardsoftware/culture_points_mall/internal/modules/values/domain"
)

type Service struct {
	repo  domain.Repository
	mu    sync.RWMutex
	cache map[int64][]domain.Dimension
	exp   time.Time
}

func New(repo domain.Repository) *Service {
	return &Service{repo: repo, cache: make(map[int64][]domain.Dimension)}
}

func (s *Service) GetDimensions(ctx context.Context, tenantID int64) ([]domain.Dimension, error) {
	s.mu.RLock()
	if time.Now().Before(s.exp) {
		if v, ok := s.cache[tenantID]; ok {
			s.mu.RUnlock()
			return v, nil
		}
	}
	s.mu.RUnlock()

	rows, err := s.repo.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.cache[tenantID] = rows
	s.exp = time.Now().Add(5 * time.Minute)
	s.mu.Unlock()
	return rows, nil
}

func (s *Service) Upsert(ctx context.Context, d *domain.Dimension) error {
	if err := s.repo.Upsert(ctx, d); err != nil {
		return err
	}
	s.invalidate()
	return nil
}

func (s *Service) invalidate() {
	s.mu.Lock()
	s.cache = make(map[int64][]domain.Dimension)
	s.exp = time.Time{}
	s.mu.Unlock()
}
