package main

import (
	"context"
	"goProjectStructure/internal/handler/routes"
	"goProjectStructure/internal/middleware"
	"goProjectStructure/internal/migration"
	"goProjectStructure/internal/servers"
	"goProjectStructure/internal/utils"
	"goProjectStructure/internal/worker"
	"strings"

	"go.uber.org/zap"
)

func main() {
	logger := middleware.New()
	defer logger.Sync()

	cfg := utils.MustLoadAPI()

	// Connect to PostgreSQL database pool
	db, err := utils.ConnectDB(cfg.DBURL)
	if err != nil {
		if strings.EqualFold(cfg.Env, "production") {
			logger.Fatal("database connection failed", zap.Error(err))
		}
		logger.Warn("database connection failed (running in offline/disconnected mode)", zap.Error(err))
	} else {
		defer db.Close()

		// Apply versioned schema changes before running compatibility checks.
		if err := migration.RunAutoMigrations(cfg.DBURL); err != nil {
			if strings.EqualFold(cfg.Env, "production") {
				logger.Fatal("database migration failed", zap.Error(err))
			}
			logger.Warn("auto-migration notice", zap.Error(err))
		} else {
			if err := migration.EnsureSchemaColumns(db); err != nil {
				if strings.EqualFold(cfg.Env, "production") {
					logger.Fatal("database schema verification failed", zap.Error(err))
				}
				logger.Warn("database schema verification warning", zap.Error(err))
			} else {
				logger.Info("database migrations and schema checks verified successfully")
			}
		}
	}

	// Setup routes and middleware
	router := routes.RouteSetup(db, logger)

	// Context for background concurrent workers
	appCtx, stopApp := context.WithCancel(context.Background())
	defer stopApp()

	if db != nil {
		go worker.StartSampleBackgroundWorker(appCtx, db, logger)
	}

	// Initialize HTTP Server
	server := servers.NewServer(cfg.Port, router)

	// Start server in background goroutine
	servers.StartServer(server, logger)

	// Listen for OS interrupt signals (SIGINT, SIGTERM) for graceful shutdown
	servers.GracefulShutdown(server, stopApp, logger)
}
