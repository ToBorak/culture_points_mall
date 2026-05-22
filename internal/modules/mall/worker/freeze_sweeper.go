package worker

import (
	"context"
	"log"
	"time"

	"github.com/standardsoftware/culture_points_mall/internal/modules/mall/repository"
	pointssvc "github.com/standardsoftware/culture_points_mall/internal/modules/points/service"
)

type FreezeSweeper struct {
	Repo   *repository.GormRepo
	Points *pointssvc.Service
}

func (w *FreezeSweeper) Start(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				w.sweep(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (w *FreezeSweeper) sweep(ctx context.Context) {
	rows, err := w.Repo.ListExpiredFreeze(ctx, time.Now(), 50)
	if err != nil {
		log.Printf("freeze sweeper list: %v", err)
		return
	}
	for _, f := range rows {
		_ = w.Points.CancelByTxID(ctx, f.TxID)
		_ = w.Repo.MarkCancelled(ctx, f.TxID)
		log.Printf("freeze sweeper cancelled %s", f.TxID)
	}
}
