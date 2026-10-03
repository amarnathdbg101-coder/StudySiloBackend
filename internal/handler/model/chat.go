package model

import (
	"time"
)

// ChatMessage represents a message exchanged with the AI Tutor.
type ChatMessage struct {
	ID        string    `json:"id" db:"id"`
	UserID    string    `json:"user_id" db:"user_id"`
	ScopeType string    `json:"scope_type" db:"scope_type"`
	ScopeID   string    `json:"scope_id" db:"scope_id"`
	Role      string    `json:"role" db:"role"` // "user" or "assistant"
	Content   string    `json:"content" db:"content"`
	ImageURL  string    `json:"image_url,omitempty" db:"image_url"`
	AudioURL  string    `json:"audio_url,omitempty" db:"audio_url"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}
