-- Migration 000012: Rollback Performance Composite Indexes
DROP INDEX IF EXISTS idx_study_plans_user_completed;
DROP INDEX IF EXISTS idx_quizzes_user_created;
DROP INDEX IF EXISTS idx_notes_user_folder;
DROP INDEX IF EXISTS idx_doubt_cache_hash_created;
