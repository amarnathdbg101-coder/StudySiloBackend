package model

import (
	"time"
)

// Folder represents a Samsung Notes-style hierarchical folder with recursive nesting support.
type Folder struct {
	ID          string    `json:"id" db:"id"`
	UserID      string    `json:"user_id" db:"user_id"`
	CourseID    string    `json:"course_id" db:"course_id"`
	ParentID    *string   `json:"parent_id" db:"parent_id"`
	Name        string    `json:"name" db:"name"`
	FolderType  string    `json:"folder_type" db:"folder_type"`
	Description string    `json:"description" db:"description"`
	ColorHex    string    `json:"color_hex" db:"color_hex"`
	IconName    string    `json:"icon_name" db:"icon_name"`
	Position    int       `json:"position" db:"position"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`

	// Nested children for tree structures
	Children []*Folder `json:"children,omitempty"`
}
