package model

import (
	"time"
)

// Quiz represents an adaptive study quiz.
type Quiz struct {
	ID             string          `json:"id" db:"id"`
	UserID         string          `json:"user_id" db:"user_id"`
	CourseID       *string         `json:"course_id" db:"course_id"`
	NoteID         *string         `json:"note_id" db:"note_id"`
	Title          string          `json:"title" db:"title"`
	Topic          string          `json:"topic" db:"topic"`
	Difficulty     string          `json:"difficulty" db:"difficulty"`
	TotalQuestions int             `json:"total_questions" db:"total_questions"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`
	Questions      []*QuizQuestion `json:"questions"`
}

// QuizQuestion represents an individual question in a quiz.
type QuizQuestion struct {
	ID                 string    `json:"id" db:"id"`
	QuizID             string    `json:"quiz_id" db:"quiz_id"`
	QuestionText       string    `json:"question_text" db:"question_text"`
	Options            []string  `json:"options" db:"options"`
	CorrectOptionIndex int       `json:"correct_option_index" db:"correct_option_index"`
	Explanation        string    `json:"explanation" db:"explanation"`
	TrapWarning        string    `json:"trap_warning" db:"trap_warning"`
	Difficulty         string    `json:"difficulty" db:"difficulty"`
	Topic              string    `json:"topic" db:"topic"`
	CreatedAt          time.Time `json:"created_at" db:"created_at"`
}

// QuizAttempt represents a completed quiz submission by a student.
type QuizAttempt struct {
	ID                   string         `json:"id" db:"id"`
	QuizID               string         `json:"quiz_id" db:"quiz_id"`
	UserID               string         `json:"user_id" db:"user_id"`
	Score                int            `json:"score" db:"score"`
	TotalScore           int            `json:"total_score" db:"total_score"`
	Percentage           float64        `json:"percentage" db:"percentage"`
	Answers              []UserAnswer   `json:"answers" db:"answers"`
	WeakAreasIdentified  []string       `json:"weak_areas_identified" db:"weak_areas_identified"`
	TimeTakenSeconds     int            `json:"time_taken_seconds" db:"time_taken_seconds"`
	CreatedAt            time.Time      `json:"created_at" db:"created_at"`
}

type UserAnswer struct {
	QuestionID    string `json:"question_id"`
	SelectedOption int   `json:"selected_option"`
	IsCorrect     bool   `json:"is_correct"`
}
