package model

import "time"

// ExamCurriculum represents a structured syllabus curriculum domain in PostgreSQL.
type ExamCurriculum struct {
	ID              string    `json:"id" db:"id"`
	ExamCode        string    `json:"exam_code" db:"exam_code"`
	DisplayName     string    `json:"display_name" db:"display_name"`
	Keywords        []string  `json:"keywords" db:"keywords"`
	HighYieldTopics []string  `json:"high_yield_topics" db:"high_yield_topics"`
	CommonTraps     []string  `json:"common_traps" db:"common_traps"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}
