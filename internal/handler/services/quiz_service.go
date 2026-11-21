package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/model"
	"goProjectStructure/internal/handler/repository"
	"goProjectStructure/internal/middleware"
	"goProjectStructure/internal/reuse/ai"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type QuizService interface {
	GenerateQuiz(ctx context.Context, userID string, req dto.GenerateQuizRequest) (*model.Quiz, error)
	GetQuiz(ctx context.Context, id, userID string) (*model.Quiz, error)
	ListQuizzes(ctx context.Context, userID string) ([]*model.Quiz, error)
	DeleteQuiz(ctx context.Context, id, userID string) error
	SubmitQuiz(ctx context.Context, id, userID string, req dto.SubmitQuizRequest) (*dto.QuizSubmissionResponse, error)
	ListAttempts(ctx context.Context, userID, quizID string) ([]*model.QuizAttempt, error)
	GetCognitiveProfile(ctx context.Context, userID string) (*model.CognitiveProfile, error)
	UpdateCognitiveProfile(ctx context.Context, userID string, req dto.UpdateCognitiveProfileRequest) (*model.CognitiveProfile, error)
}

type quizService struct {
	quizRepo      repository.QuizRepository
	cognitiveRepo repository.CognitiveRepository
	orchestrator  *ai.Orchestrator
	memQuizzes    sync.Map // string (quizID) -> *model.Quiz
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

func isValidUUID(u string) bool {
	if u == "" {
		return false
	}
	_, err := uuid.Parse(u)
	return err == nil
}

func (s *quizService) saveInMemoryQuiz(q *model.Quiz) {
	if q == nil || q.ID == "" {
		return
	}
	s.memQuizzes.Store(q.ID, q)
}

func (s *quizService) getInMemoryQuiz(id string) *model.Quiz {
	if id == "" {
		return nil
	}
	if val, ok := s.memQuizzes.Load(id); ok {
		if q, ok := val.(*model.Quiz); ok {
			return q
		}
	}
	return nil
}

func (s *quizService) GenerateQuiz(ctx context.Context, userID string, req dto.GenerateQuizRequest) (*model.Quiz, error) {
	// 1. Fetch cognitive profile to inspect existing weak topics (graceful fallback if guest or not seeded)
	var profile *model.CognitiveProfile
	if userID != "" && userID != "guest-user" && s.cognitiveRepo != nil {
		p, err := s.cognitiveRepo.GetByUserID(ctx, userID)
		if err == nil && p != nil {
			profile = p
		}
	}
	if profile == nil {
		profile = &model.CognitiveProfile{
			UserID:        userID,
			TargetExam:    "Competitive / University Exam",
			AcademicLevel: "Undergraduate",
			WeakTopics:    []string{},
		}
	}

	qCount := req.QuestionCount
	if qCount <= 0 {
		qCount = 5
	}
	if qCount > 15 {
		qCount = 15
	}
	difficulty := strings.ToLower(strings.TrimSpace(req.Difficulty))
	if difficulty == "" {
		difficulty = "adaptive"
	}

	testType := strings.ToLower(strings.TrimSpace(req.TestType))
	if testType == "" {
		testType = "mcq"
	} else if testType != "mcq" && testType != "subjective" && testType != "mixed" {
		testType = "mcq"
	}

	weakContext := ""
	if len(profile.WeakTopics) > 0 {
		weakContext = fmt.Sprintf("\nStudent's identified weak areas to reinforce: %s", strings.Join(profile.WeakTopics, ", "))
	}

	var systemPrompt string
	switch testType {
	case "subjective":
		systemPrompt = `You are an elite, rigorous university professor and competitive exam creator.
Generate an in-depth subjective examination strictly matching this JSON schema:
{
  "questions": [
    {
      "question_text": "Detailed, thought-provoking academic problem requiring conceptual explanation, derivation, or analytical reasoning with LaTeX ($...$ or $$...$$)",
      "question_type": "subjective",
      "options": [],
      "correct_option_index": -1,
      "max_marks": 5,
      "model_answer": "Complete, exemplary full-marks academic answer detailing definitions, step-by-step logic, equations, and edge cases",
      "rubric": [
        "2 marks: Accurate definition and foundational principles",
        "2 marks: Analytical derivation/explanation with correct equations",
        "1 mark: Practical considerations, constraints, or real-world example"
      ],
      "explanation": "In-depth pedagogical proof, conceptual background, and breakdown",
      "trap_warning": "Common mistakes, omissions, or faulty assumptions students make on this problem",
      "difficulty": "easy | medium | hard",
      "topic": "Specific sub-topic"
    }
  ]
}
RULES:
1. Generate EXACTLY the requested question count.
2. Every question MUST be 'question_type': 'subjective'.
3. 'options' MUST be an empty array [] and 'correct_option_index' MUST be -1.
4. 'model_answer' MUST be comprehensive, academically rigorous, and accurate.
5. 'rubric' MUST provide clear marks distribution criteria totaling 'max_marks'.
6. 'max_marks' must be 5 (or 10 for hard analytical problems).
7. Return ONLY valid JSON.`

	case "mixed":
		systemPrompt = `You are an elite, comprehensive academic examination creator.
Generate a balanced examination featuring BOTH Multiple Choice Questions (MCQ) and In-Depth Subjective Questions strictly matching this JSON schema:
{
  "questions": [
    {
      "question_text": "Problem text with clean LaTeX equations ($...$ or $$...$$) where applicable",
      "question_type": "mcq",
      "options": [
        "A) Plausible option text",
        "B) Plausible option text",
        "C) Plausible option text",
        "D) Plausible option text"
      ],
      "correct_option_index": 0,
      "max_marks": 1,
      "model_answer": "",
      "rubric": [],
      "explanation": "Pedagogical derivation and conceptual proof",
      "trap_warning": "Specific misconception or examiner trap",
      "difficulty": "easy | medium | hard",
      "topic": "Specific sub-topic"
    },
    {
      "question_text": "In-depth analytical or derivation question requiring student to write a descriptive response",
      "question_type": "subjective",
      "options": [],
      "correct_option_index": -1,
      "max_marks": 5,
      "model_answer": "Full exemplary answer detailing logic, formulas, and proofs",
      "rubric": [
        "2 marks: Core definition and governing axioms",
        "2 marks: Step-by-step logical analysis",
        "1 mark: Boundary cases and conclusions"
      ],
      "explanation": "In-depth pedagogical proof and context",
      "trap_warning": "Common conceptual or derivation traps",
      "difficulty": "easy | medium | hard",
      "topic": "Specific sub-topic"
    }
  ]
}
RULES:
1. Generate EXACTLY the requested question count (~60% MCQ, ~40% Subjective).
2. For MCQ: exactly 4 options, valid correct_option_index (0-3), max_marks: 1.
3. For Subjective: options=[], correct_option_index=-1, detailed model_answer, clear rubric criteria, max_marks: 5 (or 10 for hard).
4. Return ONLY valid JSON.`

	default: // "mcq"
		systemPrompt = `You are an elite, adaptive pedagogical quiz generator for academic competitive & university semester exams.
Generate a structured quiz strictly matching this JSON schema:
{
  "questions": [
    {
      "question_text": "Detailed question text with clean LaTeX equations ($...$ or $$...$$) where applicable",
      "question_type": "mcq",
      "options": [
        "A) Plausible option text with formulas/terms",
        "B) Plausible option text",
        "C) Plausible option text",
        "D) Plausible option text"
      ],
      "correct_option_index": 0,
      "max_marks": 1,
      "explanation": "In-depth pedagogical explanation with step-by-step mathematical derivation and conceptual proof",
      "trap_warning": "Specific misconception, calculation pitfall, or trick examiners use for this problem",
      "difficulty": "easy | medium | hard",
      "topic": "Specific sub-topic"
    }
  ]
}
RULES:
1. Generate EXACTLY the requested question count.
2. Every question MUST have exactly 4 diverse, non-trivial options.
3. correct_option_index must be 0, 1, 2, or 3 corresponding to the correct option.
4. Always provide a rich explanation with clean LaTeX formulas and a high-yield trap warning.
5. Return ONLY valid JSON.`
	}

	diffInstruction := ""
	switch difficulty {
	case "easy":
		diffInstruction = "\nDifficulty Instruction: Emphasize fundamental principles, direct definitions, foundational theorems, and essential standard scenarios."
	case "medium":
		diffInstruction = "\nDifficulty Instruction: Emphasize multi-step problem solving, comparison between approaches, edge cases, and practical trade-offs."
	case "hard":
		diffInstruction = "\nDifficulty Instruction: Emphasize deep synthesis, rigorous derivations, non-linear edge cases, advanced optimization, and examiner traps."
	default:
		diffInstruction = "\nDifficulty Instruction: Adaptive progression from foundational to advanced concepts."
	}
	aiLanguage := middleware.GetAILanguage(ctx)
	systemPrompt += ai.BuildLanguageInstruction(aiLanguage)

	userPrompt := fmt.Sprintf("Topic: %s\nTest Format: %s\nTarget Difficulty: %s%s\nExact Question Count: %d%s\nTarget Exam: %s\nAcademic Level: %s",
		req.Topic, testType, difficulty, diffInstruction, qCount, weakContext, profile.TargetExam, profile.AcademicLevel)

	var questions []*model.QuizQuestion

	provider := strings.TrimSpace(req.Provider)
	if provider == "" {
		provider = "google"
	}
	modelName := strings.TrimSpace(req.Model)
	if modelName == "" {
		modelName = "gemini-2.0-flash"
	}

	// Attempt AI generation using orchestrator
	if s.orchestrator != nil {
		var aiText string
		var err error

		if req.APIKey != "" {
			aiText, err = s.orchestrator.GenerateTextWithCustomKey(ctx, req.APIKey, provider, modelName, systemPrompt, userPrompt)
		} else {
			aiText, err = s.orchestrator.GenerateTextWithModel(ctx, provider, modelName, systemPrompt, userPrompt)
		}

		if err != nil {
			zap.L().Warn("Quiz AI orchestrator generation failed",
				zap.String("topic", req.Topic),
				zap.String("provider", provider),
				zap.String("model", modelName),
				zap.Error(err),
			)
			// If custom key or model failed, try server-configured default provider as secondary attempt
			if req.APIKey != "" {
				if fallbackText, fbErr := s.orchestrator.GenerateText(ctx, systemPrompt, userPrompt); fbErr == nil && strings.TrimSpace(fallbackText) != "" {
					aiText = fallbackText
					err = nil
				}
			}
		}

		if err == nil && strings.TrimSpace(aiText) != "" {
			questions = parseAIQuizQuestions(aiText, req.Topic, difficulty)
			zap.L().Info("Quiz AI generation finished",
				zap.String("topic", req.Topic),
				zap.String("test_type", testType),
				zap.Int("questions_parsed", len(questions)),
			)
		}
	}

	// If AI output had missing items or failed, fill up to qCount with domain-intelligent questions
	if len(questions) < qCount {
		fallbacks := generateDomainQuestions(req.Topic, difficulty, testType, qCount)
		for _, f := range fallbacks {
			if len(questions) >= qCount {
				break
			}
			questions = append(questions, f)
		}
	}

	quizID := uuid.New().String()
	for i, q := range questions {
		if q.ID == "" {
			q.ID = uuid.New().String()
		}
		q.QuizID = quizID
		if q.CreatedAt.IsZero() {
			q.CreatedAt = time.Now()
		}
		if q.Topic == "" {
			q.Topic = req.Topic
		}
		if q.Difficulty == "" {
			q.Difficulty = difficulty
		}
		if q.QuestionType == "" {
			if len(q.Options) >= 2 {
				q.QuestionType = "mcq"
			} else {
				q.QuestionType = "subjective"
			}
		}
		if q.QuestionType == "subjective" {
			if q.MaxMarks <= 0 {
				if q.Difficulty == "hard" {
					q.MaxMarks = 10
				} else {
					q.MaxMarks = 5
				}
			}
			q.CorrectOptionIndex = -1
			if q.Options == nil {
				q.Options = []string{}
			}
		} else {
			if q.MaxMarks <= 0 {
				q.MaxMarks = 1
			}
		}
		q.QuestionText = ai.SanitizeLatex(q.QuestionText)
		for optIdx := range q.Options {
			q.Options[optIdx] = ai.SanitizeLatex(q.Options[optIdx])
		}
		q.Explanation = ai.SanitizeLatex(q.Explanation)
		q.TrapWarning = ai.SanitizeLatex(q.TrapWarning)
		q.ModelAnswer = ai.SanitizeLatex(q.ModelAnswer)
		for rIdx := range q.Rubric {
			q.Rubric[rIdx] = ai.SanitizeLatex(q.Rubric[rIdx])
		}
		questions[i] = q
	}

	titlePrefix := "Adaptive Quiz"
	if testType == "subjective" {
		titlePrefix = "Subjective Examination"
	} else if testType == "mixed" {
		titlePrefix = "Comprehensive Exam (MCQ + Subjective)"
	}

	quiz := &model.Quiz{
		ID:             quizID,
		UserID:         userID,
		CourseID:       req.CourseID,
		NoteID:         req.NoteID,
		Title:          fmt.Sprintf("%s: %s", titlePrefix, req.Topic),
		Topic:          req.Topic,
		Difficulty:     difficulty,
		TestType:       testType,
		TotalQuestions: len(questions),
		CreatedAt:      time.Now(),
		Questions:      questions,
	}

	// Always save in-memory so guest users or DB hiccups never fail quiz retrieval and grading
	s.saveInMemoryQuiz(quiz)

	// Persist to database if user is authenticated (non-guest)
	if userID != "" && userID != "guest-user" && s.quizRepo != nil {
		dbQuiz := *quiz
		if req.CourseID != nil && !isValidUUID(*req.CourseID) {
			dbQuiz.CourseID = nil
		}
		if req.NoteID != nil && !isValidUUID(*req.NoteID) {
			dbQuiz.NoteID = nil
		}
		if err := s.quizRepo.CreateQuizWithQuestions(ctx, &dbQuiz, questions); err != nil {
			zap.L().Warn("Could not persist quiz to database (FK constraint or guest session), using in-memory session", zap.Error(err))
		} else {
			quiz.ID = dbQuiz.ID
			s.saveInMemoryQuiz(quiz)
		}
	}

	return quiz, nil
}

func (s *quizService) GetQuiz(ctx context.Context, id, userID string) (*model.Quiz, error) {
	if isValidUUID(id) && userID != "" && userID != "guest-user" && s.quizRepo != nil {
		q, err := s.quizRepo.GetQuizByID(ctx, id, userID)
		if err == nil && q != nil && len(q.Questions) > 0 {
			return q, nil
		}
	}
	if q := s.getInMemoryQuiz(id); q != nil {
		return q, nil
	}
	return nil, errors.New("quiz not found")
}

func (s *quizService) ListQuizzes(ctx context.Context, userID string) ([]*model.Quiz, error) {
	var list []*model.Quiz
	if userID != "" && userID != "guest-user" && s.quizRepo != nil {
		l, err := s.quizRepo.ListQuizzesByUser(ctx, userID)
		if err == nil {
			list = l
		}
	}

	seen := make(map[string]bool)
	for _, q := range list {
		seen[q.ID] = true
	}

	// Include in-memory active quizzes
	s.memQuizzes.Range(func(key, val interface{}) bool {
		q, ok := val.(*model.Quiz)
		if ok && q != nil && !seen[q.ID] {
			if userID == "" || userID == "guest-user" || q.UserID == userID {
				list = append([]*model.Quiz{q}, list...)
				seen[q.ID] = true
			}
		}
		return true
	})

	return list, nil
}

// DeleteQuiz deletes a quiz by ID from database and memory
func (s *quizService) DeleteQuiz(ctx context.Context, id, userID string) error {
	s.memQuizzes.Delete(id)
	if isValidUUID(id) && userID != "" && userID != "guest-user" && s.quizRepo != nil {
		return s.quizRepo.DeleteQuiz(ctx, id, userID)
	}
	return nil
}

// ListAttempts returns past quiz submission attempts for a student
func (s *quizService) ListAttempts(ctx context.Context, userID, quizID string) ([]*model.QuizAttempt, error) {
	if userID != "" && userID != "guest-user" && s.quizRepo != nil {
		return s.quizRepo.ListAttemptsByUser(ctx, userID, quizID)
	}
	return []*model.QuizAttempt{}, nil
}


func (s *quizService) SubmitQuiz(ctx context.Context, id, userID string, req dto.SubmitQuizRequest) (*dto.QuizSubmissionResponse, error) {
	quiz, err := s.GetQuiz(ctx, id, userID)
	if err != nil || quiz == nil {
		quiz = s.getInMemoryQuiz(id)
	}
	if quiz == nil {
		return nil, errors.New("quiz not found or session expired")
	}

	ansMap := make(map[string]dto.QuestionAnswerInput)
	for _, a := range req.Answers {
		ansMap[a.QuestionID] = a
	}

	type evalResult struct {
		score            int
		maxMarks         int
		isCorrect        bool
		feedback         string
		keyPointsCovered []string
		keyPointsMissed  []string
		studentSelected  int
		studentText      string
	}

	totalQuestions := len(quiz.Questions)
	evalResults := make([]evalResult, totalQuestions)

	// ⚡ Go Concurrency: Grade subjective questions concurrently using sync.WaitGroup
	// Common Go Concurrency Mistakes Avoided:
	// 1. Calling wg.Add(1) INSIDE goroutine causes a race condition where wg.Wait() returns early.
	//    Solution: Call wg.Add(1) strictly before spawning go func().
	// 2. Data race when appending to a shared slice without mutex locks.
	//    Solution: Pre-allocate evalResults with exact length len(quiz.Questions) and assign to distinct index evalResults[i].
	// 3. Capturing loop iteration variables by reference (in older Go versions).
	//    Solution: Pass index, question pointer, and student answer as value parameters to the goroutine.
	var wg sync.WaitGroup

	for i, q := range quiz.Questions {
		ans, hasAns := ansMap[q.ID]

		maxMarks := q.MaxMarks
		if maxMarks <= 0 {
			if q.QuestionType == "subjective" {
				maxMarks = 5
			} else {
				maxMarks = 1
			}
		}

		qType := q.QuestionType
		if qType == "" {
			if len(q.Options) >= 2 {
				qType = "mcq"
			} else {
				qType = "subjective"
			}
		}

		if !hasAns {
			evalResults[i] = evalResult{
				score:           0,
				maxMarks:        maxMarks,
				isCorrect:       false,
				feedback:        "Question not attempted.",
				studentSelected: -1,
				studentText:     "",
			}
			continue
		}

		studentSelected := ans.SelectedOption
		studentText := strings.TrimSpace(ans.AnswerText)

		if qType == "subjective" {
			if studentText == "" {
				evalResults[i] = evalResult{
					score:           0,
					maxMarks:        maxMarks,
					isCorrect:       false,
					feedback:        "Question not attempted (no written response provided).",
					studentSelected: -1,
					studentText:     "",
				}
				continue
			}

			wg.Add(1)
			go func(idx int, question *model.QuizQuestion, sText string, qMaxMarks int) {
				defer wg.Done()
				score, isCorrect, feedback, covered, missed := s.evaluateSubjectiveAnswer(ctx, question, sText)
				if score > qMaxMarks {
					score = qMaxMarks
				}
				evalResults[idx] = evalResult{
					score:            score,
					maxMarks:         qMaxMarks,
					isCorrect:        isCorrect,
					feedback:         feedback,
					keyPointsCovered: covered,
					keyPointsMissed:  missed,
					studentSelected:  -1,
					studentText:      sText,
				}
			}(i, q, studentText, maxMarks)
		} else {
			// Fast deterministic grading for Multiple Choice Questions (MCQ)
			if studentSelected < 0 || studentSelected >= len(q.Options) {
				evalResults[i] = evalResult{
					score:           0,
					maxMarks:        maxMarks,
					isCorrect:       false,
					feedback:        "Question not attempted.",
					studentSelected: -1,
					studentText:     "",
				}
			} else {
				isCorrect := (studentSelected == q.CorrectOptionIndex)
				score := 0
				if isCorrect {
					score = maxMarks
				}
				evalResults[i] = evalResult{
					score:           score,
					maxMarks:        maxMarks,
					isCorrect:       isCorrect,
					studentSelected: studentSelected,
					studentText:     "",
				}
			}
		}
	}
	wg.Wait()

	// Compute overall total max score for the quiz
	totalScore := 0
	for _, q := range quiz.Questions {
		mm := q.MaxMarks
		if mm <= 0 {
			if q.QuestionType == "subjective" {
				mm = 5
			} else {
				mm = 1
			}
		}
		totalScore += mm
	}
	if totalScore <= 0 {
		totalScore = len(quiz.Questions)
	}

	score := 0
	var userAnswers []model.UserAnswer
	var feedback []dto.QuestionFeedback
	var weakTopics []string
	var strongTopics []string

	for i, q := range quiz.Questions {
		res := evalResults[i]
		score += res.score

		if res.isCorrect {
			strongTopics = append(strongTopics, q.Topic)
		} else {
			weakTopics = append(weakTopics, q.Topic)
		}

		qType := q.QuestionType
		if qType == "" {
			if len(q.Options) >= 2 {
				qType = "mcq"
			} else {
				qType = "subjective"
			}
		}

		userAnswers = append(userAnswers, model.UserAnswer{
			QuestionID:       q.ID,
			QuestionType:     qType,
			SelectedOption:   res.studentSelected,
			AnswerText:       res.studentText,
			Score:            res.score,
			MaxMarks:         res.maxMarks,
			IsCorrect:        res.isCorrect,
			Feedback:         res.feedback,
			KeyPointsCovered: res.keyPointsCovered,
			KeyPointsMissed:  res.keyPointsMissed,
		})

		feedback = append(feedback, dto.QuestionFeedback{
			QuestionID:         q.ID,
			QuestionType:       qType,
			QuestionText:       q.QuestionText,
			Options:            q.Options,
			SelectedOption:     res.studentSelected,
			CorrectOptionIndex: q.CorrectOptionIndex,
			StudentAnswer:      res.studentText,
			ModelAnswer:        q.ModelAnswer,
			Score:              res.score,
			MaxMarks:           res.maxMarks,
			IsCorrect:          res.isCorrect,
			Explanation:        q.Explanation,
			TrapWarning:        q.TrapWarning,
			Feedback:           res.feedback,
			Rubric:             q.Rubric,
			KeyPointsCovered:   res.keyPointsCovered,
			KeyPointsMissed:    res.keyPointsMissed,
		})
	}

	percentage := 0.0
	if totalScore > 0 {
		percentage = (float64(score) / float64(totalScore)) * 100.0
	}

	attemptID := uuid.New().String()
	attempt := &model.QuizAttempt{
		ID:                  attemptID,
		QuizID:              id,
		UserID:              userID,
		Score:               score,
		TotalScore:          totalScore,
		Percentage:          percentage,
		Answers:             userAnswers,
		WeakAreasIdentified: weakTopics,
		TimeTakenSeconds:    req.TimeTakenSeconds,
	}

	// ⚡ Go Concurrency: Persist quiz attempt and update student cognitive profile in parallel
	var updatedProfile *model.CognitiveProfile

	if userID != "" && userID != "guest-user" {
		var wg sync.WaitGroup

		if isValidUUID(id) && s.quizRepo != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = s.quizRepo.SaveAttempt(ctx, attempt)
			}()
		}

		if s.cognitiveRepo != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if p, err := s.cognitiveRepo.RecordQuizResult(ctx, userID, percentage, weakTopics, strongTopics, nil); err == nil {
					updatedProfile = p
				}
			}()
		}

		wg.Wait()
	}

	if updatedProfile == nil {
		updatedProfile = &model.CognitiveProfile{
			StudyStreakDays:   1,
			TotalQuizzesTaken: 1,
			AverageQuizScore:  percentage,
		}
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
	if userID != "" && userID != "guest-user" && s.cognitiveRepo != nil {
		p, err := s.cognitiveRepo.GetByUserID(ctx, userID)
		if err == nil && p != nil {
			return p, nil
		}
	}
	return &model.CognitiveProfile{
		UserID:            userID,
		TargetExam:        "Competitive / University Exam",
		AcademicLevel:     "Undergraduate",
		WeakTopics:        []string{},
		StrongTopics:      []string{},
		ExamTraps:         []string{},
		StudyStreakDays:   1,
		TotalStudyMinutes: 0,
		TotalQuizzesTaken: 0,
		AverageQuizScore:  0.0,
	}, nil
}

