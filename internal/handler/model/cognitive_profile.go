package model

import (
	"time"
)

// CognitiveProfile stores student memory, weak topics, exam traps, study streaks, and AI knowledge diagnostics.
type CognitiveProfile struct {
	UserID             string         `json:"user_id" db:"user_id"`
	TargetExam         string         `json:"target_exam" db:"target_exam"`
	AcademicLevel      string         `json:"academic_level" db:"academic_level"`
	WeakTopics         []string       `json:"weak_topics" db:"weak_topics"`
	StrongTopics       []string       `json:"strong_topics" db:"strong_topics"`
	ExamTraps          []string       `json:"exam_traps" db:"exam_traps"`
	StudyStreakDays    int            `json:"study_streak_days" db:"study_streak_days"`
	TotalStudyMinutes  int            `json:"total_study_minutes" db:"total_study_minutes"`
	TotalQuizzesTaken  int            `json:"total_quizzes_taken" db:"total_quizzes_taken"`
	AverageQuizScore   float64        `json:"average_quiz_score" db:"average_quiz_score"`
	AISummary          string         `json:"ai_summary" db:"ai_summary"`
	TopicMastery       map[string]int `json:"topic_mastery" db:"topic_mastery"`
	ReadinessScore     int            `json:"readiness_score" db:"readiness_score"`
	RecommendedActions []string       `json:"recommended_actions" db:"recommended_actions"`
	TotalDoubtsAsked   int            `json:"total_doubts_asked" db:"total_doubts_asked"`
	CustomInstructions string         `json:"custom_instructions" db:"custom_instructions"`
	LastActiveAt       time.Time      `json:"last_active_at" db:"last_active_at"`
	UpdatedAt          time.Time      `json:"updated_at" db:"updated_at"`
}
