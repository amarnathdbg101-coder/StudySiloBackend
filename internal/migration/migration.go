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
		`CREATE INDEX IF NOT EXISTS idx_study_plans_user_completed ON study_plans(user_id, is_completed, due_date ASC);`,
		`CREATE INDEX IF NOT EXISTS idx_quizzes_user_created ON quizzes(user_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_notes_user_folder ON notes(user_id, folder_id) WHERE folder_id IS NOT NULL;`,
		`CREATE INDEX IF NOT EXISTS idx_doubt_cache_hash_created ON ai_doubt_cache(query_hash, created_at DESC);`,
		// Exam Curricula - Server-side Syllabus & Smart Cognitive Prompts
		`CREATE TABLE IF NOT EXISTS exam_curricula (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			exam_code VARCHAR(50) UNIQUE NOT NULL,
			display_name VARCHAR(255) NOT NULL,
			keywords TEXT[] NOT NULL DEFAULT '{}',
			high_yield_topics TEXT[] NOT NULL DEFAULT '{}',
			common_traps TEXT[] NOT NULL DEFAULT '{}',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);`,
		`CREATE INDEX IF NOT EXISTS idx_exam_curricula_code ON exam_curricula (exam_code);`,
		`INSERT INTO exam_curricula (exam_code, display_name, keywords, high_yield_topics, common_traps)
		VALUES
		(
			'gate_cs',
			'GATE Computer Science & IT',
			ARRAY['gate', 'computer', 'tech', 'code', 'coding', 'software', 'cs', 'it'],
			ARRAY['Data Structures & Algorithms', 'Operating Systems & Concurrency', 'Computer Networks & Protocols', 'Database Management & Indexing', 'Computer Architecture & Pipelining', 'Theory of Computation & Automata'],
			ARRAY['Deadlocks vs Starvation', 'B+ Tree Order vs Pointers', 'Pipelining Data Hazards', 'TCP Congestion Window Transitions']
		),
		(
			'neet_medical',
			'NEET Medical Entrance (UG)',
			ARRAY['neet', 'medical', 'biology', 'mbbs', 'doctor', 'usmle'],
			ARRAY['Molecular Basis of Inheritance', 'Human Physiology & Circulation', 'Genetics & Mendelian Principles', 'Organic Reaction Mechanisms', 'Chemical Equilibrium & Thermodynamics', 'Cell Structure & Division'],
			ARRAY['Mendelian Linkage Exceptions', 'Cardiac Cycle Pressure Timing', 'Le Chatelier Equilibrium Shifts', 'Enzyme Inhibition Kinetics']
		),
		(
			'jee_engineering',
			'JEE Main & Advanced (Engineering)',
			ARRAY['jee', 'iit', 'bitsat', 'engineering', 'physics', 'math'],
			ARRAY['Rotational Dynamics & Inertia', 'Electromagnetic Induction & Optics', 'Definite Integrals & Calculus', 'Thermodynamics & Heat Cycles', 'Matrices & Linear Transformations', 'Chemical Kinetics & Equilibrium'],
			ARRAY['Moment of Inertia Parallel Axis Sign', 'Lenz Law Induced EMF Signs', 'Definite Integral Discontinuity Traps', 'Carnot Efficiency Limits']
		),
		(
			'cbse_school',
			'CBSE / ICSE School Boards (Class 10 & 12)',
			ARRAY['class 10', 'class 12', 'cbse', 'icse', 'board', 'school'],
			ARRAY['Electricity & Ohm''s Law', 'Light: Reflection & Refraction', 'Carbon & Its Chemical Compounds', 'Life Processes & Circulation', 'Quadratic Equations & Trigonometry', 'Acids, Bases & Salts'],
			ARRAY['Series vs Parallel Resistance Power Dissipation', 'Lens Formula Sign Convention', 'Homologous Series Functional Groups', 'pH Scale Logarithmic Shifts']
		),
		(
			'upsc_civil',
			'UPSC Civil Services Examination',
			ARRAY['upsc', 'civil', 'ias', 'ips', 'humanities', 'history', 'polity'],
			ARRAY['Indian Constitutional Framework & Rights', 'Monsoon Dynamics & Physical Geography', 'Macroeconomics, Inflation & Monetary Policy', 'Modern Indian Freedom Movement', 'International Relations & Multilateral Treaties', 'Environmental Ecology & Biodiversity'],
			ARRAY['Fundamental Rights vs DPSP Precedence', 'El Nino vs Indian Ocean Dipole Patterns', 'Repo Rate vs Reverse Repo Impact on Money Supply', 'Ramsar Wetland Classification']
		),
		(
			'general',
			'General Academic Studies',
			ARRAY['general', 'academic', 'studies', 'college', 'foundation'],
			ARRAY['Active Recall & Spaced Repetition', 'Critical Thinking & Logic Proofs', 'Scientific Method & Data Analysis', 'Core Mathematical Foundations', 'Thermodynamics Principles', 'Probability & Applied Statistics'],
			ARRAY['Correlation vs Causation', 'Bayes Theorem Prior Probability Fallacy', 'Entropy Increase in Closed Systems']
		)
		ON CONFLICT (exam_code) DO NOTHING;`,
	}

	for index, q := range queries {
		if _, err := pool.Exec(ctx, q); err != nil {
			return fmt.Errorf("ensure schema statement %d: %w", index+1, err)
		}
	}
	return nil
}
