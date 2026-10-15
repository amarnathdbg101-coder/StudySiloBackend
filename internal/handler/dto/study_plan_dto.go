package dto

type CreateStudyPlanRequest struct {
	CourseID *string `json:"course_id,omitempty"`
	Title    string  `json:"title" validate:"required,min=2,max=250"`
	DueDate  *string `json:"due_date,omitempty"`  // ISO8601 string
	Priority string  `json:"priority,omitempty"` // low, medium, high
}

type UpdateStudyPlanRequest struct {
	Title       *string `json:"title,omitempty"`
	IsCompleted *bool   `json:"is_completed,omitempty"`
	Priority    *string `json:"priority,omitempty"`
}

type RecordStudyTimeRequest struct {
	Minutes int `json:"minutes" validate:"required,min=1,max=720"`
}

// GenerateAIStudyPlanRequest - AI se roadmap generate karne ke liye request body
type GenerateAIStudyPlanRequest struct {
	ExamName   string   `json:"exam_name" validate:"required"`
	Syllabus   []string `json:"syllabus" validate:"required,min=1"`
	TargetDays int      `json:"target_days" validate:"required,min=1,max=365"`
	// SaveToDB - agar true hai toh AI response automatically DB mein save ho jaayega
	SaveToDB bool `json:"save_to_db"`
}

type GeneratedStudyTask struct {
	DayNumber int      `json:"day_number"`
	Title     string   `json:"title"`
	FocusArea string   `json:"focus_area"`
	Priority  string   `json:"priority"`
	Subtasks  []string `json:"subtasks"`
}

// GenerateAIStudyPlanResponse - AI roadmap generation ka response (with optional saved roadmap ID)
type GenerateAIStudyPlanResponse struct {
	ExamName   string               `json:"exam_name"`
	Overview   string               `json:"overview"`
	Schedule   []GeneratedStudyTask `json:"schedule"`
	// RoadmapID - agar SaveToDB=true tha toh DB mein save hua roadmap ka UUID
	RoadmapID  *string              `json:"roadmap_id,omitempty"`
}

// SaveRoadmapRequest - already generated roadmap ko explicitly DB mein save karne ke liye
type SaveRoadmapRequest struct {
	ExamName   string               `json:"exam_name" validate:"required"`
	Overview   string               `json:"overview"`
	Syllabus   []string             `json:"syllabus"`
	TargetDays int                  `json:"target_days" validate:"required,min=1,max=365"`
	Schedule   []GeneratedStudyTask `json:"schedule" validate:"required,min=1"`
}

// UpdateRoadmapRequest - existing saved roadmap update karne ke liye (PATCH)
type UpdateRoadmapRequest struct {
	ExamName *string              `json:"exam_name,omitempty"`
	Overview *string              `json:"overview,omitempty"`
	Schedule []GeneratedStudyTask `json:"schedule,omitempty"`
	IsActive *bool                `json:"is_active,omitempty"`
}

