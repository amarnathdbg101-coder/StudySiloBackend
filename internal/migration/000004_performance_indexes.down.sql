-- Down Migration 000004: Drop Performance Indexes
DROP INDEX IF EXISTS idx_chat_messages_user_scope_created;
DROP INDEX IF EXISTS idx_notes_user_pinned_updated;
DROP INDEX IF EXISTS idx_quizzes_user_created;
DROP INDEX IF EXISTS idx_quiz_questions_quiz_difficulty;
