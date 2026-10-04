package services

import (
	"context"
	"testing"
	"time"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/model"
	"goProjectStructure/internal/reuse/ai"
)

type mockQuizRepo struct {
	quizzes  map[string]*model.Quiz
	attempts []*model.QuizAttempt
}

func (m *mockQuizRepo) CreateQuizWithQuestions(ctx context.Context, quiz *model.Quiz, questions []*model.QuizQuestion) error {
	quiz.ID = "quiz-101"
	quiz.CreatedAt = time.Now()
	for i, q := range questions {
		q.ID = "q-" + string(rune('1'+i))
		q.QuizID = quiz.ID
	}
	quiz.Questions = questions
	m.quizzes[quiz.ID] = quiz
	return nil
}

func (m *mockQuizRepo) GetQuizByID(ctx context.Context, id, userID string) (*model.Quiz, error) {
	q, ok := m.quizzes[id]
	if !ok || q.UserID != userID {
		return nil, nil
	}
	return q, nil
}

func (m *mockQuizRepo) ListQuizzesByUser(ctx context.Context, userID string) ([]*model.Quiz, error) {
	list := []*model.Quiz{}
	for _, q := range m.quizzes {
		if q.UserID == userID {
			list = append(list, q)
		}
	}
	return list, nil
}

func (m *mockQuizRepo) SaveAttempt(ctx context.Context, attempt *model.QuizAttempt) error {
	attempt.ID = "att-1"
	m.attempts = append(m.attempts, attempt)
	return nil
}

func (m *mockQuizRepo) ListAttemptsByUser(ctx context.Context, userID, quizID string) ([]*model.QuizAttempt, error) {
	return m.attempts, nil
}

type mockCognitiveRepo struct {
	profiles map[string]*model.CognitiveProfile
}

func (m *mockCognitiveRepo) GetByUserID(ctx context.Context, userID string) (*model.CognitiveProfile, error) {
	p, ok := m.profiles[userID]
	if !ok {
		p = &model.CognitiveProfile{
			UserID:            userID,
			TargetExam:        "JEE Advanced",
			AcademicLevel:     "Grade 12",
			WeakTopics:        []string{"Rotational Dynamics"},
			StrongTopics:      []string{"Kinematics"},
			StudyStreakDays:   3,
			TotalQuizzesTaken: 2,
			AverageQuizScore:  80.0,
		}
		m.profiles[userID] = p
	}
	return p, nil
}

func (m *mockCognitiveRepo) Upsert(ctx context.Context, p *model.CognitiveProfile) error {
	m.profiles[p.UserID] = p
	return nil
}

func (m *mockCognitiveRepo) RecordQuizResult(ctx context.Context, userID string, percentage float64, weak, strong, traps []string) (*model.CognitiveProfile, error) {
	p, _ := m.GetByUserID(ctx, userID)
	p.TotalQuizzesTaken++
	p.AverageQuizScore = (p.AverageQuizScore + percentage) / 2.0
	p.WeakTopics = append(p.WeakTopics, weak...)
	p.StrongTopics = append(p.StrongTopics, strong...)
	p.StudyStreakDays++
	return p, nil
}

