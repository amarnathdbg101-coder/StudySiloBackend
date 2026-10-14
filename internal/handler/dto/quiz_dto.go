package dto

// ---------------------------------------------------------------------------
// Quiz Generation & Submission DTOs
// ---------------------------------------------------------------------------

type GenerateQuizRequest struct {
	Topic         string  `json:"topic" validate:"required,min=2,max=200"`
	CourseID      *string `json:"course_id" validate:"omitempty"`
	NoteID        *string `json:"note_id" validate:"omitempty"`
	Difficulty    string  `json:"difficulty" validate:"omitempty"`
	QuestionCount int     `json:"question_count" validate:"omitempty"`
}

type QuestionAnswerInput struct {
	QuestionID     string `json:"question_id" validate:"required"`
	SelectedOption int    `json:"selected_option"` // allowed to be -1 if skipped or unanswered
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
	TargetExam       *string   `json:"target_exam" validate:"omitempty,max=250"`
	AcademicLevel    *string   `json:"academic_level" validate:"omitempty,max=100"`
	WeakTopics       *[]string `json:"weak_topics" validate:"omitempty"`
	StrongTopics     *[]string `json:"strong_topics" validate:"omitempty"`
	ExamTrapsToWatch *[]string `json:"exam_traps_to_watch" validate:"omitempty"`
}
