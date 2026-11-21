package dto

// ---------------------------------------------------------------------------
// Quiz & Examination Generation & Submission DTOs
// ---------------------------------------------------------------------------

type GenerateQuizRequest struct {
	Topic         string  `json:"topic" validate:"required,min=2,max=200"`
	CourseID      *string `json:"course_id" validate:"omitempty"`
	NoteID        *string `json:"note_id" validate:"omitempty"`
	Difficulty    string  `json:"difficulty" validate:"omitempty,oneof=easy medium hard adaptive"`
	TestType      string  `json:"test_type" validate:"omitempty,oneof=mcq subjective mixed"` // "mcq" | "subjective" | "mixed"
	QuestionCount int     `json:"question_count" validate:"omitempty,min=1,max=25"`
	Provider      string  `json:"provider" validate:"omitempty"`
	Model         string  `json:"model" validate:"omitempty"`
	APIKey        string  `json:"api_key" validate:"omitempty"`
}

type QuestionAnswerInput struct {
	QuestionID     string `json:"question_id" validate:"required"`
	QuestionType   string `json:"question_type,omitempty" validate:"omitempty,oneof=mcq subjective"`
	SelectedOption int    `json:"selected_option"` // For MCQ: 0-3 (or -1 if skipped)
	AnswerText     string `json:"answer_text,omitempty"` // For Subjective: written answer
}

type SubmitQuizRequest struct {
	TimeTakenSeconds int                   `json:"time_taken_seconds" validate:"gte=0"`
	Answers          []QuestionAnswerInput `json:"answers" validate:"required,min=1"`
}

type QuestionFeedback struct {
	QuestionID         string   `json:"question_id"`
	QuestionType       string   `json:"question_type"` // "mcq" | "subjective"
	QuestionText       string   `json:"question_text"`
	SelectedOption     int      `json:"selected_option"`
	CorrectOptionIndex int      `json:"correct_option_index"`
	StudentAnswer      string   `json:"student_answer,omitempty"`
	ModelAnswer        string   `json:"model_answer,omitempty"`
	Score              int      `json:"score"`
	MaxMarks           int      `json:"max_marks"`
	IsCorrect          bool     `json:"is_correct"`
	Explanation        string   `json:"explanation"`
	TrapWarning        string   `json:"trap_warning"`
	Feedback           string   `json:"feedback,omitempty"`
	Rubric             []string `json:"rubric,omitempty"`
	KeyPointsCovered   []string `json:"key_points_covered,omitempty"`
	KeyPointsMissed    []string `json:"key_points_missed,omitempty"`
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
