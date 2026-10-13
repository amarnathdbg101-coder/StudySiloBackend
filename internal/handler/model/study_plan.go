package model

import "time"

type StudyPlan struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	CourseID    *string    `json:"course_id,omitempty"`
	Title       string     `json:"title"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	IsCompleted bool       `json:"is_completed"`
	Priority    string     `json:"priority"` // low, medium, high
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
