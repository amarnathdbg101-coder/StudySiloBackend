package utils

import (
	"context"
	"fmt"
	"log"
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

	// Supabase Pooler (Port 6543 / PgBouncer) Compatibility:
	// PgBouncer in transaction mode does not support session-level prepared statements.
	// Using QueryExecModeExec prevents "prepared statement already exists" errors under load.
	if strings.Contains(databaseURL, "6543") || strings.Contains(databaseURL, "pooler") {
		config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	}

	// Production pool tuning for low latency & high concurrency
	config.MaxConns = 50
	config.MinConns = 10
	config.MaxConnLifetime = 1 * time.Hour
	config.MaxConnIdleTime = 15 * time.Minute
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