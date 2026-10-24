package utils

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ConnectDB establishes a tuned connection pool to PostgreSQL.
// It includes first-class compatibility for Supabase PgBouncer pooler (port 6543)
// and handles automatic timeouts and connection health checks.
func ConnectDB(databaseURL string) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database config: %w", err)
	}

	log.Printf("attempting database connection: host=%s port=%d user=%s dbname=%s",
		config.ConnConfig.Host,
		config.ConnConfig.Port,
		config.ConnConfig.User,
		config.ConnConfig.Database,
	)

	// Cloud Pooler Compatibility (Neon Serverless PgBouncer / Supabase Port 6543):
	// PgBouncer in transaction pooling mode does not support session-level prepared statements.
	// Using QueryExecModeExec prevents "prepared statement already exists" / "does not exist" errors under load.
	if strings.Contains(databaseURL, "6543") || strings.Contains(databaseURL, "pooler") || strings.Contains(databaseURL, "neon.tech") {
		config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	}

	// Low-resource VPS tuning (1 CPU / 1GB RAM) with configurable overrides.
	// Defaulting to 10 max connections prevents PostgreSQL from eating hundreds of MBs of RAM,
	// while easily serving hundreds of requests per second for personal / small-team workloads.
	maxConns := 10
	minConns := 2
	if customMax := os.Getenv("DB_MAX_CONNS"); customMax != "" {
		if val, err := strconv.Atoi(customMax); err == nil && val > 0 {
			maxConns = val
		}
	}
	if customMin := os.Getenv("DB_MIN_CONNS"); customMin != "" {
		if val, err := strconv.Atoi(customMin); err == nil && val >= 0 {
			minConns = val
		}
	}
	config.MaxConns = int32(maxConns)
	config.MinConns = int32(minConns)
	config.MaxConnLifetime = 30 * time.Minute
	config.MaxConnIdleTime = 5 * time.Minute
	config.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("unable to create database connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("unable to ping database at host=%s: %w", config.ConnConfig.Host, err)
	}

	log.Println("database connection pool initialized successfully!")
	return pool, nil
}