func TestQuizService_GenerateAndSubmitFlow(t *testing.T) {
	orch := ai.NewOrchestrator([]string{}, []string{}, []string{})
	qRepo := &mockQuizRepo{quizzes: make(map[string]*model.Quiz)}
	cRepo := &mockCognitiveRepo{profiles: make(map[string]*model.CognitiveProfile)}

	service := NewQuizService(qRepo, cRepo, orch)
	ctx := context.Background()

	// 1. Generate Quiz
	genReq := dto.GenerateQuizRequest{
		Topic:         "Thermodynamics",
		Difficulty:    "adaptive",
		QuestionCount: 2,
	}

	quiz, err := service.GenerateQuiz(ctx, "user-1", genReq)
	if err != nil {
		t.Fatalf("failed to generate adaptive quiz: %v", err)
	}

	if quiz.ID != "quiz-101" || len(quiz.Questions) != 2 {
		t.Fatalf("unexpected quiz result: %+v", quiz)
	}

	// 2. Submit Quiz Answers (1 correct, 1 wrong)
	q1 := quiz.Questions[0]
	q2 := quiz.Questions[1]

	subReq := dto.SubmitQuizRequest{
		TimeTakenSeconds: 45,
		Answers: []dto.QuestionAnswerInput{
			{QuestionID: q1.ID, SelectedOption: q1.CorrectOptionIndex},      // Correct
			{QuestionID: q2.ID, SelectedOption: (q2.CorrectOptionIndex + 1) % 4}, // Wrong
		},
	}

	result, err := service.SubmitQuiz(ctx, quiz.ID, "user-1", subReq)
	if err != nil {
		t.Fatalf("failed to submit quiz: %v", err)
	}

	if result.Score != 1 || result.TotalScore != 2 {
		t.Errorf("expected score 1/2, got %d/%d", result.Score, result.TotalScore)
	}
	if result.Percentage != 50.0 {
		t.Errorf("expected 50%% score, got %f", result.Percentage)
	}
	if len(result.WeakAreasIdentified) == 0 {
		t.Error("expected identified weak areas for incorrect answers")
	}

	// 3. Verify cognitive profile updated
	profile, err := service.GetCognitiveProfile(ctx, "user-1")
	if err != nil {
		t.Fatalf("failed to get cognitive profile: %v", err)
	}

	if profile.StudyStreakDays < 4 {
		t.Errorf("expected updated study streak, got %d", profile.StudyStreakDays)
	}
}

func TestQuizService_GuestUserAndFallback(t *testing.T) {
	orch := ai.NewOrchestrator([]string{}, []string{}, []string{})
	service := NewQuizService(nil, nil, orch)
	ctx := context.Background()

	// 1. Generate Quiz as Guest with 5 questions
	genReq := dto.GenerateQuizRequest{
		Topic:         "Computer Architecture",
		Difficulty:    "adaptive",
		QuestionCount: 5,
	}

	quiz, err := service.GenerateQuiz(ctx, "guest-user", genReq)
	if err != nil {
		t.Fatalf("failed to generate guest quiz: %v", err)
	}

	if quiz == nil || len(quiz.Questions) != 5 {
		t.Fatalf("expected 5 questions for guest quiz, got %v", quiz)
	}

	for i, q := range quiz.Questions {
		if q.QuestionText == "" {
			t.Errorf("question %d has empty text", i)
		}
		if len(q.Options) != 4 {
			t.Errorf("question %d does not have 4 options: %d", i, len(q.Options))
		}
		if q.Explanation == "" {
			t.Errorf("question %d has empty explanation", i)
		}
		if q.TrapWarning == "" {
			t.Errorf("question %d has empty trap warning", i)
		}
	}

	// 2. Submit Quiz as Guest
	answers := make([]dto.QuestionAnswerInput, len(quiz.Questions))
	for i, q := range quiz.Questions {
		answers[i] = dto.QuestionAnswerInput{
			QuestionID:     q.ID,
			SelectedOption: q.CorrectOptionIndex, // All correct
		}
	}

	subReq := dto.SubmitQuizRequest{
		TimeTakenSeconds: 90,
		Answers:          answers,
	}

	result, err := service.SubmitQuiz(ctx, quiz.ID, "guest-user", subReq)
	if err != nil {
		t.Fatalf("failed to submit guest quiz: %v", err)
	}

	if result.Score != 5 || result.Percentage != 100.0 {
		t.Errorf("expected 100%% score, got score=%d percentage=%f", result.Score, result.Percentage)
	}
	if len(result.Feedback) != 5 {
		t.Errorf("expected 5 feedbacks, got %d", len(result.Feedback))
	}
}

