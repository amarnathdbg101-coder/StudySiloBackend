package model

import (
	"time"
)

// Quiz represents an adaptive study quiz or formal exam.
type Quiz struct {
	ID             string          `json:"id" db:"id"`
	UserID         string          `json:"user_id" db:"user_id"`
	CourseID       *string         `json:"course_id" db:"course_id"`
	NoteID         *string         `json:"note_id" db:"note_id"`
	Title          string          `json:"title" db:"title"`
	Topic          string          `json:"topic" db:"topic"`
	Difficulty     string          `json:"difficulty" db:"difficulty"`
	TestType       string          `json:"test_type" db:"test_type"` // "mcq" | "subjective" | "mixed"
	TotalQuestions int             `json:"total_questions" db:"total_questions"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`
	Questions      []*QuizQuestion `json:"questions"`
}

// QuizQuestion represents an individual question (MCQ or Subjective) in a quiz/exam.
type QuizQuestion struct {
	ID                 string    `json:"id" db:"id"`
	QuizID             string    `json:"quiz_id" db:"quiz_id"`
	QuestionType       string    `json:"question_type" db:"question_type"` // "mcq" | "subjective"
	QuestionText       string    `json:"question_text" db:"question_text"`
	Options            []string  `json:"options" db:"options"` // 4 choices for MCQ, empty for subjective
	CorrectOptionIndex int       `json:"correct_option_index" db:"correct_option_index"` // 0-3 for MCQ, -1 for subjective
	Explanation        string    `json:"explanation" db:"explanation"`
	TrapWarning        string    `json:"trap_warning" db:"trap_warning"`
	ModelAnswer        string    `json:"model_answer,omitempty" db:"model_answer"` // Ideal full-marks answer for subjective
	Rubric             []string  `json:"rubric,omitempty" db:"rubric"`             // Marking breakdown criteria for subjective
	MaxMarks           int       `json:"max_marks" db:"max_marks"`                 // Marks weight (e.g. 1 for MCQ, 5 for subjective)
	Difficulty         string    `json:"difficulty" db:"difficulty"`
	Topic              string    `json:"topic" db:"topic"`
	CreatedAt          time.Time `json:"created_at" db:"created_at"`
}

// QuizAttempt represents a completed quiz submission by a student.
type QuizAttempt struct {
	ID                  string       `json:"id" db:"id"`
	QuizID              string       `json:"quiz_id" db:"quiz_id"`
	UserID              string       `json:"user_id" db:"user_id"`
	Score               int          `json:"score" db:"score"`
	TotalScore          int          `json:"total_score" db:"total_score"`
	Percentage          float64      `json:"percentage" db:"percentage"`
	Answers             []UserAnswer `json:"answers" db:"answers"`
	WeakAreasIdentified []string     `json:"weak_areas_identified" db:"weak_areas_identified"`
	TimeTakenSeconds    int          `json:"time_taken_seconds" db:"time_taken_seconds"`
	CreatedAt           time.Time    `json:"created_at" db:"created_at"`
}

// UserAnswer captures both MCQ selections and written subjective responses with AI grading.
type UserAnswer struct {
	QuestionID        string   `json:"question_id"`
	QuestionType      string   `json:"question_type,omitempty"` // "mcq" | "subjective"
	SelectedOption    int      `json:"selected_option"`         // For MCQ: 0-3 (or -1 if skipped)
	AnswerText        string   `json:"answer_text,omitempty"`   // For Subjective: written answer
	Score             int      `json:"score,omitempty"`         // Marks awarded
	MaxMarks          int      `json:"max_marks,omitempty"`     // Total marks for question
	IsCorrect         bool     `json:"is_correct"`              // True if correct or passing grade
	Feedback          string   `json:"feedback,omitempty"`      // AI grading feedback
	KeyPointsCovered  []string `json:"key_points_covered,omitempty"`
	KeyPointsMissed   []string `json:"key_points_missed,omitempty"`
}
