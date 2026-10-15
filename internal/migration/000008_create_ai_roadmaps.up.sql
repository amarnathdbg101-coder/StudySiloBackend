-- Migration 000008: Create AI Study Roadmaps Table
-- AI se generate hue exam roadmaps ko persist karne ke liye
CREATE TABLE IF NOT EXISTS ai_roadmaps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    exam_name VARCHAR(250) NOT NULL,
    overview TEXT NOT NULL DEFAULT '',
    syllabus_topics TEXT[] NOT NULL DEFAULT '{}',
    target_days INT NOT NULL DEFAULT 30,
    schedule JSONB NOT NULL DEFAULT '[]',  -- Array of {day_number, title, focus_area, priority, subtasks}
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Performance indexes
CREATE INDEX IF NOT EXISTS idx_ai_roadmaps_user_id ON ai_roadmaps(user_id);
CREATE INDEX IF NOT EXISTS idx_ai_roadmaps_user_active ON ai_roadmaps(user_id, is_active);
CREATE INDEX IF NOT EXISTS idx_ai_roadmaps_created ON ai_roadmaps(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_roadmaps_schedule_gin ON ai_roadmaps USING GIN(schedule);
