-- Migration 000007: Create Flashcard Decks Table
CREATE TABLE IF NOT EXISTS flashcard_decks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    topic VARCHAR(255) NOT NULL,
    difficulty VARCHAR(50) NOT NULL DEFAULT 'medium',
    card_count INT NOT NULL DEFAULT 5,
    cards JSONB NOT NULL DEFAULT '[]',
    mastered_count INT NOT NULL DEFAULT 0,
    is_pinned BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_flashcard_decks_user_updated ON flashcard_decks(user_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_flashcard_decks_user_topic ON flashcard_decks(user_id, topic);
CREATE INDEX IF NOT EXISTS idx_flashcard_decks_cards_gin ON flashcard_decks USING GIN(cards);
