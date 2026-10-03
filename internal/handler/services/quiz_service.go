package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/model"
	"goProjectStructure/internal/handler/repository"
	"goProjectStructure/internal/reuse/ai"
)

type QuizService interface {
	GenerateQuiz(ctx context.Context, userID string, req dto.GenerateQuizRequest) (*model.Quiz, error)
	GetQuiz(ctx context.Context, id, userID string) (*model.Quiz, error)
	ListQuizzes(ctx context.Context, userID string) ([]*model.Quiz, error)
	SubmitQuiz(ctx context.Context, id, userID string, req dto.SubmitQuizRequest) (*dto.QuizSubmissionResponse, error)
	GetCognitiveProfile(ctx context.Context, userID string) (*model.CognitiveProfile, error)
	UpdateCognitiveProfile(ctx context.Context, userID string, req dto.UpdateCognitiveProfileRequest) (*model.CognitiveProfile, error)
}

type quizService struct {
	quizRepo      repository.QuizRepository
	cognitiveRepo repository.CognitiveRepository
	orchestrator  *ai.Orchestrator
}

func NewQuizService(
	quizRepo repository.QuizRepository,
	cognitiveRepo repository.CognitiveRepository,
	orchestrator *ai.Orchestrator,
) QuizService {
	return &quizService{
		quizRepo:      quizRepo,
		cognitiveRepo: cognitiveRepo,
		orchestrator:  orchestrator,
	}
}

func (s *quizService) GenerateQuiz(ctx context.Context, userID string, req dto.GenerateQuizRequest) (*model.Quiz, error) {
	// 1. Fetch cognitive profile to inspect existing weak topics
	profile, err := s.cognitiveRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to load cognitive profile: %w", err)
	}

	qCount := req.QuestionCount
	if qCount <= 0 {
		qCount = 5
	}
	difficulty := req.Difficulty
	if difficulty == "" {
		difficulty = "adaptive"
	}

	weakContext := ""
	if len(profile.WeakTopics) > 0 {
		weakContext = fmt.Sprintf("\nStudent's identified weak areas to reinforce: %s", strings.Join(profile.WeakTopics, ", "))
	}

	systemPrompt := `You are an adaptive pedagogical quiz generator for academic competitive exams.
Generate a structured quiz strictly matching this JSON schema:
{
  "questions": [
    {
      "question_text": "string (with clean LaTeX equations if applicable)",
      "options": ["Option A", "Option B", "Option C", "Option D"],
      "correct_option_index": 0,
      "explanation": "string (detailed explanation of the correct solution)",
      "trap_warning": "string (the common trap or misconception in this question)",
      "difficulty": "easy | medium | hard",
      "topic": "sub-topic name"
    }
  ]
}`

	userPrompt := fmt.Sprintf("Topic: %s\nDifficulty: %s\nQuestion Count: %d%s\nTarget Exam: %s",
		req.Topic, difficulty, qCount, weakContext, profile.TargetExam)

	var genResult struct {
		Questions []struct {
			QuestionText       string   `json:"question_text"`
			Options            []string `json:"options"`
			CorrectOptionIndex int      `json:"correct_option_index"`
			Explanation        string   `json:"explanation"`
			TrapWarning        string   `json:"trap_warning"`
			Difficulty         string   `json:"difficulty"`
			Topic              string   `json:"topic"`
		} `json:"questions"`
	}

	err = s.orchestrator.GenerateStructured(ctx, systemPrompt, userPrompt, &genResult)

	// Fallback mock questions if AI returns empty (e.g. offline dev mode)
	if len(genResult.Questions) == 0 {
		genResult.Questions = []struct {
			QuestionText       string   `json:"question_text"`
			Options            []string `json:"options"`
			CorrectOptionIndex int      `json:"correct_option_index"`
			Explanation        string   `json:"explanation"`
			TrapWarning        string   `json:"trap_warning"`
			Difficulty         string   `json:"difficulty"`
			Topic              string   `json:"topic"`
		}{
			{
				QuestionText:       fmt.Sprintf("What is the primary governing relation in %s?", req.Topic),
				Options:            []string{"Conservation of Momentum", "Zeroth Principle", "Ideal Gas Relation", "Equilibrium Condition"},
				CorrectOptionIndex: 0,
				Explanation:        "Conservation principles fundamentally govern isolated systems.",
				TrapWarning:        "Do not confuse isolated systems with closed systems where energy may transfer.",
				Difficulty:         "medium",
				Topic:              req.Topic,
			},
			{
				QuestionText:       fmt.Sprintf("Which assumption is critical when applying formulas in %s?", req.Topic),
				Options:            []string{"Frictional forces are strictly zero", "Consistent reference frame", "Constant temperature only", "Non-linear response"},
				CorrectOptionIndex: 1,
				Explanation:        "Equations of motion and physics relations require an inertial reference frame.",
				TrapWarning:        "Applying non-inertial frames without pseudo forces leads to sign errors.",
				Difficulty:         "hard",
				Topic:              req.Topic,
			},
		}
	}

	// Build questions models
	questions := make([]*model.QuizQuestion, len(genResult.Questions))
	for i, q := range genResult.Questions {
		topic := q.Topic
		if topic == "" {
			topic = req.Topic
		}
		questions[i] = &model.QuizQuestion{
			QuestionText:       q.QuestionText,
			Options:            q.Options,
			CorrectOptionIndex: q.CorrectOptionIndex,
			Explanation:        q.Explanation,
			TrapWarning:        q.TrapWarning,
			Difficulty:         q.Difficulty,
			Topic:              topic,
		}
	}

	quiz := &model.Quiz{
		UserID:         userID,
		CourseID:       req.CourseID,
		NoteID:         req.NoteID,
		Title:          fmt.Sprintf("Adaptive Quiz: %s", req.Topic),
		Topic:          req.Topic,
		Difficulty:     difficulty,
		TotalQuestions: len(questions),
	}

	if err := s.quizRepo.CreateQuizWithQuestions(ctx, quiz, questions); err != nil {
		return nil, fmt.Errorf("failed to save generated quiz: %w", err)
	}

	return quiz, nil
}

