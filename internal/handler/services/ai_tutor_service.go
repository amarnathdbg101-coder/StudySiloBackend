package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/model"
	"goProjectStructure/internal/handler/repository"
	"goProjectStructure/internal/reuse/ai"
)

type AITutorService interface {
	SolveDoubt(ctx context.Context, userID string, req dto.DoubtRequest) (*dto.DoubtResponse, error)
	GetChatHistory(ctx context.Context, userID, scopeType, scopeID string) ([]*model.ChatMessage, error)
}

type aiTutorService struct {
	orchestrator *ai.Orchestrator
	chatRepo     repository.ChatRepository
}

func NewAITutorService(orchestrator *ai.Orchestrator, chatRepo repository.ChatRepository) AITutorService {
	return &aiTutorService{
		orchestrator: orchestrator,
		chatRepo:     chatRepo,
	}
}

func (s *aiTutorService) SolveDoubt(ctx context.Context, userID string, req dto.DoubtRequest) (*dto.DoubtResponse, error) {
	systemPrompt := `You are an elite, pedagogical AI Tutor for university and competitive exam students (JEE, NEET, SAT, College).
Your goal is to explain concepts clearly step-by-step.
Follow these guidelines strictly:
1. Always format mathematical and scientific formulas in clean LaTeX notation using single dollar signs for inline ($E = mc^2$) and double dollar signs for display equations.
2. Structure your explanation:
   - ## 🎯 Core Concept: State the underlying law, formula, or principle.
   - ## 📝 Step-by-Step Resolution: Break down the problem logically with clear steps.
   - ## ⚠️ Common Pitfalls & Exam Traps: Highlight subtle mistakes students make.
3. Keep the tone encouraging, concise, and academic.`

	userQuery := req.Question
	if req.ContextNotes != "" {
		userQuery = fmt.Sprintf("Context from student's notes:\n\"%s\"\n\nStudent Doubt:\n%s", req.ContextNotes, req.Question)
	}

	var answerText string
	var err error

	// If handwritten image is provided, run vision multimodal pipeline
	if req.ImageBase64 != "" {
		imgBytes, decErr := base64.StdEncoding.DecodeString(req.ImageBase64)
		if decErr == nil && len(imgBytes) > 0 {
			mime := req.ImageMimeType
			if mime == "" {
				mime = "image/jpeg"
			}
			answerText, err = s.orchestrator.GenerateVision(ctx, systemPrompt, userQuery, imgBytes, mime)
		} else {
			answerText, err = s.orchestrator.GenerateText(ctx, systemPrompt, userQuery)
		}
	} else {
		answerText, err = s.orchestrator.GenerateText(ctx, systemPrompt, userQuery)
	}

	if err != nil && answerText == "" {
		return nil, fmt.Errorf("AI service error: %w", err)
	}

	// Extract core concept & pitfalls if present, or assign smart defaults
	coreConcept := extractSection(answerText, "Core Concept")
	if coreConcept == "" {
		coreConcept = "Conceptual Analysis"
	}
	commonPitfalls := extractSection(answerText, "Common Pitfalls")
	if commonPitfalls == "" {
		commonPitfalls = "Verify units and boundary conditions."
	}

	// Generate follow up chips
	followUpChips := generateFollowUpChips(req.Question)

	resp := &dto.DoubtResponse{
		Question:         req.Question,
		SolutionMarkdown: answerText,
		CoreConcept:      coreConcept,
		CommonPitfalls:   commonPitfalls,
		FollowUpChips:    followUpChips,
	}

	// Persist to chat messages if user is authenticated
	if userID != "" && s.chatRepo != nil {
		scopeType := req.ScopeType
		if scopeType == "" {
			scopeType = "global"
		}
		_ = s.chatRepo.SaveMessage(ctx, &model.ChatMessage{
			UserID:    userID,
			ScopeType: scopeType,
			ScopeID:   req.ScopeID,
			Role:      "user",
			Content:   req.Question,
		})
		_ = s.chatRepo.SaveMessage(ctx, &model.ChatMessage{
			UserID:    userID,
			ScopeType: scopeType,
			ScopeID:   req.ScopeID,
			Role:      "assistant",
			Content:   answerText,
		})
	}

	return resp, nil
}

func (s *aiTutorService) GetChatHistory(ctx context.Context, userID, scopeType, scopeID string) ([]*model.ChatMessage, error) {
	if scopeType == "" {
		scopeType = "global"
	}
	return s.chatRepo.ListMessages(ctx, userID, scopeType, scopeID, 50)
}

func extractSection(text, title string) string {
	lower := strings.ToLower(text)
	target := strings.ToLower(title)
	idx := strings.Index(lower, target)
	if idx == -1 {
		return ""
	}
	sub := text[idx:]
	lines := strings.Split(sub, "\n")
	content := []string{}
	for i, l := range lines {
		if i == 0 {
			continue
		}
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "##") || strings.HasPrefix(trimmed, "#") {
			break
		}
		if trimmed != "" {
			content = append(content, trimmed)
		}
	}
	return strings.Join(content, " ")
}

func generateFollowUpChips(question string) []string {
	return []string{
		"Give a practice question on this",
		"Explain with a real-world analogy",
		"Show visual diagram",
		"What are the key exam formulas?",
	}
}
