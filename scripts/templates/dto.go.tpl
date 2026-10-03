package dto

import "time"

type Create__PASCAL__Request struct {
	Title       string `json:"title" validate:"required,min=2,max=150"`
	Description string `json:"description" validate:"max=1000"`
}

type Update__PASCAL__Request struct {
	Title       string `json:"title" validate:"omitempty,min=2,max=150"`
	Description string `json:"description" validate:"omitempty,max=1000"`
}

type __PASCAL__Response struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
