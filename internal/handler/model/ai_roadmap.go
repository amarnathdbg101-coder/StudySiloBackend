package model

import "time"

// AIRoadmap represents an AI-generated exam study roadmap saved to database.
// Ye model exam ke liye AI se generate hue spaced repetition study plan ko hold karta hai.
type AIRoadmap struct {
	ID             string           `json:"id" db:"id"`
	UserID         string           `json:"user_id" db:"user_id"`
	ExamName       string           `json:"exam_name" db:"exam_name"`
	Overview       string           `json:"overview" db:"overview"`
	SyllabusTopics []string         `json:"syllabus_topics" db:"syllabus_topics"`
	TargetDays     int              `json:"target_days" db:"target_days"`
	Schedule       []RoadmapTask    `json:"schedule" db:"schedule"`
	IsActive       bool             `json:"is_active" db:"is_active"`
	CreatedAt      time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at" db:"updated_at"`
}

// RoadmapTask ek din ka study schedule item hai.
// AI generate karta hai - topic, priority aur subtasks ke saath.
type RoadmapTask struct {
	DayNumber int      `json:"day_number"`
	Title     string   `json:"title"`
	FocusArea string   `json:"focus_area"` // "Theory", "Problem Solving", "PYQ Recall", "Spaced Review"
	Priority  string   `json:"priority"`   // "high", "medium", "low"
	Subtasks  []string `json:"subtasks"`
}
