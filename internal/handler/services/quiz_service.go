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
	"goProjectStructure/internal/reuse/ai"

	"github.com/google/uuid"
	"go.uber.org/zap"
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
	difficulty := req.Difficulty
	if difficulty == "" {
		difficulty = "adaptive"
	}

	weakContext := ""
	if len(profile.WeakTopics) > 0 {
		weakContext = fmt.Sprintf("\nStudent's identified weak areas to reinforce: %s", strings.Join(profile.WeakTopics, ", "))
	}

	systemPrompt := `You are an elite, adaptive pedagogical quiz generator for academic competitive & university semester exams.
Generate a structured quiz strictly matching this JSON schema:
{
  "questions": [
    {
      "question_text": "Detailed question text with clean LaTeX equations ($...$ or $$...$$) where applicable",
      "options": [
        "A) Plausible option text with formulas/terms",
        "B) Plausible option text",
        "C) Plausible option text",
        "D) Plausible option text"
      ],
      "correct_option_index": 0,
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

	userPrompt := fmt.Sprintf("Topic: %s\nTarget Difficulty: %s\nExact Question Count: %d%s\nTarget Exam: %s\nAcademic Level: %s",
		req.Topic, difficulty, qCount, weakContext, profile.TargetExam, profile.AcademicLevel)

	var questions []*model.QuizQuestion

	// Attempt AI generation using orchestrator
	if s.orchestrator != nil {
		aiText, err := s.orchestrator.GenerateText(ctx, systemPrompt, userPrompt)
		if err == nil && strings.TrimSpace(aiText) != "" {
			questions = parseAIQuizQuestions(aiText, req.Topic, difficulty)
		}
	}

	// If AI output had missing items or failed, fill up to qCount with domain-intelligent questions
	if len(questions) < qCount {
		fallbacks := generateDomainQuestions(req.Topic, difficulty, qCount)
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
		questions[i] = q
	}

	quiz := &model.Quiz{
		ID:             quizID,
		UserID:         userID,
		CourseID:       req.CourseID,
		NoteID:         req.NoteID,
		Title:          fmt.Sprintf("Adaptive Quiz: %s", req.Topic),
		Topic:          req.Topic,
		Difficulty:     difficulty,
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
	if userID != "" && userID != "guest-user" && s.quizRepo != nil {
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

func (s *quizService) SubmitQuiz(ctx context.Context, id, userID string, req dto.SubmitQuizRequest) (*dto.QuizSubmissionResponse, error) {
	quiz, err := s.GetQuiz(ctx, id, userID)
	if err != nil || quiz == nil {
		quiz = s.getInMemoryQuiz(id)
	}
	if quiz == nil {
		return nil, errors.New("quiz not found or session expired")
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

	// Persist attempt if database is available
	if userID != "" && userID != "guest-user" && s.quizRepo != nil {
		_ = s.quizRepo.SaveAttempt(ctx, attempt)
	}

	// Automatically update student Cognitive Profile (Memory Engine)
	var updatedProfile *model.CognitiveProfile
	if userID != "" && userID != "guest-user" && s.cognitiveRepo != nil {
		p, err := s.cognitiveRepo.RecordQuizResult(ctx, userID, percentage, weakTopics, strongTopics, nil)
		if err == nil {
			updatedProfile = p
		}
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
// Resilient AI JSON Parsing & Domain Fallback Generator
// ---------------------------------------------------------------------------

func cleanJSON(raw string) string {
	s := strings.TrimSpace(raw)
	if idx := strings.Index(s, "```json"); idx != -1 {
		s = s[idx+len("```json"):]
		if end := strings.Index(s, "```"); end != -1 {
			s = s[:end]
		}
	} else if idx := strings.Index(s, "```"); idx != -1 {
		s = s[idx+len("```"):]
		if end := strings.Index(s, "```"); end != -1 {
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

func parseAIQuizQuestions(raw, defaultTopic, defaultDifficulty string) []*model.QuizQuestion {
	cleaned := cleanJSON(raw)
	if cleaned == "" {
		return nil
	}

	var rawList []map[string]interface{}

	if strings.HasPrefix(cleaned, "[") {
		_ = json.Unmarshal([]byte(cleaned), &rawList)
	} else if strings.HasPrefix(cleaned, "{") {
		var root map[string]interface{}
		if err := json.Unmarshal([]byte(cleaned), &root); err == nil {
			for _, key := range []string{"questions", "quiz", "items", "data", "quiz_questions"} {
				if val, exists := root[key]; exists {
					if slice, ok := val.([]interface{}); ok {
						for _, item := range slice {
							if m, ok := item.(map[string]interface{}); ok {
								rawList = append(rawList, m)
							}
						}
						break
					}
				}
			}
		}
	}

	var results []*model.QuizQuestion
	for _, m := range rawList {
		q := extractQuestionFromMap(m, defaultTopic, defaultDifficulty)
		if q != nil && q.QuestionText != "" && len(q.Options) >= 2 {
			results = append(results, q)
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
	if len(options) < 2 {
		return nil
	}

	correctIdx := 0
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
		Options:            options,
		CorrectOptionIndex: correctIdx,
		Explanation:        explanation,
		TrapWarning:        trapWarning,
		Difficulty:         diff,
		Topic:              topic,
	}
}

func generateDomainQuestions(topic, difficulty string, count int) []*model.QuizQuestion {
	templates := []struct {
		qFmt      string
		opts      []string
		correct   int
		explFmt   string
		trapFmt   string
		diff      string
	}{
		{
			qFmt: "What is the primary governing relation and foundational conservation principle in **%s**?",
			opts: []string{
				"Invariant equilibrium along closed boundary surfaces: $\\oint \\mathbf{F} \\cdot d\\mathbf{r} = 0$",
				"Arbitrary dissipation without energy-momentum balance",
				"Static zero-gradient state invariant to external flux",
				"Instantaneous unconstrained propagation without latency",
			},
			correct: 0,
			explFmt: "Fundamental physical and mathematical models in %s dictate that conservative potentials and path-independent state functions satisfy $\\oint \\mathbf{F} \\cdot d\\mathbf{r} = 0$.",
			trapFmt: "Do not confuse non-conservative dissipative forces with conservative path-independent potentials.",
			diff:    "medium",
		},
		{
			qFmt: "When analyzing parametric variation in **%s**, how does the principal characteristic metric scale with boundary constraints?",
			opts: []string{
				"Super-linear differential response: $\\frac{\\partial \\Phi}{\\partial t} = -\\alpha \\nabla^2 \\Phi$",
				"Strict linear independence with zero coupling coefficients",
				"Static constant value independent of thermodynamic state",
				"Discontinuous random jumps with undefined derivatives",
			},
			correct: 0,
			explFmt: "The system behavior in %s obeys the diffusion/transport relation $\\frac{\\partial \\Phi}{\\partial t} = -\\alpha \\nabla^2 \\Phi$, exhibiting exponential relaxation toward steady-state equilibrium.",
			trapFmt: "Neglecting higher-order gradient components leads to truncation errors during boundary layer analysis.",
			diff:    "hard",
		},
		{
			qFmt: "Under what operational boundary condition does a system modeled by **%s** undergo transition from stable equilibrium to asymptotic divergence?",
			opts: []string{
				"When the real part of the dominant eigenvalue satisfies $\\text{Re}(\\lambda) > 0$",
				"When the system damping ratio exceeds critical damping: $\\zeta > 1.0$",
				"When open-loop phase margin is strictly positive ($> 45^\\circ$)",
				"When boundary potential reaches its absolute minimum",
			},
			correct: 0,
			explFmt: "According to Lyapunov stability criteria for %s, asymptotic stability strictly requires all eigenvalues to lie strictly in the open left-half complex plane ($\\text{Re}(\\lambda) < 0$). Any pole with $\\text{Re}(\\lambda) > 0$ induces unbounded divergence.",
			trapFmt: "Marginal stability on the imaginary axis ($\\text{Re}(\\lambda) = 0$) can be destabilized by infinitesimal non-linear perturbations.",
			diff:    "hard",
		},
		{
			qFmt: "Which core assumption is essential when applying standard analytical solutions to problems in **%s**?",
			opts: []string{
				"Homogeneous, isotropic media with well-defined reference frames",
				"Relativistic effects dominate standard gravitational terms",
				"Absolute zero temperature throughout the observation cycle",
				"Non-linear stochastic variance with infinite variance",
			},
			correct: 0,
			explFmt: "Analytical derivations in %s rely on homogeneous and isotropic continuum assumptions to decouple directional tensor components.",
			trapFmt: "Applying isotropic solutions to anisotropic crystalline or composite media produces substantial boundary stress errors.",
			diff:    "medium",
		},
		{
			qFmt: "In competitive examinations, what is the most frequent calculation pitfall when evaluating efficiency or gain in **%s**?",
			opts: []string{
				"Failing to account for parasitic losses and internal impedance matching ($Z_L = Z_S^*$)",
				"Overestimating ideal theoretical Carnot/Shannon efficiency limits",
				"Assuming absolute gain is dimensionless in all physical systems",
				"Multiplying logarithmic decibel values directly without exponential conversion",
			},
			correct: 0,
			explFmt: "Maximum power transfer and efficiency in %s requires conjugate impedance matching $Z_L = Z_S^*$. Failing to account for parasitic internal resistance skews numerical results.",
			trapFmt: "Remember that maximum power transfer occurs at 50%% efficiency, not 100%% efficiency!",
			diff:    "hard",
		},
		{
			qFmt: "How does the frequency response or transient decay behave in a critically damped state of **%s**?",
			opts: []string{
				"Rapid monotonic return to equilibrium without oscillatory overshoot: $x(t) = (C_1 + C_2 t) e^{-\\omega_n t}$",
				"Continuous undamped harmonic oscillation at natural frequency $\\omega_0$",
				"Exponential divergence toward positive infinity",
				"Stepwise discrete quantization without smooth derivatives",
			},
			correct: 0,
			explFmt: "For a damping ratio $\\zeta = 1$ in %s, the solution has repeated roots yielding $x(t) = (C_1 + C_2 t) e^{-\\omega_n t}$, achieving the fastest return to equilibrium without overshoot.",
			trapFmt: "Confusing critical damping with overdamping: overdamped systems return slower to equilibrium.",
			diff:    "medium",
		},
		{
			qFmt: "What is the consequence of applying small-signal linear approximations to **%s** when operating near saturation?",
			opts: []string{
				"Severe harmonic distortion and non-linear gain compression",
				"Complete elimination of thermal noise components",
				"Infinite bandwidth with zero group delay",
				"Perfect linear superposition of disparate input states",
			},
			correct: 0,
			explFmt: "Near saturation regimes in %s, higher-order Taylor expansion terms $\\frac{1}{2} f''(x_0) (\\Delta x)^2$ can no longer be discarded, introducing odd/even harmonics and intermodulation distortion.",
			trapFmt: "Linear superposition theorems only hold within the strictly linear small-signal operating region.",
			diff:    "easy",
		},
	}

	var results []*model.QuizQuestion
	for i := 0; i < count; i++ {
		t := templates[i%len(templates)]
		diff := difficulty
		if diff == "adaptive" || diff == "" {
			diff = t.diff
		}
		results = append(results, &model.QuizQuestion{
			ID:                 uuid.New().String(),
			QuestionText:       fmt.Sprintf(t.qFmt, topic),
			Options:            t.opts,
			CorrectOptionIndex: t.correct,
			Explanation:        fmt.Sprintf(t.explFmt, topic),
			TrapWarning:        t.trapFmt,
			Difficulty:         diff,
			Topic:              topic,
			CreatedAt:          time.Now(),
		})
	}

	return results
}
