-- Migration 000007: Upgrade Cognitive Profiles Schema
-- Adds AI diagnosis, dynamic topic mastery, readiness score, and activity counters

ALTER TABLE cognitive_profiles ADD COLUMN IF NOT EXISTS ai_summary TEXT NOT NULL DEFAULT '';
ALTER TABLE cognitive_profiles ADD COLUMN IF NOT EXISTS topic_mastery JSONB NOT NULL DEFAULT '{}';
ALTER TABLE cognitive_profiles ADD COLUMN IF NOT EXISTS readiness_score INT NOT NULL DEFAULT 50;
ALTER TABLE cognitive_profiles ADD COLUMN IF NOT EXISTS recommended_actions JSONB NOT NULL DEFAULT '[]';
ALTER TABLE cognitive_profiles ADD COLUMN IF NOT EXISTS total_doubts_asked INT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_cognitive_profiles_readiness ON cognitive_profiles (user_id, readiness_score);
