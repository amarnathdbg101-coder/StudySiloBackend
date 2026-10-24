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
	aiLanguage := middleware.GetAILanguage(ctx)
	systemPrompt += ai.BuildLanguageInstruction(aiLanguage)

	userPrompt := fmt.Sprintf("Topic: %s\nTarget Difficulty: %s\nExact Question Count: %d%s\nTarget Exam: %s\nAcademic Level: %s",
		req.Topic, difficulty, qCount, weakContext, profile.TargetExam, profile.AcademicLevel)

	var questions []*model.QuizQuestion

	provider := strings.TrimSpace(req.Provider)
	if provider == "" {
		provider = "google"
	}
	modelName := strings.TrimSpace(req.Model)
	if modelName == "" {
		modelName = "gemini-3.5-flash-lite"
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
				zap.Int("questions_parsed", len(questions)),
			)
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
		q.QuestionText = ai.SanitizeLatex(q.QuestionText)
		for optIdx := range q.Options {
			q.Options[optIdx] = ai.SanitizeLatex(q.Options[optIdx])
		}
		q.Explanation = ai.SanitizeLatex(q.Explanation)
		q.TrapWarning = ai.SanitizeLatex(q.TrapWarning)
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
	if userID == "" {
		userID = "guest-user"
	}
	if s.cognitiveRepo != nil {
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
	if userID == "" {
		userID = "guest-user"
	}
	if s.cognitiveRepo == nil {
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
		profile = &model.CognitiveProfile{
			UserID:        userID,
			TargetExam:    "Competitive / University Exam",
			AcademicLevel: "Undergraduate",
			WeakTopics:    []string{},
			StrongTopics:  []string{},
			ExamTraps:     []string{},
		}
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

type domainQuizTemplate struct {
	qFmt    string
	opts    []string
	correct int
	explFmt string
	trapFmt string
	diff    string
}

func generateDomainQuestions(topic, difficulty string, count int) []*model.QuizQuestion {
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

	var templates []domainQuizTemplate

	if isCS {
		templates = []domainQuizTemplate{
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
				qFmt: "What is a frequent cause of production crashes or memory corruption when working with **%s**?",
				opts: []string{
					"A) Failing to validate null/nil references, boundary off-by-one indices, and unclosed resources",
					"B) Utilizing strongly typed variables and deterministic compile-time interfaces",
					"C) Structuring code into pure idempotent functions without side-effects",
					"D) Enabling compiler optimization flags during the production build cycle",
				},
				correct: 0,
				explFmt: "In %s, unhandled edge conditions such as nil dereferences, unclosed descriptors, or concurrent data races produce unpredictable crashes.",
				trapFmt: "Always release allocated resources deterministically using cleanup patterns (e.g. defer, using, or finally blocks).",
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
			{
				qFmt: "Which testing methodology provides the highest confidence when deploying complex logic in **%s**?",
				opts: []string{
					"A) Automated table-driven unit and integration tests covering boundary conditions and error pathways",
					"B) A single manual run verifying only the happy path on a developer workstation",
					"C) Disabling automated checks and testing directly on end users in production",
					"D) Relying exclusively on syntax highlighting and static lint formatting",
				},
				correct: 0,
				explFmt: "Comprehensive test suites for %s assert both valid and erroneous inputs to guarantee resilience against edge-case anomalies.",
				trapFmt: "High code line coverage does not ensure correctness if boundary edge-cases and error branches are omitted.",
				diff:    "hard",
			},
		}
	} else if isBio {
		templates = []domainQuizTemplate{
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
				qFmt: "How do variations in pH and ambient temperature typically influence biological processes in **%s**?",
				opts: []string{
					"A) By altering protein tertiary conformation and shifting enzymatic catalytic reaction rates",
					"B) They have zero influence on molecular folding or reaction kinetics",
					"C) By permanently reversing the thermodynamic direction of cellular respiration",
					"D) By instantaneously eliminating genetic nucleotide sequences",
				},
				correct: 0,
				explFmt: "Enzymatic and cellular machinery in %s operates within narrow optimal pH and temperature ranges; deviations cause denaturation.",
				trapFmt: "Denaturation disrupts secondary and tertiary folding without breaking the primary covalent peptide bonds.",
				diff:    "medium",
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
	} else {
		// Academic General & Conceptual Mastery (Applicable to Science, Humanities, Math, and Professional Exams)
		templates = []domainQuizTemplate{
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
				qFmt: "When applying principles of **%s** to complex real-world scenarios, which practice delivers optimal results?",
				opts: []string{
					"A) Systematic decomposition into primary variables, validating edge cases, and structured verification",
					"B) Making broad unsubstantiated assumptions without validating boundary criteria",
					"C) Treating interdependent system variables as completely disconnected and isolated",
					"D) Relying exclusively on anecdotal observations rather than verified standards",
				},
				correct: 0,
				explFmt: "Rigorous execution in %s requires decomposing multi-factor problems into cohesive units and validating operational assumptions.",
				trapFmt: "Do not extrapolate simplified ideal conditions into regimes where non-ideal parasitic factors dominate.",
				diff:    "medium",
			},
			{
				qFmt: "In competitive and academic examinations, what is the most frequent misconception regarding **%s**?",
				opts: []string{
					"A) Misapplying generalized formulas or rules outside their validated operational domain",
					"B) Verifying dimensional consistency and physical boundary limits before finalizing answers",
					"C) Step-by-step mathematical or logical validation from established definitions",
					"D) Assuming that real-world problems require structured analytical solutions",
				},
				correct: 0,
				explFmt: "Students frequently lose marks in %s by applying standard formulas to problems where prerequisite boundary conditions are not satisfied.",
				trapFmt: "Always verify that all foundational criteria are met before invoking specialized theorems or shortcuts.",
				diff:    "medium",
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
			{
				qFmt: "Which evaluative metric confirms true conceptual mastery of **%s** rather than surface-level memorization?",
				opts: []string{
					"A) Ability to derive solutions from first principles and accurately diagnose subtle edge-case traps",
					"B) Rote memorization of final formulas without comprehension of their origins",
					"C) Selecting answer options solely based on paragraph length or unfamiliar vocabulary",
					"D) Disregarding units, constraints, and sanity checks on final calculations",
				},
				correct: 0,
				explFmt: "True expertise in %s is demonstrated through first-principles reasoning, diagnostic rigor, and identifying examiner traps.",
				trapFmt: "Always execute a sanity check on magnitude, sign, and boundary limits before submitting your final response.",
				diff:    "hard",
			},
		}
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
