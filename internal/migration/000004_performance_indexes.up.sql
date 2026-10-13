-- Migration 000004: Performance Indexes for Ultra-Fast Queries
CREATE INDEX IF NOT EXISTS idx_chat_messages_user_scope_created ON chat_messages(user_id, scope_type, scope_id, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_notes_user_pinned_updated ON notes(user_id, is_pinned DESC, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_quizzes_user_created ON quizzes(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_quiz_questions_quiz_difficulty ON quiz_questions(quiz_id, difficulty);
