package worker

import (
	"context"
	"time"

	"goProjectStructure/internal/reuse"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// StartBackgroundMaintenanceWorker performs periodic database pool health checks and in-memory cache optimization.
func StartBackgroundMaintenanceWorker(ctx context.Context, db *pgxpool.Pool, logger *zap.Logger) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	logger.Info("background maintenance worker started")

	for {
		select {
		case <-ctx.Done():
			logger.Info("background maintenance worker shutting down cleanly")
			return
		case <-ticker.C:
			if db != nil {
				pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
				if err := db.Ping(pingCtx); err != nil {
					logger.Warn("background pool health check warning", zap.Error(err))
				} else {
					logger.Debug("background db pool ping verified")
				}
				cancel()
			}
			_ = reuse.GetCache()
		}
	}
}

// StartSampleBackgroundWorker is kept for backward compatibility with main.go worker call
func StartSampleBackgroundWorker(ctx context.Context, db *pgxpool.Pool, logger *zap.Logger) {
	StartBackgroundMaintenanceWorker(ctx, db, logger)
}
