package migration

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RunAutoMigrations executes all pending migrations from internal/migration folder.
func RunAutoMigrations(dbURL string) error {
	m, err := migrate.New("file://internal/migration", dbURL)
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run up migrations: %w", err)
	}

	return nil
}

// EnsureSchemaColumns runs fast idempotent ALTER statements to ensure critical columns/extensions exist
func EnsureSchemaColumns(pool *pgxpool.Pool) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	queries := []string{
		"CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\";",
		"CREATE EXTENSION IF NOT EXISTS \"pgcrypto\";",
		`CREATE TABLE IF NOT EXISTS chat_sessions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			title VARCHAR(255) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);`,
		`CREATE INDEX IF NOT EXISTS idx_chat_sessions_user_updated ON chat_sessions(user_id, updated_at DESC);`,
	}

	for _, q := range queries {
		_, _ = pool.Exec(ctx, q)
	}
}