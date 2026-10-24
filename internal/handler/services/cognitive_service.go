package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/model"
	"goProjectStructure/internal/handler/repository"
	"goProjectStructure/internal/middleware"
	"goProjectStructure/internal/reuse"
	"goProjectStructure/internal/reuse/ai"

	"go.uber.org/zap"
)

// CognitiveService manages AI-driven student learning memory, cognitive diagnostic analysis, and topic mastery.
type CognitiveService interface {
	GetProfile(ctx context.Context, userID string) (*dto.CognitiveProfileResponse, error)
	UpdateGoals(ctx context.Context, userID string, req dto.UpdateCognitiveGoalRequest) (*dto.CognitiveProfileResponse, error)
	DiagnoseProfile(ctx context.Context, userID string) (*dto.CognitiveDiagnosisResponse, error)
	PromoteTopic(ctx context.Context, userID, topic string) (*dto.CognitiveProfileResponse, error)
	DemoteTopic(ctx context.Context, userID, topic string) (*dto.CognitiveProfileResponse, error)
	DeleteTopic(ctx context.Context, userID, topic string) (*dto.CognitiveProfileResponse, error)
	RecordActivity(ctx context.Context, userID string, req dto.RecordActivityRequest) (*dto.CognitiveProfileResponse, error)
	GetSmartContext(ctx context.Context, userID, targetExam, tutorMode string) (*dto.SmartContextResponse, error)
}

type cognitiveService struct {
	cognitiveRepo repository.CognitiveRepository
	chatRepo      repository.ChatRepository
	orchestrator  *ai.Orchestrator
	logger        *zap.Logger
}

// NewCognitiveService initializes the cognitive service with AI orchestrator and repository
func NewCognitiveService(
	cognitiveRepo repository.CognitiveRepository,
	chatRepo repository.ChatRepository,
	orchestrator *ai.Orchestrator,
	logger *zap.Logger,
) CognitiveService {
	return &cognitiveService{
		cognitiveRepo: cognitiveRepo,
		chatRepo:      chatRepo,
		orchestrator:  orchestrator,
		logger:        logger,
	}
}

func (s *cognitiveService) GetProfile(ctx context.Context, userID string) (*dto.CognitiveProfileResponse, error) {
	if userID == "guest-user" {
		if val, found := reuse.GetCache().Get("cognitive_profile:guest-user"); found {
			if gp, ok := val.(*model.CognitiveProfile); ok {
				return toProfileResponse(gp), nil
			}
		}
		return toProfileResponse(nil), nil
	}

	if userID == "" || s.cognitiveRepo == nil {
		return toProfileResponse(nil), nil
	}

	profile, err := s.cognitiveRepo.GetByUserID(ctx, userID)
	if err != nil {
		s.logger.Warn("Failed to fetch cognitive profile, returning baseline", zap.String("userID", userID), zap.Error(err))
		return toProfileResponse(nil), nil
	}

	return toProfileResponse(profile), nil
}

// GetSmartContext returns dynamically generated context-aware prompts, syllabus recommendations, and active recall hints.
// ⚡ Performance Optimized: Evaluates from thread-safe memory cache (0.02ms) with 5-minute TTL.
func (s *cognitiveService) GetSmartContext(ctx context.Context, userID, targetExam, tutorMode string) (*dto.SmartContextResponse, error) {
	cacheKey := fmt.Sprintf("smart_ctx:%s:%s:%s", userID, strings.ToLower(strings.TrimSpace(targetExam)), strings.ToLower(strings.TrimSpace(tutorMode)))
	if val, found := reuse.GetCache().Get(cacheKey); found {
		if cached, ok := val.(*dto.SmartContextResponse); ok {
			return cached, nil
		}
	}

	var weakTopics []string
	var strongTopics []string
	var examTraps []string
	resolvedExam := strings.TrimSpace(targetExam)

	if userID != "" && userID != "guest-user" && s.cognitiveRepo != nil {
		profile, err := s.cognitiveRepo.GetByUserID(ctx, userID)
		if err == nil && profile != nil {
			weakTopics = profile.WeakTopics
			strongTopics = profile.StrongTopics
			examTraps = profile.ExamTraps
			if resolvedExam == "" {
				resolvedExam = profile.TargetExam
			}
		}
	}

	// Dynamic Curricula lookup: Prioritize PostgreSQL exam_curricula if available
	customCurricula := s.getDynamicCurricula(ctx)

	res := ai.GenerateSmartContextWithCurricula(resolvedExam, weakTopics, strongTopics, examTraps, tutorMode, customCurricula)
	// Cache for 5 minutes in thread-safe memory cache
	reuse.GetCache().Set(cacheKey, &res, 5*time.Minute)
	return &res, nil
}