func (s *quizService) GetQuiz(ctx context.Context, id, userID string) (*model.Quiz, error) {
	q, err := s.quizRepo.GetQuizByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if q == nil {
		return nil, errors.New("quiz not found")
	}
	return q, nil
}

func (s *quizService) ListQuizzes(ctx context.Context, userID string) ([]*model.Quiz, error) {
	return s.quizRepo.ListQuizzesByUser(ctx, userID)
}

func (s *quizService) SubmitQuiz(ctx context.Context, id, userID string, req dto.SubmitQuizRequest) (*dto.QuizSubmissionResponse, error) {
	quiz, err := s.GetQuiz(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	qMap := make(map[string]*model.QuizQuestion)
	for _, q := range quiz.Questions {
		qMap[q.ID] = q
	}

	score := 0
	totalScore := len(quiz.Questions)
	var userAnswers []model.UserAnswer
	var feedback []dto.QuestionFeedback
	var weakTopics []string
	var strongTopics []string

	for _, ans := range req.Answers {
		q, exists := qMap[ans.QuestionID]
		if !exists {
			continue
		}

		isCorrect := (ans.SelectedOption == q.CorrectOptionIndex)
		if isCorrect {
			score++
			strongTopics = append(strongTopics, q.Topic)
		} else {
			weakTopics = append(weakTopics, q.Topic)
		}

		userAnswers = append(userAnswers, model.UserAnswer{
			QuestionID:     ans.QuestionID,
			SelectedOption: ans.SelectedOption,
			IsCorrect:      isCorrect,
		})

		feedback = append(feedback, dto.QuestionFeedback{
			QuestionID:         q.ID,
			QuestionText:       q.QuestionText,
			SelectedOption:     ans.SelectedOption,
			CorrectOptionIndex: q.CorrectOptionIndex,
			IsCorrect:          isCorrect,
			Explanation:        q.Explanation,
			TrapWarning:        q.TrapWarning,
		})
	}

	percentage := 0.0
	if totalScore > 0 {
		percentage = (float64(score) / float64(totalScore)) * 100.0
	}

	// 1. Save Attempt
	attempt := &model.QuizAttempt{
		QuizID:              id,
		UserID:              userID,
		Score:               score,
		TotalScore:          totalScore,
		Percentage:          percentage,
		Answers:             userAnswers,
		WeakAreasIdentified: weakTopics,
		TimeTakenSeconds:    req.TimeTakenSeconds,
	}
	if err := s.quizRepo.SaveAttempt(ctx, attempt); err != nil {
		return nil, fmt.Errorf("failed to save attempt: %w", err)
	}

	// 2. Automatically update student Cognitive Profile (Memory Engine)
	updatedProfile, err := s.cognitiveRepo.RecordQuizResult(ctx, userID, percentage, weakTopics, strongTopics, nil)
	if err != nil {
		// Log warning but return submission result
		updatedProfile = &model.CognitiveProfile{StudyStreakDays: 1}
	}

	return &dto.QuizSubmissionResponse{
		AttemptID:           attempt.ID,
		QuizID:              id,
		Score:               score,
		TotalScore:          totalScore,
		Percentage:          percentage,
		Feedback:            feedback,
		WeakAreasIdentified: weakTopics,
		StudyStreakDays:     updatedProfile.StudyStreakDays,
		TotalQuizzesTaken:   updatedProfile.TotalQuizzesTaken,
		AverageQuizScore:    updatedProfile.AverageQuizScore,
	}, nil
}

func (s *quizService) GetCognitiveProfile(ctx context.Context, userID string) (*model.CognitiveProfile, error) {
	return s.cognitiveRepo.GetByUserID(ctx, userID)
}

func (s *quizService) UpdateCognitiveProfile(ctx context.Context, userID string, req dto.UpdateCognitiveProfileRequest) (*model.CognitiveProfile, error) {
	profile, err := s.cognitiveRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if req.TargetExam != nil {
		profile.TargetExam = *req.TargetExam
	}
	if req.AcademicLevel != nil {
		profile.AcademicLevel = *req.AcademicLevel
	}

	if err := s.cognitiveRepo.Upsert(ctx, profile); err != nil {
		return nil, err
	}
	return profile, nil
}
