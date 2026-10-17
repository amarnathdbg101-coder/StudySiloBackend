-- Migration 000010: High-Performance Safe AI Doubt Cache
CREATE TABLE IF NOT EXISTS ai_doubt_cache (
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
);

CREATE INDEX IF NOT EXISTS idx_ai_doubt_cache_hash_lookup 
ON ai_doubt_cache(query_hash, mode, language);