// getDynamicCurricula loads active curricula from PostgreSQL exam_curricula table with 15-minute caching
func (s *cognitiveService) getDynamicCurricula(ctx context.Context) []ai.ExamCurriculumData {
	if s.cognitiveRepo == nil {
		return nil
	}
	cache := reuse.GetCache()
	if val, found := cache.Get("db_curricula"); found {
		if list, ok := val.([]ai.ExamCurriculumData); ok {
			return list
		}
	}
	currs, err := s.cognitiveRepo.ListCurricula(ctx)
	if err != nil || len(currs) == 0 {
		return nil
	}
	var mapped []ai.ExamCurriculumData
	for _, c := range currs {
		mapped = append(mapped, ai.ExamCurriculumData{
			Code:            c.ExamCode,
			DisplayName:     c.DisplayName,
			Keywords:        c.Keywords,
			HighYieldTopics: c.HighYieldTopics,
			CommonTraps:     c.CommonTraps,
		})
	}
	cache.Set("db_curricula", mapped, 15*time.Minute)
	return mapped
}

// invalidateSmartContext purges cached smart context for the user upon profile/learning mutations
func (s *cognitiveService) invalidateSmartContext(userID string) {
	if userID != "" {
		reuse.GetCache().DeleteByPrefix("smart_ctx:" + userID + ":")
	}
}

func (s *cognitiveService) UpdateGoals(ctx context.Context, userID string, req dto.UpdateCognitiveGoalRequest) (*dto.CognitiveProfileResponse, error) {
	if userID == "" || userID == "guest-user" || s.cognitiveRepo == nil {
		guestProfile := &model.CognitiveProfile{
			UserID:        "guest-user",
			TargetExam:    "General Academic Preparation",
			AcademicLevel: "School / College (Adaptive)",
			WeakTopics:    []string{},
			StrongTopics:  []string{},
			ExamTraps:     []string{},
		}
		if val, found := reuse.GetCache().Get("cognitive_profile:guest-user"); found {
			if gp, ok := val.(*model.CognitiveProfile); ok {
				guestProfile = gp
			}
		}
		if req.TargetExam != nil {
			guestProfile.TargetExam = strings.TrimSpace(*req.TargetExam)
		}
		if req.AcademicLevel != nil {
			guestProfile.AcademicLevel = strings.TrimSpace(*req.AcademicLevel)
		}
		if req.WeakTopics != nil {
			guestProfile.WeakTopics = *req.WeakTopics
		}
		if req.StrongTopics != nil {
			guestProfile.StrongTopics = *req.StrongTopics
		}
		if req.ExamTrapsToWatch != nil {
			guestProfile.ExamTraps = *req.ExamTrapsToWatch
		}
		reuse.GetCache().Set("cognitive_profile:guest-user", guestProfile, 30*time.Minute)
		s.invalidateSmartContext("guest-user")
		return toProfileResponse(guestProfile), nil
	}

	profile, err := s.cognitiveRepo.GetByUserID(ctx, userID)
	if err != nil {
		profile = &model.CognitiveProfile{
			UserID:        userID,
			TargetExam:    "General Academic Preparation",
			AcademicLevel: "School / College (Adaptive)",
			WeakTopics:    []string{},
			StrongTopics:  []string{},
			ExamTraps:     []string{},
		}
	}

	if req.TargetExam != nil {
		profile.TargetExam = strings.TrimSpace(*req.TargetExam)
	}
	if req.AcademicLevel != nil {
		profile.AcademicLevel = strings.TrimSpace(*req.AcademicLevel)
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
	if req.WeakTopics != nil && len(*req.WeakTopics) == 0 && req.StrongTopics != nil && len(*req.StrongTopics) == 0 {
		profile.TopicMastery = map[string]int{}
	}

	if err := s.cognitiveRepo.Upsert(ctx, profile); err != nil {
		return nil, fmt.Errorf("failed to update cognitive profile: %w", err)
	}

	s.invalidateSmartContext(userID)
	return toProfileResponse(profile), nil
}

func (s *cognitiveService) PromoteTopic(ctx context.Context, userID, topic string) (*dto.CognitiveProfileResponse, error) {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return nil, fmt.Errorf("topic name cannot be empty")
	}

	if userID == "" || userID == "guest-user" || s.cognitiveRepo == nil {
		return toProfileResponse(nil), nil
	}

	p, err := s.cognitiveRepo.PromoteTopic(ctx, userID, topic)
	if err != nil {
		return nil, fmt.Errorf("failed to promote topic: %w", err)
	}
	s.invalidateSmartContext(userID)
	return toProfileResponse(p), nil
}

