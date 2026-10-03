package dto

// ---------------------------------------------------------------------------
// Quiz Generation & Submission DTOs
// ---------------------------------------------------------------------------

type GenerateQuizRequest struct {
	Topic         string  `json:"topic" validate:"required,min=2,max=200"`
	CourseID      *string `json:"course_id" validate:"omitempty,uuid"`
	NoteID        *string `json:"note_id" validate:"omitempty,uuid"`
	Difficulty    string  `json:"difficulty" validate:"omitempty,oneof=adaptive easy medium hard"`
	QuestionCount int     `json:"question_count" validate:"omitempty,min=1,max=20"`
}

type QuestionAnswerInput struct {
	QuestionID     string `json:"question_id" validate:"required,uuid"`
	SelectedOption int    `json:"selected_option" validate:"gte=0"`
}

type SubmitQuizRequest struct {
	TimeTakenSeconds int                   `json:"time_taken_seconds" validate:"gte=0"`
	Answers          []QuestionAnswerInput `json:"answers" validate:"required,min=1"`
}

type QuestionFeedback struct {
	QuestionID         string `json:"question_id"`
	QuestionText       string `json:"question_text"`
	SelectedOption     int    `json:"selected_option"`
	CorrectOptionIndex int    `json:"correct_option_index"`
	IsCorrect          bool   `json:"is_correct"`
	Explanation        string `json:"explanation"`
	TrapWarning        string `json:"trap_warning"`
}

type QuizSubmissionResponse struct {
	AttemptID           string             `json:"attempt_id"`
	QuizID              string             `json:"quiz_id"`
	Score               int                `json:"score"`
	TotalScore          int                `json:"total_score"`
	Percentage          float64            `json:"percentage"`
	Feedback            []QuestionFeedback `json:"feedback"`
	WeakAreasIdentified []string           `json:"weak_areas_identified"`
	StudyStreakDays     int                `json:"study_streak_days"`
	TotalQuizzesTaken   int                `json:"total_quizzes_taken"`
	AverageQuizScore    float64            `json:"average_quiz_score"`
}

// ---------------------------------------------------------------------------
// Cognitive Profile DTOs
// ---------------------------------------------------------------------------

type UpdateCognitiveProfileRequest struct {
	TargetExam    *string `json:"target_exam" validate:"omitempty,max=250"`
	AcademicLevel *string `json:"academic_level" validate:"omitempty,max=100"`
}
