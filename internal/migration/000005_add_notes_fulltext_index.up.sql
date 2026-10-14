-- Migration 000005: PostgreSQL GIN Index for Lightning-Fast Full-Text Search
CREATE INDEX IF NOT EXISTS idx_notes_fulltext ON notes 
USING GIN (to_tsvector('english', title || ' ' || COALESCE(description, '') || ' ' || COALESCE(content, '')));