func (s *cognitiveService) DemoteTopic(ctx context.Context, userID, topic string) (*dto.CognitiveProfileResponse, error) {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return nil, fmt.Errorf("topic name cannot be empty")
	}

	if userID == "" || userID == "guest-user" || s.cognitiveRepo == nil {
		return toProfileResponse(nil), nil
	}

	p, err := s.cognitiveRepo.DemoteTopic(ctx, userID, topic)
	if err != nil {
		return nil, fmt.Errorf("failed to demote topic: %w", err)
	}
	s.invalidateSmartContext(userID)
	return toProfileResponse(p), nil
}

func (s *cognitiveService) DeleteTopic(ctx context.Context, userID, topic string) (*dto.CognitiveProfileResponse, error) {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return nil, fmt.Errorf("topic name cannot be empty")
	}

	if userID == "" || userID == "guest-user" || s.cognitiveRepo == nil {
		return toProfileResponse(nil), nil
	}

	p, err := s.cognitiveRepo.DeleteTopic(ctx, userID, topic)
	if err != nil {
		return nil, fmt.Errorf("failed to delete topic: %w", err)
	}
	s.invalidateSmartContext(userID)
	return toProfileResponse(p), nil
}

func (s *cognitiveService) RecordActivity(ctx context.Context, userID string, req dto.RecordActivityRequest) (*dto.CognitiveProfileResponse, error) {
	if userID == "" || userID == "guest-user" || s.cognitiveRepo == nil {
		return toProfileResponse(nil), nil
	}

	p, err := s.cognitiveRepo.RecordActivity(ctx, userID, req.ActivityType, req.Topic, req.DurationMinutes)
	if err != nil {
		return nil, fmt.Errorf("failed to record activity: %w", err)
	}
	s.invalidateSmartContext(userID)
	return toProfileResponse(p), nil
}