func (s *quizService) UpdateCognitiveProfile(ctx context.Context, userID string, req dto.UpdateCognitiveProfileRequest) (*model.CognitiveProfile, error) {
	if userID == "" || userID == "guest-user" || s.cognitiveRepo == nil {
		profile := &model.CognitiveProfile{
			UserID:        userID,
			TargetExam:    "Competitive / University Exam",
			AcademicLevel: "Undergraduate",
			WeakTopics:    []string{},
			StrongTopics:  []string{},
			ExamTraps:     []string{},
		}
		if req.TargetExam != nil {
			profile.TargetExam = *req.TargetExam
		}
		if req.AcademicLevel != nil {
			profile.AcademicLevel = *req.AcademicLevel
		}
		if req.WeakTopics != nil {
			profile.WeakTopics = *req.WeakTopics
		}
		if req.StrongTopics != nil {
			profile.StrongTopics = *req.StrongTopics
		}
		if req.ExamTrapsToWatch != nil {
			profile.ExamTraps = *req.ExamTrapsToWatch
		}
		return profile, nil
	}

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
	if req.WeakTopics != nil {
		profile.WeakTopics = *req.WeakTopics
	}
	if req.StrongTopics != nil {
		profile.StrongTopics = *req.StrongTopics
	}
	if req.ExamTrapsToWatch != nil {
		profile.ExamTraps = *req.ExamTrapsToWatch
	}

	if err := s.cognitiveRepo.Upsert(ctx, profile); err != nil {
		return nil, err
	}
	return profile, nil
}

