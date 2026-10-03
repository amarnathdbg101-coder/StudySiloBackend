package main

import (
	"context"
	"goProjectStructure/internal/handler/routes"
	"goProjectStructure/internal/middleware"
	"goProjectStructure/internal/migration"
	"goProjectStructure/internal/servers"
	"goProjectStructure/internal/utils"
	"goProjectStructure/internal/worker"

	"go.uber.org/zap"
)

func main() {
	logger := middleware.New()
	defer logger.Sync()

	cfg := utils.MustLoad()

	// Connect to PostgreSQL database pool
	db, err := utils.ConnectDB(cfg.DBURL)
	if err != nil {
		logger.Warn("database connection failed (running in offline/disconnected mode)", zap.Error(err))
	} else {
		defer db.Close()

		// Run automated schema checks and database migrations
		migration.EnsureSchemaColumns(db)
		if err := migration.RunAutoMigrations(cfg.DBURL); err != nil {
			logger.Warn("auto-migration notice", zap.Error(err))
		} else {
			logger.Info("database auto-migrations verified successfully")
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