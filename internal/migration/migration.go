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
func EnsureSchemaColumns(pool *pgxpool.Pool) error {
	if pool == nil {
		return fmt.Errorf("database pool is required")
	}

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
		`CREATE TABLE IF NOT EXISTS mind_maps (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			title VARCHAR(255) NOT NULL,
			concept VARCHAR(255) NOT NULL,
			diagram_type VARCHAR(50) NOT NULL DEFAULT 'mindmap',
			domain_type VARCHAR(50) NOT NULL DEFAULT 'general',
			note_id UUID REFERENCES notes(id) ON DELETE SET NULL,
			data JSONB NOT NULL,
			is_pinned BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);`,
		`CREATE INDEX IF NOT EXISTS idx_mind_maps_user_updated ON mind_maps(user_id, updated_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_mind_maps_user_concept ON mind_maps(user_id, concept);`,
		`CREATE INDEX IF NOT EXISTS idx_mind_maps_note ON mind_maps(note_id) WHERE note_id IS NOT NULL;`,
		`CREATE INDEX IF NOT EXISTS idx_mind_maps_data_gin ON mind_maps USING GIN(data);`,
		`ALTER TABLE cognitive_profiles ADD COLUMN IF NOT EXISTS ai_summary TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE cognitive_profiles ADD COLUMN IF NOT EXISTS topic_mastery JSONB NOT NULL DEFAULT '{}';`,
		`ALTER TABLE cognitive_profiles ADD COLUMN IF NOT EXISTS readiness_score INT NOT NULL DEFAULT 50;`,
		`ALTER TABLE cognitive_profiles ADD COLUMN IF NOT EXISTS recommended_actions JSONB NOT NULL DEFAULT '[]';`,
		`ALTER TABLE cognitive_profiles ADD COLUMN IF NOT EXISTS total_doubts_asked INT NOT NULL DEFAULT 0;`,
		`CREATE INDEX IF NOT EXISTS idx_cognitive_profiles_readiness ON cognitive_profiles (user_id, readiness_score);`,
		`CREATE TABLE IF NOT EXISTS flashcard_decks (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			title VARCHAR(255) NOT NULL,
			topic VARCHAR(255) NOT NULL,
			difficulty VARCHAR(50) NOT NULL DEFAULT 'medium',
			card_count INT NOT NULL DEFAULT 5,
			cards JSONB NOT NULL DEFAULT '[]',
			mastered_count INT NOT NULL DEFAULT 0,
			is_pinned BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);`,
		`CREATE INDEX IF NOT EXISTS idx_flashcard_decks_user_updated ON flashcard_decks(user_id, updated_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_flashcard_decks_user_topic ON flashcard_decks(user_id, topic);`,
		`CREATE INDEX IF NOT EXISTS idx_flashcard_decks_cards_gin ON flashcard_decks USING GIN(cards);`,
		// AI Exam Study Roadmaps table - AI se generate hue roadmaps ko persist karne ke liye
		`CREATE TABLE IF NOT EXISTS ai_roadmaps (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			exam_name VARCHAR(250) NOT NULL,
			overview TEXT NOT NULL DEFAULT '',
			syllabus_topics TEXT[] NOT NULL DEFAULT '{}',
			target_days INT NOT NULL DEFAULT 30,
			schedule JSONB NOT NULL DEFAULT '[]',
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);`,
		`CREATE INDEX IF NOT EXISTS idx_ai_roadmaps_user_id ON ai_roadmaps(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_ai_roadmaps_user_active ON ai_roadmaps(user_id, is_active);`,
		`CREATE INDEX IF NOT EXISTS idx_ai_roadmaps_created ON ai_roadmaps(user_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_ai_roadmaps_schedule_gin ON ai_roadmaps USING GIN(schedule);`,
		// AI Doubt Cache - 15ms instant responses for identical self-contained questions with 0 false positives
		`CREATE TABLE IF NOT EXISTS ai_doubt_cache (
			query_hash VARCHAR(64) PRIMARY KEY,
			question TEXT NOT NULL,
			mode VARCHAR(50) NOT NULL DEFAULT 'academic',
			language VARCHAR(50) NOT NULL DEFAULT 'auto',
			model VARCHAR(100) NOT NULL,
			active_provider VARCHAR(50) NOT NULL DEFAULT 'google',
			solution_markdown TEXT NOT NULL,
			spoken_transcript TEXT NOT NULL DEFAULT '',
			core_concept TEXT NOT NULL DEFAULT '',
			common_pitfalls TEXT NOT NULL DEFAULT '',
			follow_up_chips JSONB NOT NULL DEFAULT '[]'::jsonb,
			hit_count INT NOT NULL DEFAULT 1,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);`,
		`CREATE INDEX IF NOT EXISTS idx_ai_doubt_cache_hash_lookup ON ai_doubt_cache(query_hash, mode, language);`,
	}

	for index, q := range queries {
		if _, err := pool.Exec(ctx, q); err != nil {
			return fmt.Errorf("ensure schema statement %d: %w", index+1, err)
		}
	}
	return nil
}