// ---------------------------------------------------------------------------
// Academic Subjective Answer Evaluation (AI + Heuristic Fallback)
// ---------------------------------------------------------------------------

func (s *quizService) evaluateSubjectiveAnswer(
	ctx context.Context,
	q *model.QuizQuestion,
	studentAnswer string,
) (score int, isCorrect bool, feedback string, covered []string, missed []string) {
	trimmed := strings.TrimSpace(studentAnswer)
	maxMarks := q.MaxMarks
	if maxMarks <= 0 {
		maxMarks = 5
	}

	if trimmed == "" {
		return 0, false, "Question left blank. No marks awarded.", nil, []string{"Candidate provided no response"}
	}

	// 1. Rigorous evaluation using AI Orchestrator
	if s.orchestrator != nil {
		rubricText := strings.Join(q.Rubric, "\n- ")
		if rubricText != "" {
			rubricText = "- " + rubricText
		} else {
			rubricText = fmt.Sprintf("- Accuracy and conceptual completeness (%d marks)\n- Clarity and precision of academic explanation", maxMarks)
		}

		systemPrompt := `You are an elite academic professor and strict examination evaluator.
Grade the student's written response against the official model answer and grading rubric.
Return ONLY valid JSON matching this schema:
{
  "score": <integer from 0 to MAX_MARKS>,
  "is_passing": <boolean, true if score >= 50% of max marks>,
  "feedback": "<2-3 sentence academic evaluation highlighting strong points and explaining mark deductions>",
  "key_points_covered": ["point 1 accurately explained", "point 2 mentioned"],
  "key_points_missed": ["missing derivation / edge case / proof"]
}`
		userPrompt := fmt.Sprintf("QUESTION: %s\nMAX MARKS: %d\nMODEL ANSWER: %s\nGRADING RUBRIC:\n%s\n\nSTUDENT'S SUBMITTED ANSWER:\n%s",
			q.QuestionText, maxMarks, q.ModelAnswer, rubricText, trimmed)

		aiText, err := s.orchestrator.GenerateText(ctx, systemPrompt, userPrompt)
		if err == nil && strings.TrimSpace(aiText) != "" {
			cleaned := cleanJSON(aiText)
			var parsed struct {
				Score            int      `json:"score"`
				IsPassing        bool     `json:"is_passing"`
				Feedback         string   `json:"feedback"`
				KeyPointsCovered []string `json:"key_points_covered"`
				KeyPointsMissed  []string `json:"key_points_missed"`
			}
			if err := json.Unmarshal([]byte(cleaned), &parsed); err == nil {
				if parsed.Score < 0 {
					parsed.Score = 0
				}
				if parsed.Score > maxMarks {
					parsed.Score = maxMarks
				}
				isPassing := parsed.IsPassing
				if parsed.Score >= (maxMarks+1)/2 {
					isPassing = true
				}
				return parsed.Score, isPassing, parsed.Feedback, parsed.KeyPointsCovered, parsed.KeyPointsMissed
			}
		}
	}

	// 2. Resilient Heuristic Fallback (deterministic keyword & concept density grading)
	return evaluateSubjectiveHeuristic(q, trimmed, maxMarks)
}

