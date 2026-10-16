-- Rollback migration 000008
ALTER TABLE cognitive_profiles DROP COLUMN IF EXISTS ai_summary;
ALTER TABLE cognitive_profiles DROP COLUMN IF EXISTS topic_mastery;
ALTER TABLE cognitive_profiles DROP COLUMN IF EXISTS readiness_score;
ALTER TABLE cognitive_profiles DROP COLUMN IF EXISTS recommended_actions;
ALTER TABLE cognitive_profiles DROP COLUMN IF EXISTS total_doubts_asked;
DROP INDEX IF EXISTS idx_cognitive_profiles_readiness;
