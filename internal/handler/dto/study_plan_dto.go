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

type GenerateAIStudyPlanRequest struct {
	ExamName   string   `json:"exam_name" validate:"required"`
	Syllabus   []string `json:"syllabus" validate:"required,min=1"`
	TargetDays int      `json:"target_days" validate:"required,min=1,max=365"`
}

type GeneratedStudyTask struct {
	DayNumber int      `json:"day_number"`
	Title     string   `json:"title"`
	FocusArea string   `json:"focus_area"`
	Priority  string   `json:"priority"`
	Subtasks  []string `json:"subtasks"`
}

type GenerateAIStudyPlanResponse struct {
	ExamName string               `json:"exam_name"`
	Overview string               `json:"overview"`
	Schedule []GeneratedStudyTask `json:"schedule"`
}