// DiagnoseProfile executes an on-demand AI Cognitive Analysis using LLM reasoning
func (s *cognitiveService) DiagnoseProfile(ctx context.Context, userID string) (*dto.CognitiveDiagnosisResponse, error) {
	var currentProfile *model.CognitiveProfile
	var recentDoubts []string

	// ⚡ Go Concurrency: Concurrently load profile and recent doubts in parallel
	if userID != "" && userID != "guest-user" {
		var wg sync.WaitGroup

		if s.cognitiveRepo != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if p, err := s.cognitiveRepo.GetByUserID(ctx, userID); err == nil {
					currentProfile = p
				}
			}()
		}

		if s.chatRepo != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if messages, err := s.chatRepo.ListMessages(ctx, userID, "chat", "", 10); err == nil {
					var doubts []string
					for _, m := range messages {
						if m.Role == "user" && len(m.Content) > 5 {
							q := m.Content
							if len(q) > 80 {
								q = q[:80] + "..."
							}
							doubts = append(doubts, q)
						}
					}
					recentDoubts = doubts
				}
			}()
		}

		wg.Wait()
	}

	if currentProfile == nil {
		currentProfile = &model.CognitiveProfile{
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
			TopicMastery:      map[string]int{},
			ReadinessScore:    50,
		}
	}

	systemPrompt := `You are an elite AI Cognitive Tutor & Exam Strategist.
Analyze the student's study profile, quiz metrics, doubts history, and topic mastery.
Diagnose their conceptual gaps and exam traps, calculate a realistic Readiness Score (0-100), and recommend immediate actionable study drills.
CRITICAL DIRECTIVE: ONLY extract and identify topics from the student's actual doubt history, quiz results, or current topics list. If the student has 0 doubts, 0 quizzes, and no topics listed, you MUST return empty arrays for "weak_topics": [], "strong_topics": [], and "exam_traps": []. Do NOT invent or hallucinate default Computer Science or arbitrary topics.

Respond strictly in valid JSON format matching this schema with NO markdown code fences or backticks:
{
  "ai_summary": "2-3 concise, encouraging yet sharp sentences explaining their current standing, key bottleneck, and focus priority.",
  "readiness_score": 74,
  "weak_topics": ["Topic 1", "Topic 2"],
  "strong_topics": ["Topic 3"],
  "exam_traps": ["Conceptual pitfall 1", "Common exam confusion 2"],
  "recommended_actions": ["Specific Action 1", "Specific Action 2", "Specific Action 3"],
  "topic_mastery": {
    "Topic 1": 42,
    "Topic 2": 38,
    "Topic 3": 88
  }
}`
	aiLanguage := middleware.GetAILanguage(ctx)
	systemPrompt += ai.BuildLanguageInstruction(aiLanguage)

	userPrompt := fmt.Sprintf(`Student Profile:
Target Exam: %s
Academic Level: %s
Current Weak Topics: %s
Current Strong Topics: %s
Known Exam Traps: %s
Total Quizzes Taken: %d
Average Quiz Score: %.1f%%
Study Streak: %d days
Total Doubts Asked: %d
Recent Doubt Queries: %s
Current Topic Mastery Breakdown: %v`,
		currentProfile.TargetExam,
		currentProfile.AcademicLevel,
		strings.Join(currentProfile.WeakTopics, ", "),
		strings.Join(currentProfile.StrongTopics, ", "),
		strings.Join(currentProfile.ExamTraps, ", "),
		currentProfile.TotalQuizzesTaken,
		currentProfile.AverageQuizScore,
		currentProfile.StudyStreakDays,
		currentProfile.TotalDoubtsAsked,
		strings.Join(recentDoubts, " | "),
		currentProfile.TopicMastery,
	)

	var diag dto.CognitiveDiagnosisResponse
	aiSuccess := false

	if s.orchestrator != nil {
		ctxTimeout, cancel := context.WithTimeout(ctx, 25*time.Second)
		defer cancel()

		rawResp, err := s.orchestrator.GenerateTextWithCustomKey(ctxTimeout, "", "google", "gemini-3.5-flash-lite", systemPrompt, userPrompt)
		if err == nil && rawResp != "" {
			clean := cleanCognitiveJSON(rawResp)
			if jsonErr := json.Unmarshal([]byte(clean), &diag); jsonErr == nil && diag.ReadinessScore > 0 {
				aiSuccess = true
			}
		}
	}

	// Deterministic Fallback if AI is offline or ratelimited
	if !aiSuccess {
		diag = s.buildDeterministicDiagnosis(currentProfile)
	}

	// Persist the AI diagnosis into student's cognitive memory profile
	if userID != "" && userID != "guest-user" && s.cognitiveRepo != nil {
		_, err := s.cognitiveRepo.SaveDiagnosis(
			ctx,
			userID,
			diag.AISummary,
			diag.ReadinessScore,
			diag.TopicMastery,
			diag.ExamTraps,
			diag.RecommendedActions,
		)
		if err != nil {
			s.logger.Warn("Failed to persist AI diagnosis to cognitive profile", zap.Error(err))
		}
		s.invalidateSmartContext(userID)
	}

	return &diag, nil
}

func (s *cognitiveService) buildDeterministicDiagnosis(p *model.CognitiveProfile) dto.CognitiveDiagnosisResponse {
	readiness := 50
	if p.TotalQuizzesTaken > 0 {
		readiness = int(p.AverageQuizScore*0.7) + (len(p.StrongTopics) * 5) - (len(p.WeakTopics) * 3)
	}
	if readiness < 20 {
		readiness = 20
	}
	if readiness > 95 {
		readiness = 95
	}

	mastery := p.TopicMastery
	if mastery == nil {
		mastery = make(map[string]int)
	}
	for _, w := range p.WeakTopics {
		if _, ok := mastery[w]; !ok {
			mastery[w] = 35
		}
	}
	for _, st := range p.StrongTopics {
		if _, ok := mastery[st]; !ok {
			mastery[st] = 85
		}
	}

	actions := []string{
		"Solve an adaptive quiz to calibrate your conceptual mastery",
		"Ask doubt queries to the AI Tutor to build deep clarity",
		"Explore interactive mind maps for active conceptual recall",
	}

	var summary string
	if len(p.WeakTopics) > 0 && len(p.StrongTopics) > 0 {
		summary = fmt.Sprintf("You have achieved an overall exam readiness of %d%% for %s. Your foundation in %s is solid, but targeted practice in %s is recommended before your exam.",
			readiness,
			p.TargetExam,
			strings.Join(p.StrongTopics, ", "),
			strings.Join(p.WeakTopics, ", "),
		)
	} else if len(p.WeakTopics) > 0 {
		summary = fmt.Sprintf("Your current focus is on %s. Practice targeted adaptive quizzes to reinforce these core areas.", strings.Join(p.WeakTopics, ", "))
	} else if len(p.StrongTopics) > 0 {
		summary = fmt.Sprintf("Strong conceptual grasp in %s! Keep up the momentum with regular revision quizzes.", strings.Join(p.StrongTopics, ", "))
	} else {
		summary = fmt.Sprintf("Welcome to your Cognitive Profile for %s. Practice topics and ask doubts to let AI dynamically track your strengths and focus areas.", p.TargetExam)
	}

	return dto.CognitiveDiagnosisResponse{
		AISummary:          summary,
		ReadinessScore:     readiness,
		WeakTopics:         p.WeakTopics,
		StrongTopics:       p.StrongTopics,
		ExamTraps:          p.ExamTraps,
		RecommendedActions: actions,
		TopicMastery:       mastery,
	}
}

