package model

import (
	"encoding/json"
	"time"
)

// MindMap represents a saved visual mind map or concept graph entity stored in PostgreSQL.
type MindMap struct {
	ID          string          `json:"id" db:"id"`
	UserID      string          `json:"user_id" db:"user_id"`
	Title       string          `json:"title" db:"title"`
	Concept     string          `json:"concept" db:"concept"`
	DiagramType string          `json:"diagram_type" db:"diagram_type"`
	DomainType  string          `json:"domain_type" db:"domain_type"`
	NoteID      *string         `json:"note_id,omitempty" db:"note_id"`
	Data        json.RawMessage `json:"data" db:"data"`
	IsPinned    bool            `json:"is_pinned" db:"is_pinned"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
}
