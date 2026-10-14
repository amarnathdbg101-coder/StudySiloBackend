package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/model"
	"goProjectStructure/internal/handler/repository"
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
	if userID == "" || userID == "guest-user" || s.cognitiveRepo == nil {
		return toProfileResponse(nil), nil
	}

	profile, err := s.cognitiveRepo.GetByUserID(ctx, userID)
	if err != nil {
		s.logger.Warn("Failed to fetch cognitive profile, returning baseline", zap.String("userID", userID), zap.Error(err))
		return toProfileResponse(nil), nil
	}

	return toProfileResponse(profile), nil
}

func (s *cognitiveService) UpdateGoals(ctx context.Context, userID string, req dto.UpdateCognitiveGoalRequest) (*dto.CognitiveProfileResponse, error) {
	if userID == "" || userID == "guest-user" || s.cognitiveRepo == nil {
		return toProfileResponse(nil), nil
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

	if err := s.cognitiveRepo.Upsert(ctx, profile); err != nil {
		return nil, fmt.Errorf("failed to update cognitive profile: %w", err)
	}

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
	return toProfileResponse(p), nil
}

// DiagnoseProfile executes an on-demand AI Cognitive Analysis using LLM reasoning
func (s *cognitiveService) DiagnoseProfile(ctx context.Context, userID string) (*dto.CognitiveDiagnosisResponse, error) {
	var currentProfile *model.CognitiveProfile
	if userID != "" && userID != "guest-user" && s.cognitiveRepo != nil {
		p, err := s.cognitiveRepo.GetByUserID(ctx, userID)
		if err == nil {
			currentProfile = p
		}
	}

	if currentProfile == nil {
		currentProfile = &model.CognitiveProfile{
			UserID:            userID,
			TargetExam:        "Competitive / University Exam",
			AcademicLevel:     "Undergraduate",
			WeakTopics:        []string{"Data Structures", "Dynamic Programming"},
			StrongTopics:      []string{"Object-Oriented Programming"},
			ExamTraps:         []string{"Off-by-one boundary cases", "Time complexity underestimation"},
			StudyStreakDays:   1,
			TotalStudyMinutes: 30,
			TotalQuizzesTaken: 2,
			AverageQuizScore:  65.0,
			TopicMastery: map[string]int{
				"Data Structures":               45,
				"Dynamic Programming":           35,
				"Object-Oriented Programming": 85,
			},
			ReadinessScore: 62,
		}
	}

	// Prepare recent context for AI
	var recentDoubts []string
	if userID != "" && userID != "guest-user" && s.chatRepo != nil {
		messages, err := s.chatRepo.ListMessages(ctx, userID, "chat", "", 10)
		if err == nil {
			for _, m := range messages {
				if m.Role == "user" && len(m.Content) > 5 {
					q := m.Content
					if len(q) > 80 {
						q = q[:80] + "..."
					}
					recentDoubts = append(recentDoubts, q)
				}
			}
		}
	}

	systemPrompt := `You are an elite AI Cognitive Tutor & Exam Strategist.
Analyze the student's study profile, quiz metrics, doubts history, and topic mastery.
Diagnose their conceptual gaps and exam traps, calculate a realistic Readiness Score (0-100), and recommend immediate actionable study drills.

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
		"Solve a 5-question targeted adaptive quiz on your weakest topic",
		"Review core concepts using an AI-generated mind map",
		"Resolve pending doubt queries with the AI Tutor",
	}

	summary := fmt.Sprintf("You have achieved an overall exam readiness of %d%% for %s. Your foundation in %s is solid, but targeted practice in %s is needed before your target exam.",
		readiness,
		p.TargetExam,
		strings.Join(p.StrongTopics, ", "),
		strings.Join(p.WeakTopics, ", "),
	)
	if len(p.WeakTopics) == 0 {
		summary = fmt.Sprintf("Great momentum! Your readiness score is %d%% across all subjects. Take an advanced adaptive quiz to discover higher-order traps.", readiness)
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
			AISummary:          "Welcome! Start exploring topics, asking AI doubts, or taking quizzes to unlock your AI Cognitive Profile.",
			TopicMastery:       map[string]int{},
			ReadinessScore:     50,
			RecommendedActions: []string{"Ask your first AI doubt", "Generate a diagnostic quiz", "Create a visual mind map"},
			TotalDoubtsAsked:   0,
			LastActiveAt:       time.Now(),
			UpdatedAt:          time.Now(),
		}
	}

	mastery := p.TopicMastery
	if mastery == nil {
		mastery = make(map[string]int)
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
		readiness = 60
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
