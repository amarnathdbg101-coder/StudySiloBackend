-- 000013_add_subjective_to_quiz_questions.up.sql
-- Adds subjective question support, model answers, rubric grading, and test type selection.

ALTER TABLE quiz_questions 
    ADD COLUMN IF NOT EXISTS question_type VARCHAR(30) NOT NULL DEFAULT 'mcq',
    ADD COLUMN IF NOT EXISTS model_answer TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS rubric JSONB NOT NULL DEFAULT '[]',
    ADD COLUMN IF NOT EXISTS max_marks INT NOT NULL DEFAULT 1;

ALTER TABLE quizzes 
    ADD COLUMN IF NOT EXISTS test_type VARCHAR(30) NOT NULL DEFAULT 'mcq';

CREATE INDEX IF NOT EXISTS idx_quiz_questions_type ON quiz_questions (quiz_id, question_type);
