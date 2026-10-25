package dto

import "time"

// CognitiveProfileResponse provides complete academic memory and AI diagnostic state
type CognitiveProfileResponse struct {
	UserID             string         `json:"user_id"`
	TargetExam         string         `json:"target_exam"`
	AcademicLevel      string         `json:"academic_level"`
	WeakTopics         []string       `json:"weak_topics"`
	StrongTopics       []string       `json:"strong_topics"`
	ExamTraps          []string       `json:"exam_traps"`
	StudyStreakDays    int            `json:"study_streak_days"`
	TotalStudyMinutes  int            `json:"total_study_minutes"`
	TotalQuizzesTaken  int            `json:"total_quizzes_taken"`
	AverageQuizScore   float64        `json:"average_quiz_score"`
	AISummary          string         `json:"ai_summary"`
	TopicMastery       map[string]int `json:"topic_mastery"`
	ReadinessScore     int            `json:"readiness_score"`
	RecommendedActions []string       `json:"recommended_actions"`
	TotalDoubtsAsked   int            `json:"total_doubts_asked"`
	CustomInstructions string         `json:"custom_instructions"`
	LastActiveAt       time.Time      `json:"last_active_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

// UpdateCognitiveGoalRequest handles manual editing of exam goals or topics
type UpdateCognitiveGoalRequest struct {
	TargetExam         *string   `json:"target_exam" validate:"omitempty,max=250"`
	AcademicLevel      *string   `json:"academic_level" validate:"omitempty,max=100"`
	CustomInstructions *string   `json:"custom_instructions" validate:"omitempty,max=1000"`
	WeakTopics         *[]string `json:"weak_topics" validate:"omitempty"`
	StrongTopics       *[]string `json:"strong_topics" validate:"omitempty"`
	ExamTrapsToWatch   *[]string `json:"exam_traps_to_watch" validate:"omitempty"`
}

// TopicActionRequest allows promoting or demoting a topic between Weak and Mastered
type TopicActionRequest struct {
	Topic string `json:"topic" validate:"required,min=2,max=200"`
}

// RecordActivityRequest logs learning events (doubts asked, mind maps created, study time)
type RecordActivityRequest struct {
	ActivityType    string `json:"activity_type" validate:"required,oneof=doubt quiz mindmap study"`
	Topic           string `json:"topic" validate:"omitempty,max=200"`
	DurationMinutes int    `json:"duration_minutes" validate:"omitempty,gte=0"`
	ScorePercentage *float64 `json:"score_percentage,omitempty"`
}

// CognitiveDiagnosisResponse represents an on-demand AI diagnostic assessment
type CognitiveDiagnosisResponse struct {
	AISummary          string         `json:"ai_summary"`
	ReadinessScore     int            `json:"readiness_score"`
	WeakTopics         []string       `json:"weak_topics"`
	StrongTopics       []string       `json:"strong_topics"`
	ExamTraps          []string       `json:"exam_traps"`
	RecommendedActions []string       `json:"recommended_actions"`
	TopicMastery       map[string]int `json:"topic_mastery"`
}

// SmartPromptItemDto represents a dynamically generated context-aware prompt suggestion.
type SmartPromptItemDto struct {
	Label  string `json:"label"`
	Prompt string `json:"prompt"`
	Icon   string `json:"icon"` // e.g., "track_changes", "lightbulb", "functions", "bolt", "timer", "compare_arrows", "help", "layers", "warning", "table", "checklist", "science", "rule", "show_chart", "psychology"
}

// SmartContextResponse contains complete server-synthesized prompts, hints, and syllabus recommendations.
type SmartContextResponse struct {
	TargetExam       string               `json:"target_exam"`
	QuickChips       []SmartPromptItemDto `json:"quick_chips"`
	TutorSuggestions []SmartPromptItemDto `json:"tutor_suggestions"`
	SuggestedTopics  []string             `json:"suggested_topics"`
	FocusTopic       string               `json:"focus_topic"`
	InputHint        string               `json:"input_hint"`
	TopicHint        string               `json:"topic_hint"`
}

