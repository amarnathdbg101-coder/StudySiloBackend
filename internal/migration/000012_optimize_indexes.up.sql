-- Migration 000012: Additional Performance Composite Indexes for Ultra-Fast Queries
CREATE INDEX IF NOT EXISTS idx_study_plans_user_completed ON study_plans(user_id, is_completed, due_date ASC);
CREATE INDEX IF NOT EXISTS idx_quizzes_user_created ON quizzes(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notes_user_folder ON notes(user_id, folder_id) WHERE folder_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_doubt_cache_hash_created ON ai_doubt_cache(query_hash, created_at DESC);