func evaluateSubjectiveHeuristic(q *model.QuizQuestion, studentAnswer string, maxMarks int) (int, bool, string, []string, []string) {
	trimmed := strings.TrimSpace(studentAnswer)
	words := strings.Fields(trimmed)
	if len(words) < 3 {
		return 0, false, "Answer is too brief to demonstrate conceptual understanding.", nil, []string{"Insufficient explanation"}
	}

	lowerAns := strings.ToLower(trimmed)

	// Extract key technical tokens from ModelAnswer and Rubric
	combinedKeyText := strings.ToLower(q.ModelAnswer + " " + strings.Join(q.Rubric, " "))
	tokens := strings.Fields(combinedKeyText)
	stopWords := map[string]bool{
		"the": true, "and": true, "a": true, "an": true, "in": true, "on": true, "is": true, "of": true,
		"for": true, "to": true, "with": true, "that": true, "this": true, "it": true, "as": true, "by": true,
		"are": true, "was": true, "were": true, "be": true, "been": true, "marks": true, "mark": true,
		"which": true, "from": true, "when": true, "what": true, "how": true, "why": true, "or": true,
	}

	uniqueKeywords := make(map[string]bool)
	for _, tok := range tokens {
		clean := strings.Trim(tok, ".,;:()[]{}\"'`$-/*")
		if len(clean) >= 4 && !stopWords[clean] {
			uniqueKeywords[clean] = true
		}
	}

	matchedKeywords := 0
	var covered []string
	var missed []string

	for kw := range uniqueKeywords {
		if strings.Contains(lowerAns, kw) {
			matchedKeywords++
			if len(covered) < 4 {
				covered = append(covered, fmt.Sprintf("Addressed concept '%s'", kw))
			}
		} else {
			if len(missed) < 3 {
				missed = append(missed, fmt.Sprintf("Missing elaboration on '%s'", kw))
			}
		}
	}

	ratio := 0.0
	if len(uniqueKeywords) > 0 {
		ratio = float64(matchedKeywords) / float64(len(uniqueKeywords))
	} else {
		ratio = float64(len(words)) / 50.0
		if ratio > 1.0 {
			ratio = 1.0
		}
	}

	score := int(float64(maxMarks) * ratio * 1.25)
	if len(words) >= 20 && score == 0 {
		score = 1
	}
	if score > maxMarks {
		score = maxMarks
	}

	isPassing := score >= (maxMarks+1)/2

	var feedback string
	if score == maxMarks {
		feedback = "Outstanding academic response! Core principles, mechanisms, and expected technical terms were thoroughly explained."
	} else if isPassing {
		feedback = fmt.Sprintf("Satisfactory answer (%d/%d marks). Demonstrates solid understanding, though certain technical nuances or edge cases were missed.", score, maxMarks)
	} else {
		feedback = fmt.Sprintf("Partial response (%d/%d marks). Core theoretical tenets and key analytical concepts specified in the rubric were incomplete.", score, maxMarks)
	}

	return score, isPassing, feedback, covered, missed
}

