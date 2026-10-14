-- Migration 000006: Create Mind Maps Table
CREATE TABLE IF NOT EXISTS mind_maps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    concept VARCHAR(255) NOT NULL,
    diagram_type VARCHAR(50) NOT NULL DEFAULT 'mindmap',
    domain_type VARCHAR(50) NOT NULL DEFAULT 'general',
    note_id UUID REFERENCES notes(id) ON DELETE SET NULL,
    data JSONB NOT NULL,
    is_pinned BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_mind_maps_user_updated ON mind_maps(user_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_mind_maps_user_concept ON mind_maps(user_id, concept);
CREATE INDEX IF NOT EXISTS idx_mind_maps_note ON mind_maps(note_id) WHERE note_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_mind_maps_data_gin ON mind_maps USING GIN(data);
