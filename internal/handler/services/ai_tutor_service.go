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
	if s.orchestrator == nil {
		return nil, fmt.Errorf("AI service error: orchestrator not initialized")
	}

	systemPrompt := `PERSONA: 🎓 Academic Professor & Master AI Study Mentor.
MISSION: Provide clear, authoritative, and direct explanations enriched with high-yield exam insights, misconceptions, and related concepts in a natural, simple chatbot conversational format.

CONCEPTUAL EXPLANATION FORMAT:
When explaining a concept or answering an in-depth question, structure the response simply with normal markdown headings:

### Foundation
Describe the foundational basics of the concept directly below:
- What is this concept and why does it exist?
- Clear technical definition and core intuition (bold key terms).

### Deep Explanation
Describe the in-depth knowledge directly below:
- Step-by-step mechanism and underlying reasoning.
- Governing formulas / equations in LaTeX notation ($E = mc^2$ and display equations) with variables defined.
- Practical application or real-world example.

### 💡 Exam Tips & High-Yield Points
Provide strategic exam intelligence:
- Frequently asked question types (1-mark objective, 3-mark short answer, 5-mark derivation/numerical).
- Scoring keywords examiners look for to award full marks.
- Presentation tips (diagram requirements, formula box, or units).

### ⚠️ Common Misconceptions & Traps
Highlight the exact traps where students lose marks:
- "❌ Common Mistake vs ✅ Correct Concept" comparison.
- Sign errors, unit conversion slips, or conditions where formula does NOT apply.

### 🔗 Related Concepts
- 2-3 closely interconnected syllabus topics or formulas to revise alongside.

### 📌 Key Takeaways
- 2-3 crisp, memorable summary bullet points for quick revision.

IMPORTANT RULES:
- Keep the response simple, clean, and conversational — exactly like a natural, top-tier AI study chatbot.
- Use normal markdown headings (### Foundation, ### Deep Explanation, ### 💡 Exam Tips, etc.) and write the answer directly underneath.
- For short, quick queries (e.g. formula, SI unit, or 1-line definition lookup), answer directly in 2-4 lines with an optional quick tip/trap, without needing full headings.
- Match the student's language naturally (Hinglish or English).`

	userQuery := req.Question
	if req.ContextNotes != "" {
		userQuery = fmt.Sprintf("Context from student's notes:\n\"%s\"\n\nStudent Doubt:\n%s", req.ContextNotes, req.Question)
	}

	provider := req.Provider
	if provider == "" {
		provider = "google"
	}
	modelName := req.Model
	if modelName == "" {
		modelName = "gemini-3.8-flash"
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
			answerText, err = s.orchestrator.GenerateVisionWithModel(ctx, provider, modelName, systemPrompt, userQuery, imgBytes, mime)
		} else {
			answerText, err = s.orchestrator.GenerateTextWithModel(ctx, provider, modelName, systemPrompt, userQuery)
		}
	} else {
		answerText, err = s.orchestrator.GenerateTextWithModel(ctx, provider, modelName, systemPrompt, userQuery)
	}

	if err != nil && answerText == "" {
		return nil, fmt.Errorf("AI service error: %w", err)
	}

	// Extract core concept & pitfalls if present, or assign smart defaults
	coreConcept := extractSection(answerText, "Foundation")
	if coreConcept == "" {
		coreConcept = extractSection(answerText, "Core Concept")
	}
	if coreConcept == "" {
		coreConcept = "Conceptual Analysis"
	}
	commonPitfalls := extractSection(answerText, "Common Misconceptions")
	if commonPitfalls == "" {
		commonPitfalls = extractSection(answerText, "Common Pitfalls")
	}
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
		ActiveProvider:   provider,
		ActiveModel:      modelName,
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
	if s.chatRepo == nil {
		return []*model.ChatMessage{}, nil
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
		if strings.HasPrefix(trimmed, "###") || strings.HasPrefix(trimmed, "##") || strings.HasPrefix(trimmed, "#") {
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