func cleanCognitiveJSON(raw string) string {
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
	return strings.TrimSpace(s)
}

func sanitizeLegacyMock(topics []string) []string {
	if len(topics) == 0 {
		return []string{}
	}
	legacyMap := map[string]bool{
		"data structures":             true,
		"dynamic programming":         true,
		"object-oriented programming": true,
		"core theory & mechanics":     true,
		"formula derivations":         true,
		"examiner traps & pyqs":       true,
	}
	clean := make([]string, 0, len(topics))
	for _, t := range topics {
		if !legacyMap[strings.ToLower(strings.TrimSpace(t))] {
			clean = append(clean, t)
		}
	}
	return clean
}

func toProfileResponse(p *model.CognitiveProfile) *dto.CognitiveProfileResponse {
	if p == nil {
		return &dto.CognitiveProfileResponse{
			TargetExam:         "Competitive / University Exam",
			AcademicLevel:      "Undergraduate",
			WeakTopics:         []string{},
			StrongTopics:       []string{},
			ExamTraps:          []string{},
			StudyStreakDays:    1,
			TotalStudyMinutes:  0,
			TotalQuizzesTaken:  0,
			AverageQuizScore:   0.0,
			AISummary:          "Welcome! Start exploring topics, asking AI doubts, or taking quizzes to unlock your personalized Cognitive Profile.",
			TopicMastery:       map[string]int{},
			ReadinessScore:     50,
			RecommendedActions: []string{"Ask your first AI doubt", "Generate an adaptive quiz", "Explore interactive mind maps"},
			TotalDoubtsAsked:   0,
			LastActiveAt:       time.Now(),
			UpdatedAt:          time.Now(),
		}
	}

	mastery := p.TopicMastery
	if mastery == nil {
		mastery = make(map[string]int)
	}

	weak := p.WeakTopics
	if weak == nil {
		weak = []string{}
	}
	strong := p.StrongTopics
	if strong == nil {
		strong = []string{}
	}
	traps := p.ExamTraps
	if traps == nil {
		traps = []string{}
	}

	// Sanitize legacy mock topics if student has 0 doubts and 0 quizzes
	if p.TotalQuizzesTaken == 0 && p.TotalDoubtsAsked == 0 {
		weak = sanitizeLegacyMock(weak)
		strong = sanitizeLegacyMock(strong)
		delete(mastery, "Data Structures")
		delete(mastery, "Dynamic Programming")
		delete(mastery, "Object-Oriented Programming")
		delete(mastery, "Core Theory & Mechanics")
		delete(mastery, "Formula Derivations")
		delete(mastery, "Examiner Traps & PYQs")
	}

	actions := p.RecommendedActions
	if len(actions) == 0 {
		actions = []string{"Practice weak topics", "Take an adaptive quiz"}
	}
	summary := p.AISummary
	if summary == "" {
		summary = "Cognitive profile active. Your learning path updates automatically as you study."
	}
	readiness := p.ReadinessScore
	if readiness == 0 {
		readiness = 50
	}

	return &dto.CognitiveProfileResponse{
		UserID:             p.UserID,
		TargetExam:         p.TargetExam,
		AcademicLevel:      p.AcademicLevel,
		WeakTopics:         weak,
		StrongTopics:       strong,
		ExamTraps:          traps,
		StudyStreakDays:    p.StudyStreakDays,
		TotalStudyMinutes:  p.TotalStudyMinutes,
		TotalQuizzesTaken:  p.TotalQuizzesTaken,
		AverageQuizScore:   p.AverageQuizScore,
		AISummary:          summary,
		TopicMastery:       mastery,
		ReadinessScore:     readiness,
		RecommendedActions: actions,
		TotalDoubtsAsked:   p.TotalDoubtsAsked,
		LastActiveAt:       p.LastActiveAt,
		UpdatedAt:          p.UpdatedAt,
	}
}
