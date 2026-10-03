package model

import (
	"time"
)

// __PASCAL__ represents the database entity for __PLURAL__
type __PASCAL__ struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
