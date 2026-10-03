package worker

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// StartSampleBackgroundWorker demonstrates a resilient background concurrency worker with cancelable context
func StartSampleBackgroundWorker(ctx context.Context, db *pgxpool.Pool, logger *zap.Logger) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	logger.Info("sample background worker started")

	for {
		select {
		case <-ctx.Done():
			logger.Info("sample background worker shutting down cleanly")
			return
		case <-ticker.C:
			// Execute periodic maintenance, cleanup, or data synchronization
			logger.Debug("background worker periodic tick completed")
		}
	}
}