package model

import (
	"time"
)

// Note represents student lecture notes, summaries, or attached study documents.
type Note struct {
	ID          string    `json:"id" db:"id"`
	UserID      string    `json:"user_id" db:"user_id"`
	CourseID    *string   `json:"course_id" db:"course_id"`
	FolderID    *string   `json:"folder_id" db:"folder_id"`
	ModuleName  string    `json:"module_name" db:"module_name"`
	Title       string    `json:"title" db:"title"`
	Description string    `json:"description" db:"description"`
	Content     string    `json:"content" db:"content"`
	FileURL     string    `json:"file_url" db:"file_url"`
	FileType    string    `json:"file_type" db:"file_type"`
	FileSize    int64     `json:"file_size" db:"file_size"`
	Tags        []string  `json:"tags" db:"tags"`
	IsPinned    bool      `json:"is_pinned" db:"is_pinned"`
	IsArchived  bool      `json:"is_archived" db:"is_archived"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}
