-- 000013_add_subjective_to_quiz_questions.down.sql

DROP INDEX IF EXISTS idx_quiz_questions_type;

ALTER TABLE quiz_questions 
    DROP COLUMN IF EXISTS question_type,
    DROP COLUMN IF EXISTS model_answer,
    DROP COLUMN IF EXISTS rubric,
    DROP COLUMN IF EXISTS max_marks;

ALTER TABLE quizzes 
    DROP COLUMN IF EXISTS test_type;
