package model

import (
	"encoding/json"
	"time"
)

// FlashcardItem represents an individual Anki-style active recall flashcard.
type FlashcardItem struct {
	ID          string  `json:"id"`
	Front       string  `json:"front"`
	Back        string  `json:"back"`
	Difficulty  string  `json:"difficulty"` // "easy", "medium", "hard"
	Topic       string  `json:"topic"`
	IsMastered  bool    `json:"is_mastered"`
	IsReviewed  bool    `json:"is_reviewed"`
	Repetitions int     `json:"repetitions,omitempty"`
	Interval    int     `json:"interval_days,omitempty"`
	EaseFactor  float64 `json:"ease_factor,omitempty"`
}

// FlashcardDeck represents a saved active recall flashcard deck entity in PostgreSQL.
type FlashcardDeck struct {
	ID            string          `json:"id" db:"id"`
	UserID        string          `json:"user_id" db:"user_id"`
	Title         string          `json:"title" db:"title"`
	Topic         string          `json:"topic" db:"topic"`
	Difficulty    string          `json:"difficulty" db:"difficulty"`
	CardCount     int             `json:"card_count" db:"card_count"`
	Cards         json.RawMessage `json:"cards" db:"cards"`
	MasteredCount int             `json:"mastered_count" db:"mastered_count"`
	IsPinned      bool            `json:"is_pinned" db:"is_pinned"`
	CreatedAt     time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at" db:"updated_at"`
}
