package model

import (
	"time"
)

// Course represents an academic course or subject created by a student.
type Course struct {
	ID          string    `json:"id" db:"id"`
	UserID      string    `json:"user_id" db:"user_id"`
	Title       string    `json:"title" db:"title"`
	Code        string    `json:"code" db:"code"`
	Semester    string    `json:"semester" db:"semester"`
	Description string    `json:"description" db:"description"`
	ColorHex    string    `json:"color_hex" db:"color_hex"`
	IconName    string    `json:"icon_name" db:"icon_name"`
	IsArchived  bool      `json:"is_archived" db:"is_archived"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}