// ---------------------------------------------------------------------------
// Resilient AI JSON Parsing & Domain Fallback Generator
// ---------------------------------------------------------------------------

func cleanJSON(raw string) string {
	s := strings.TrimSpace(raw)
	if idx := strings.Index(s, "```json"); idx != -1 {
		s = s[idx+len("```json"):]
		if end := strings.LastIndex(s, "```"); end != -1 {
			s = s[:end]
		}
	} else if idx := strings.Index(s, "```"); idx != -1 {
		s = s[idx+len("```"):]
		if end := strings.LastIndex(s, "```"); end != -1 {
			s = s[:end]
		}
	}

	firstBrace := strings.IndexAny(s, "{[")
	if firstBrace != -1 {
		var lastBrace int
		if s[firstBrace] == '{' {
			lastBrace = strings.LastIndex(s, "}")
		} else {
			lastBrace = strings.LastIndex(s, "]")
		}
		if lastBrace != -1 && lastBrace > firstBrace {
			s = s[firstBrace : lastBrace+1]
		}
	}

	return strings.TrimSpace(s)
}

// sanitizeJSONEscapes handles unescaped LaTeX backslashes (\frac, \alpha, \mathbf, etc.) and raw control characters
func sanitizeJSONEscapes(s string) string {
	var buf strings.Builder
	buf.Grow(len(s) + 64)
	inString := false
	escaped := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if !inString {
			if ch == '"' {
				inString = true
			}
			buf.WriteByte(ch)
			continue
		}

		// Inside JSON string literal
		if escaped {
			escaped = false
			buf.WriteByte(ch)
			continue
		}

		if ch == '"' {
			inString = false
			buf.WriteByte(ch)
			continue
		}

		if ch == '\\' {
			if i+1 < len(s) {
				next := s[i+1]
				// Known valid JSON escapes
				if next == '"' || next == '\\' || next == '/' || next == 'b' || next == 'f' || next == 'n' || next == 'r' || next == 't' || next == 'u' {
					// Check if it's a common LaTeX command starting with \t or \f
					rest := s[i:]
					if strings.HasPrefix(rest, `\text`) ||
						strings.HasPrefix(rest, `\times`) ||
						strings.HasPrefix(rest, `\theta`) ||
						strings.HasPrefix(rest, `\tau`) ||
						strings.HasPrefix(rest, `\frac`) {
						buf.WriteString(`\\`)
					} else {
						escaped = true
						buf.WriteByte(ch)
					}
				} else {
					// Invalid escape in standard JSON (e.g. \mathbf, \alpha, \sum, \oint, \nabla, \cdot, etc.)
					buf.WriteString(`\\`)
				}
			} else {
				buf.WriteString(`\\`)
			}
			continue
		}

		if ch == '\n' {
			buf.WriteString(`\n`)
			continue
		} else if ch == '\r' {
			buf.WriteString(`\r`)
			continue
		} else if ch == '\t' {
			buf.WriteString(`\t`)
			continue
		}

		buf.WriteByte(ch)
	}
	return buf.String()
}

