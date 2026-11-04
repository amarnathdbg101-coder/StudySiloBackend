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

	// Supabase Pooler (Port 6543 / PgBouncer) Compatibility:
	// PgBouncer in transaction mode does not support session-level prepared statements.
	// Using QueryExecModeExec prevents "prepared statement already exists" errors under load.
	if strings.Contains(databaseURL, "6543") || strings.Contains(databaseURL, "pooler") {
		config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	}

	// Production pool tuning:
	// Safe default maxConns (25) protects against Supabase/PostgreSQL connection exhaustion.
	// Allows dynamic scaling via DATABASE_MAX_CONNS and DATABASE_MIN_CONNS env.
	maxConns := int32(25)
	if envMax := os.Getenv("DATABASE_MAX_CONNS"); envMax != "" {
		if val, err := strconv.Atoi(envMax); err == nil && val > 0 {
			maxConns = int32(val)
		}
	}

	minConns := int32(5)
	if envMin := os.Getenv("DATABASE_MIN_CONNS"); envMin != "" {
		if val, err := strconv.Atoi(envMin); err == nil && val >= 0 {
			minConns = int32(val)
		}
	}

	config.MaxConns = maxConns
	config.MinConns = minConns
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

	log.Printf("database connection pool initialized successfully! (max_conns=%d, min_conns=%d)", maxConns, minConns)
	return pool, nil
}