-- Migration 000002: Create StudySilo Schema
-- Enables uuid-ossp if not already enabled
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 1. Student Cognitive Profiles (Tracks weak areas, study streaks, and exam traps)
CREATE TABLE IF NOT EXISTS cognitive_profiles (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    target_exam VARCHAR(250) NOT NULL DEFAULT 'General / University',
    academic_level VARCHAR(100) NOT NULL DEFAULT 'Undergraduate',
    weak_topics TEXT[] NOT NULL DEFAULT '{}',
    strong_topics TEXT[] NOT NULL DEFAULT '{}',
    exam_traps TEXT[] NOT NULL DEFAULT '{}',
    study_streak_days INT NOT NULL DEFAULT 0,
    total_study_minutes INT NOT NULL DEFAULT 0,
    total_quizzes_taken INT NOT NULL DEFAULT 0,
    average_quiz_score NUMERIC(5,2) NOT NULL DEFAULT 0.00,
    last_active_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 2. Academic Courses / Subjects
CREATE TABLE IF NOT EXISTS courses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(200) NOT NULL,
    code VARCHAR(50) NOT NULL DEFAULT '',
    semester VARCHAR(100) NOT NULL DEFAULT '',
    description TEXT,
    color_hex VARCHAR(20) NOT NULL DEFAULT '#4F46E5',
    icon_name VARCHAR(50) NOT NULL DEFAULT 'book',
    is_archived BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 3. Samsung Notes-Style Hierarchical Folders (Supports recursive unlimited nesting)
CREATE TABLE IF NOT EXISTS folders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    course_id UUID NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    parent_id UUID REFERENCES folders(id) ON DELETE CASCADE,
    name VARCHAR(200) NOT NULL,
    folder_type VARCHAR(50) NOT NULL DEFAULT 'module',
    description TEXT,
    color_hex VARCHAR(20) NOT NULL DEFAULT '#4F46E5',
    icon_name VARCHAR(50) NOT NULL DEFAULT 'folder',
    position INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 4. Notes & Study Materials (Markdown notes, summaries, and R2 file attachments)
CREATE TABLE IF NOT EXISTS notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    course_id UUID REFERENCES courses(id) ON DELETE SET NULL,
    folder_id UUID REFERENCES folders(id) ON DELETE SET NULL,
    module_name VARCHAR(150) NOT NULL DEFAULT '',
    title VARCHAR(250) NOT NULL,
    description TEXT,
    content TEXT NOT NULL DEFAULT '',
    file_url TEXT,
    file_type VARCHAR(50),
    file_size BIGINT DEFAULT 0,
    tags TEXT[] DEFAULT '{}',
    is_pinned BOOLEAN NOT NULL DEFAULT false,
    is_archived BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 5. Quizzes
CREATE TABLE IF NOT EXISTS quizzes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    course_id UUID REFERENCES courses(id) ON DELETE SET NULL,
    note_id UUID REFERENCES notes(id) ON DELETE SET NULL,
    title VARCHAR(250) NOT NULL,
    topic VARCHAR(200) NOT NULL,
    difficulty VARCHAR(50) NOT NULL DEFAULT 'adaptive',
    total_questions INT NOT NULL DEFAULT 5,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 6. Quiz Questions
CREATE TABLE IF NOT EXISTS quiz_questions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    quiz_id UUID NOT NULL REFERENCES quizzes(id) ON DELETE CASCADE,
    question_text TEXT NOT NULL,
    options JSONB NOT NULL DEFAULT '[]',
    correct_option_index INT NOT NULL,
    explanation TEXT NOT NULL DEFAULT '',
    trap_warning TEXT NOT NULL DEFAULT '',
    difficulty VARCHAR(50) NOT NULL DEFAULT 'medium',
    topic VARCHAR(150) NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 7. Quiz Attempts & Submissions
CREATE TABLE IF NOT EXISTS quiz_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    quiz_id UUID NOT NULL REFERENCES quizzes(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    score INT NOT NULL DEFAULT 0,
    total_score INT NOT NULL DEFAULT 0,
    percentage NUMERIC(5,2) NOT NULL DEFAULT 0.00,
    answers JSONB NOT NULL DEFAULT '[]',
    weak_areas_identified TEXT[] NOT NULL DEFAULT '{}',
    time_taken_seconds INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 8. Multi-Modal AI Tutor Conversation History
CREATE TABLE IF NOT EXISTS chat_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    scope_type VARCHAR(50) NOT NULL DEFAULT 'global',
    scope_id VARCHAR(255) NOT NULL DEFAULT '',
    role VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,
    image_url TEXT,
    audio_url TEXT,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 9. Study Planner & Spaced Repetition Reminders
CREATE TABLE IF NOT EXISTS study_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    course_id UUID REFERENCES courses(id) ON DELETE SET NULL,
    title VARCHAR(250) NOT NULL,
    due_date TIMESTAMP WITH TIME ZONE,
    is_completed BOOLEAN NOT NULL DEFAULT false,
    priority VARCHAR(20) NOT NULL DEFAULT 'medium',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Performance Indexes
CREATE INDEX IF NOT EXISTS idx_courses_user_id ON courses(user_id);
CREATE INDEX IF NOT EXISTS idx_courses_updated_at ON courses(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_folders_course_parent ON folders(course_id, parent_id);
CREATE INDEX IF NOT EXISTS idx_folders_user_id ON folders(user_id);
CREATE INDEX IF NOT EXISTS idx_notes_user_course ON notes(user_id, course_id);
CREATE INDEX IF NOT EXISTS idx_notes_folder_id ON notes(folder_id);
CREATE INDEX IF NOT EXISTS idx_notes_updated_at ON notes(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_quizzes_user_id ON quizzes(user_id);
CREATE INDEX IF NOT EXISTS idx_quiz_questions_quiz_id ON quiz_questions(quiz_id);
CREATE INDEX IF NOT EXISTS idx_quiz_attempts_user_quiz ON quiz_attempts(user_id, quiz_id);
CREATE INDEX IF NOT EXISTS idx_chat_messages_scope ON chat_messages(scope_type, scope_id);
CREATE INDEX IF NOT EXISTS idx_chat_messages_user_created ON chat_messages(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_study_plans_user_due ON study_plans(user_id, due_date);