func parseAIQuizQuestions(raw, defaultTopic, defaultDifficulty string) []*model.QuizQuestion {
	cleaned := cleanJSON(raw)
	if cleaned == "" {
		return nil
	}

	var rawList []map[string]interface{}
	parseSuccess := false

	for _, textToTry := range []string{cleaned, sanitizeJSONEscapes(cleaned)} {
		if strings.HasPrefix(textToTry, "[") {
			if err := json.Unmarshal([]byte(textToTry), &rawList); err == nil && len(rawList) > 0 {
				parseSuccess = true
				break
			}
		} else if strings.HasPrefix(textToTry, "{") {
			var root map[string]interface{}
			if err := json.Unmarshal([]byte(textToTry), &root); err == nil {
				for _, key := range []string{"questions", "quiz", "items", "data", "quiz_questions", "results", "questions_list"} {
					if val, exists := root[key]; exists {
						if slice, ok := val.([]interface{}); ok {
							for _, item := range slice {
								if m, ok := item.(map[string]interface{}); ok {
									rawList = append(rawList, m)
								}
							}
							if len(rawList) > 0 {
								parseSuccess = true
								break
							}
						}
					}
				}
				if parseSuccess {
					break
				}
				// If not found in known keys, check any top-level key that holds a slice of maps
				for _, val := range root {
					if slice, ok := val.([]interface{}); ok {
						for _, item := range slice {
							if m, ok := item.(map[string]interface{}); ok {
								if _, hasQ := m["question_text"]; hasQ {
									rawList = append(rawList, m)
								} else if _, hasQ := m["question"]; hasQ {
									rawList = append(rawList, m)
								}
							}
						}
						if len(rawList) > 0 {
							parseSuccess = true
							break
						}
					}
				}
				if parseSuccess {
					break
				}
			}
		}
	}

	var results []*model.QuizQuestion
	for _, m := range rawList {
		q := extractQuestionFromMap(m, defaultTopic, defaultDifficulty)
		if q != nil && q.QuestionText != "" {
			if q.QuestionType == "subjective" || len(q.Options) >= 2 {
				results = append(results, q)
			}
		}
	}

	return results
}

func extractQuestionFromMap(m map[string]interface{}, defaultTopic, defaultDifficulty string) *model.QuizQuestion {
	qText := ""
	for _, k := range []string{"question_text", "question", "prompt", "problem", "title", "q"} {
		if val, ok := m[k]; ok {
			if s, ok := val.(string); ok && strings.TrimSpace(s) != "" {
				qText = strings.TrimSpace(s)
				break
			}
		}
	}
	if qText == "" {
		return nil
	}

	qType := ""
	if val, ok := m["question_type"]; ok {
		if s, ok := val.(string); ok {
			qType = strings.ToLower(strings.TrimSpace(s))
		}
	}

	var options []string
	for _, k := range []string{"options", "choices", "answers"} {
		if val, ok := m[k]; ok {
			if slice, ok := val.([]interface{}); ok {
				for _, item := range slice {
					if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
						options = append(options, strings.TrimSpace(s))
					} else if str := fmt.Sprintf("%v", item); strings.TrimSpace(str) != "" {
						options = append(options, strings.TrimSpace(str))
					}
				}
				if len(options) > 0 {
					break
				}
			}
		}
	}

	modelAnswer := ""
	for _, k := range []string{"model_answer", "ideal_answer", "solution_key", "sample_answer"} {
		if val, ok := m[k]; ok {
			if s, ok := val.(string); ok {
				modelAnswer = strings.TrimSpace(s)
				break
			}
		}
	}

	var rubric []string
	if val, ok := m["rubric"]; ok {
		if slice, ok := val.([]interface{}); ok {
			for _, item := range slice {
				if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
					rubric = append(rubric, strings.TrimSpace(s))
				}
			}
		} else if s, ok := val.(string); ok && strings.TrimSpace(s) != "" {
			rubric = append(rubric, strings.TrimSpace(s))
		}
	}

	maxMarks := 0
	for _, k := range []string{"max_marks", "marks", "points", "weight"} {
		if val, ok := m[k]; ok {
			switch v := val.(type) {
			case float64:
				maxMarks = int(v)
			case int:
				maxMarks = v
			case string:
				if idx, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
					maxMarks = idx
				}
			}
			if maxMarks > 0 {
				break
			}
		}
	}

	if qType == "" {
		if len(options) >= 2 {
			qType = "mcq"
		} else if modelAnswer != "" || len(rubric) > 0 {
			qType = "subjective"
		} else {
			qType = "mcq"
		}
	}

	correctIdx := 0
	if qType == "subjective" {
		correctIdx = -1
		options = []string{}
		if maxMarks <= 0 {
			if defaultDifficulty == "hard" {
				maxMarks = 10
			} else {
				maxMarks = 5
			}
		}
		if modelAnswer == "" {
			if val, ok := m["explanation"]; ok {
				if s, ok := val.(string); ok {
					modelAnswer = strings.TrimSpace(s)
				}
			}
		}
		if len(rubric) == 0 {
			rubric = []string{
				fmt.Sprintf("Conceptual accuracy and governing axioms (%d marks)", (maxMarks+1)/2),
				fmt.Sprintf("Analytical reasoning, derivations, and edge cases (%d marks)", maxMarks-(maxMarks+1)/2),
			}
		}
	} else {
		if len(options) < 2 {
			return nil
		}
		if maxMarks <= 0 {
			maxMarks = 1
		}
		foundCorrect := false
		for _, k := range []string{"correct_option_index", "correct_answer_index", "answer_index", "correct_index", "answer"} {
			if val, ok := m[k]; ok {
				switch v := val.(type) {
				case float64:
					correctIdx = int(v)
					foundCorrect = true
				case int:
					correctIdx = v
					foundCorrect = true
				case string:
					v = strings.TrimSpace(strings.ToUpper(v))
					if len(v) == 1 && v[0] >= 'A' && v[0] <= 'D' {
						correctIdx = int(v[0] - 'A')
						foundCorrect = true
					} else if idx, err := strconv.Atoi(v); err == nil {
						correctIdx = idx
						foundCorrect = true
					}
				}
				if foundCorrect {
					break
				}
			}
		}
		if correctIdx < 0 || correctIdx >= len(options) {
			correctIdx = 0
		}
	}

	explanation := ""
	for _, k := range []string{"explanation", "solution", "rationale", "reasoning"} {
		if val, ok := m[k]; ok {
			if s, ok := val.(string); ok {
				explanation = strings.TrimSpace(s)
				break
			}
		}
	}

	trapWarning := ""
	for _, k := range []string{"trap_warning", "trap", "misconception", "examiner_trap", "warning"} {
		if val, ok := m[k]; ok {
			if s, ok := val.(string); ok {
				trapWarning = strings.TrimSpace(s)
				break
			}
		}
	}

	topic := defaultTopic
	for _, k := range []string{"topic", "subtopic", "concept"} {
		if val, ok := m[k]; ok {
			if s, ok := val.(string); ok && strings.TrimSpace(s) != "" {
				topic = strings.TrimSpace(s)
				break
			}
		}
	}

	diff := defaultDifficulty
	for _, k := range []string{"difficulty", "level"} {
		if val, ok := m[k]; ok {
			if s, ok := val.(string); ok && strings.TrimSpace(s) != "" {
				diff = strings.TrimSpace(s)
				break
			}
		}
	}

	return &model.QuizQuestion{
		QuestionText:       qText,
		QuestionType:       qType,
		Options:            options,
		CorrectOptionIndex: correctIdx,
		ModelAnswer:        modelAnswer,
		Rubric:             rubric,
		MaxMarks:           maxMarks,
		Explanation:        explanation,
		TrapWarning:        trapWarning,
		Difficulty:         diff,
		Topic:              topic,
	}
}

type domainQuizTemplate struct {
	qFmt    string
	opts    []string
	correct int
	explFmt string
	trapFmt string
	diff    string
}

type domainSubjectiveTemplate struct {
	qFmt        string
	modelAnswer string
	rubric      []string
	explFmt     string
	trapFmt     string
	diff        string
	maxMarks    int
}

func generateDomainQuestions(topic, difficulty, testType string, count int) []*model.QuizQuestion {
	lower := strings.ToLower(topic)

	// 1. Computer Science & Programming
	isCS := strings.Contains(lower, "python") || strings.Contains(lower, "java") ||
		strings.Contains(lower, "c++") || strings.Contains(lower, "c#") ||
		strings.Contains(lower, "go") || strings.Contains(lower, "golang") ||
		strings.Contains(lower, "javascript") || strings.Contains(lower, "js") ||
		strings.Contains(lower, "typescript") || strings.Contains(lower, "ts") ||
		strings.Contains(lower, "sql") || strings.Contains(lower, "database") ||
		strings.Contains(lower, "dbms") || strings.Contains(lower, "html") ||
		strings.Contains(lower, "css") || strings.Contains(lower, "react") ||
		strings.Contains(lower, "flutter") || strings.Contains(lower, "dart") ||
		strings.Contains(lower, "algorithm") || strings.Contains(lower, "dsa") ||
		strings.Contains(lower, "array") || strings.Contains(lower, "pointer") ||
		strings.Contains(lower, "oop") || strings.Contains(lower, "recursion") ||
		strings.Contains(lower, "stack") || strings.Contains(lower, "queue") ||
		strings.Contains(lower, "graph") || strings.Contains(lower, "tree") ||
		strings.Contains(lower, "linux") || strings.Contains(lower, "git") ||
		strings.Contains(lower, "api") || strings.Contains(lower, "docker") ||
		strings.Contains(lower, "backend") || strings.Contains(lower, "code")

	// 2. Biology & Life Sciences
	isBio := strings.Contains(lower, "bio") || strings.Contains(lower, "cell") ||
		strings.Contains(lower, "dna") || strings.Contains(lower, "rna") ||
		strings.Contains(lower, "gene") || strings.Contains(lower, "photosynthesis") ||
		strings.Contains(lower, "respiration") || strings.Contains(lower, "enzyme") ||
		strings.Contains(lower, "protein") || strings.Contains(lower, "organ") ||
		strings.Contains(lower, "heart") || strings.Contains(lower, "brain") ||
		strings.Contains(lower, "blood") || strings.Contains(lower, "bacteria") ||
		strings.Contains(lower, "virus") || strings.Contains(lower, "plant") ||
		strings.Contains(lower, "animal") || strings.Contains(lower, "ecology")

	var mcqTemplates []domainQuizTemplate
	var subjTemplates []domainSubjectiveTemplate

	if isCS {
		mcqTemplates = []domainQuizTemplate{
			{
				qFmt: "In **%s**, what is the primary benefit of modular architectural design and function decomposition?",
				opts: []string{
					"A) Enhances code reusability, testability, and separation of concerns",
					"B) Dramatically increases runtime memory footprint and compilation delay",
					"C) Bypasses variable scoping to permit uncontrolled global state mutation",
					"D) Eliminates the requirement for defensive input validation and error handling",
				},
				correct: 0,
				explFmt: "Modular decomposition in %s isolates independent responsibilities into cohesive units, enhancing maintainability and deterministic unit testability.",
				trapFmt: "Avoid monolithic functions that blend presentation, business logic, and state mutation together.",
				diff:    "easy",
			},
			{
				qFmt: "When designing data structures or algorithms for **%s**, why is asymptotic complexity analysis ($O(N)$ vs $O(N^2)$) critical?",
				opts: []string{
					"A) To ensure predictable scalability and prevent severe throughput degradation under high load",
					"B) Because compilers automatically convert all quadratic algorithms to constant time",
					"C) To eliminate the necessity of managing memory pointers and heap allocations",
					"D) It only applies to compiled languages and has no impact on interpreted scripts",
				},
				correct: 0,
				explFmt: "Asymptotic complexity analysis in %s ensures predictable execution time as input size scales, preventing system starvation and latency spikes.",
				trapFmt: "Premature micro-optimization is unproductive, but ignoring asymptotic quadratic scaling causes severe production bottlenecks.",
				diff:    "medium",
			},
			{
				qFmt: "In concurrent or asynchronous environments involving **%s**, how are data races and state corruption prevented?",
				opts: []string{
					"A) Through synchronization primitives (mutexes, locks) or message-passing channels and immutability",
					"B) By permitting arbitrary concurrent writes without memory barriers",
					"C) By disabling thread scheduling at the operating system kernel level",
					"D) By simply assigning global static variables across worker threads",
				},
				correct: 0,
				explFmt: "Safe concurrency in %s requires synchronized access or immutable structures so multiple threads do not concurrently mutate shared state.",
				trapFmt: "Race conditions are notoriously non-deterministic; they rarely show up during simple local testing without race detector tooling.",
				diff:    "hard",
			},
		}

		subjTemplates = []domainSubjectiveTemplate{
			{
				qFmt:        "Explain the core architectural principles of **%s**. Discuss how state mutation is governed and compare its concurrency or execution model with traditional blocking architectures.",
				modelAnswer: "Core architecture in " + topic + " emphasizes separation of concerns, deterministic data flow, and controlled state mutations. In concurrent environments, race conditions are mitigated via synchronization primitives (mutexes/locks) or message channels, preventing dirty reads and torn writes. Context-switching and non-blocking I/O event loops minimize idle thread overhead.",
				rubric: []string{
					"2 marks: Architectural breakdown and component separation",
					"2 marks: State mutation control, thread safety, and concurrency mechanisms",
					"1 mark: Practical trade-offs and performance implications under high throughput",
				},
				explFmt:  "A rigorous response must contrast blocking I/O with non-blocking event-driven paradigms and identify synchronization primitives.",
				trapFmt:  "Do not treat asynchronous programming as automatically multi-threaded; single-threaded event loops (e.g., Node.js/Dart) differ fundamentally from thread pools.",
				diff:     "medium",
				maxMarks: 5,
			},
			{
				qFmt:        "Analyze the time and space complexity trade-offs encountered when scaling algorithms in **%s**. How do memory allocation overheads (heap vs stack) and CPU cache locality influence overall real-world latency?",
				modelAnswer: "Asymptotic complexity $O(N)$ predicts scaling growth, but real-world latency is heavily dominated by CPU hardware architecture. Stack allocations are contiguous, cache-friendly, and free from garbage collection overhead, whereas frequent heap allocations induce fragmentation and cache misses. Optimal systems utilize contiguous array structures to exploit L1/L2 cache prefetching.",
				rubric: []string{
					"2 marks: Asymptotic complexity derivations ($O(N)$ vs $O(N^2)$)",
					"2 marks: Heap vs Stack memory mechanics and allocation overheads",
					"1 mark: CPU cache line alignment and hardware prefetching dynamics",
				},
				explFmt:  "Theoretical complexity must be coupled with hardware realities including memory hierarchy and cache locality.",
				trapFmt:  "Confusing worst-case asymptotic bounds with constant-factor cache miss penalties.",
				diff:     "hard",
				maxMarks: 5,
			},
		}
	} else if isBio {
		mcqTemplates = []domainQuizTemplate{
			{
				qFmt: "What is the primary governing function and biological significance of **%s** in living organisms?",
				opts: []string{
					"A) Facilitating specialized metabolic pathways and maintaining dynamic homeostatic equilibrium",
					"B) Inducing random cellular breakdown without conservation of energy or mass",
					"C) Maintaining a completely static state devoid of biochemical interaction",
					"D) Eliminating all cellular transport and membrane permeability",
				},
				correct: 0,
				explFmt: "In %s, precise molecular signaling and enzymatic catalysis regulate internal conditions to preserve cellular homeostasis.",
				trapFmt: "Do not confuse positive feedback cascades with standard negative homeostatic regulation.",
				diff:    "easy",
			},
			{
				qFmt: "Which factor primarily limits the rate of biochemical transport or transformation in **%s**?",
				opts: []string{
					"A) Substrate saturation and the activation energy barrier of the slowest rate-limiting step",
					"B) The absolute volume of extracellular fluid independent of concentration gradients",
					"C) The total number of non-functional structural lipids in the organism",
					"D) Spontaneous instantaneous chemical diffusion with zero thermal resistance",
				},
				correct: 0,
				explFmt: "Flux through metabolic pathways in %s is throttled by the bottleneck step possessing the highest activation energy barrier.",
				trapFmt: "Enzymatic catalysts accelerate rate constants by lowering activation energy without altering net equilibrium constants.",
				diff:    "hard",
			},
		}

		subjTemplates = []domainSubjectiveTemplate{
			{
				qFmt:        "Explain the biochemical pathways and homeostatic regulation mechanisms governing **%s**. Detail the role of feedback inhibition and enzyme kinetics in maintaining cellular balance.",
				modelAnswer: "Regulation of " + topic + " relies on allosteric enzyme kinetics and negative feedback loops. When end-product concentrations exceed threshold levels, allosteric inhibitors bind regulatory enzyme domains, altering active site conformation ($V_{max}$ or $K_m$) to conserve energetic substrates and prevent metabolic toxicity.",
				rubric: []string{
					"2 marks: Biochemical pathway description and key catalytic steps",
					"2 marks: Negative feedback loops and allosteric kinetics ($V_{max}$ and $K_m$)",
					"1 mark: Physiological homeostatic significance in cellular survival",
				},
				explFmt:  "Provide clear molecular explanations of negative feedback and Michaelis-Menten kinetic modulation.",
				trapFmt:  "Failing to distinguish between competitive inhibition (affects $K_m$) and non-competitive allosteric inhibition (affects $V_{max}$).",
				diff:     "medium",
				maxMarks: 5,
			},
		}
	} else {
		mcqTemplates = []domainQuizTemplate{
			{
				qFmt: "What constitutes the foundational principle and governing core of **%s**?",
				opts: []string{
					"A) Structured methodologies, validated relationships, and predictable outcomes derived from first principles",
					"B) Arbitrary memorization of disconnected facts without underlying conceptual reasoning",
					"C) Ignoring boundary conditions and fundamental operational constraints",
					"D) Random unsystematic variance devoid of analytical rules",
				},
				correct: 0,
				explFmt: "Mastery of %s requires understanding governing axioms, operational mechanisms, and verified cause-and-effect relationships.",
				trapFmt: "Beware of confusing surface-level symptoms with root governing principles.",
				diff:    "easy",
			},
			{
				qFmt: "How does the theoretical framework of **%s** evolve when advancing from basic models to advanced real-world systems?",
				opts: []string{
					"A) Incorporates higher-order coupling, non-linear dynamics, and real-world boundary constraints",
					"B) Completely discards all elementary laws in favor of random approximations",
					"C) Assumes that all system variables remain strictly static and constant over time",
					"D) Reduces all quantitative measures to purely subjective impressions",
				},
				correct: 0,
				explFmt: "Advanced analysis in %s accounts for perturbations, feedback loops, and secondary effects that are neglected in simplified models.",
				trapFmt: "Linear first-order approximations break down rapidly outside small-signal perturbation limits.",
				diff:    "hard",
			},
		}

		subjTemplates = []domainSubjectiveTemplate{
			{
				qFmt:        "From first principles, derive and critically evaluate the governing theoretical framework of **%s**. What are the primary boundary assumptions, and how does the model behave when those assumptions are relaxed?",
				modelAnswer: "The analytical framework of " + topic + " is formulated upon fundamental conservation laws and boundary axioms. Under idealized conditions, first-order linear interactions dominate. However, real-world systems exhibit higher-order coupling, non-linear dissipation, and boundary edge effects that necessitate perturbation corrections and numerical validations.",
				rubric: []string{
					"2 marks: First-principles derivation and conceptual axioms",
					"2 marks: Boundary condition validation and breakdown regimes",
					"1 mark: Practical synthesis and real-world empirical adjustments",
				},
				explFmt:  "Explain both the idealized model and the mathematical/practical implications when operational constraints fail.",
				trapFmt:  "Applying idealized formulas in domains where non-linear parasitic effects dominate.",
				diff:     "medium",
				maxMarks: 5,
			},
		}
	}

	var results []*model.QuizQuestion

	for i := 0; i < count; i++ {
		isSubjectiveThis := false
		if testType == "subjective" {
			isSubjectiveThis = true
		} else if testType == "mixed" {
			// In mixed mode: alternating or ~40% subjective
			if i%2 == 1 && len(subjTemplates) > 0 {
				isSubjectiveThis = true
			}
		}

		if isSubjectiveThis && len(subjTemplates) > 0 {
			st := subjTemplates[i%len(subjTemplates)]
			diff := difficulty
			if diff == "adaptive" || diff == "" {
				diff = st.diff
			}
			maxM := st.maxMarks
			if diff == "hard" {
				maxM = 10
			}
			results = append(results, &model.QuizQuestion{
				ID:                 uuid.New().String(),
				QuestionType:       "subjective",
				QuestionText:       fmt.Sprintf(st.qFmt, topic),
				Options:            []string{},
				CorrectOptionIndex: -1,
				ModelAnswer:        st.modelAnswer,
				Rubric:             st.rubric,
				MaxMarks:           maxM,
				Explanation:        fmt.Sprintf(st.explFmt, topic),
				TrapWarning:        st.trapFmt,
				Difficulty:         diff,
				Topic:              topic,
				CreatedAt:          time.Now(),
			})
		} else {
			mt := mcqTemplates[i%len(mcqTemplates)]
			diff := difficulty
			if diff == "adaptive" || diff == "" {
				diff = mt.diff
			}
			results = append(results, &model.QuizQuestion{
				ID:                 uuid.New().String(),
				QuestionType:       "mcq",
				QuestionText:       fmt.Sprintf(mt.qFmt, topic),
				Options:            mt.opts,
				CorrectOptionIndex: mt.correct,
				MaxMarks:           1,
				Explanation:        fmt.Sprintf(mt.explFmt, topic),
				TrapWarning:        mt.trapFmt,
				Difficulty:         diff,
				Topic:              topic,
				CreatedAt:          time.Now(),
			})
		}
	}

	return results
}
