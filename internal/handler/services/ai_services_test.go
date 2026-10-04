package services

import (
	"context"
	"testing"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/model"
	"goProjectStructure/internal/reuse/ai"
)

type mockChatRepo struct {
	messages []*model.ChatMessage
}

func (m *mockChatRepo) SaveMessage(ctx context.Context, msg *model.ChatMessage) error {
	msg.ID = "msg-1"
	m.messages = append(m.messages, msg)
	return nil
}

func (m *mockChatRepo) ListMessages(ctx context.Context, userID, scopeType, scopeID string, limit int) ([]*model.ChatMessage, error) {
	return m.messages, nil
}

func TestAITutorService_SolveDoubt(t *testing.T) {
	orch := ai.NewOrchestrator([]string{}, []string{}, []string{})
	chatRepo := &mockChatRepo{}
	tutor := NewAITutorService(orch, chatRepo, nil)

	ctx := context.Background()
	req := dto.DoubtRequest{
		Question:  "What is the difference between speed and velocity?",
		ScopeType: "global",
	}

	resp, err := tutor.SolveDoubt(ctx, "user-1", req)
	if err != nil {
		t.Fatalf("unexpected error solving doubt: %v", err)
	}

	if resp.Question != req.Question {
		t.Errorf("expected question %q, got %q", req.Question, resp.Question)
	}
	if resp.SolutionMarkdown == "" {
		t.Error("expected non-empty solution markdown")
	}
	if len(resp.FollowUpChips) == 0 {
		t.Error("expected follow-up chips")
	}

	// Verify chat message persisted
	if len(chatRepo.messages) != 2 {
		t.Fatalf("expected 2 chat messages saved (user and assistant), got %d", len(chatRepo.messages))
	}
}

func TestNotesAnalyzerService_Analyze(t *testing.T) {
	orch := ai.NewOrchestrator([]string{}, []string{}, []string{})
	noteRepo := &mockNoteRepo{}
	analyzer := NewNotesAnalyzerService(orch, noteRepo)

	ctx := context.Background()
	req := dto.NotesAnalysisRequest{
		Content:           "Newton's second law states F = ma. Force equals mass times acceleration.",
		IncludeFlashcards: true,
		IncludeSummary:    true,
	}

	resp, err := analyzer.AnalyzeNotes(ctx, "user-1", req)
	if err != nil {
		t.Fatalf("unexpected error analyzing notes: %v", err)
	}

	if resp.Summary == "" {
		t.Error("expected non-empty summary")
	}
	if len(resp.Flashcards) == 0 {
		t.Error("expected generated flashcards")
	}
	if len(resp.KeyPoints) == 0 {
		t.Error("expected key points")
	}
}

func TestDiagramService_Generate(t *testing.T) {
	orch := ai.NewOrchestrator([]string{}, []string{}, []string{})
	diagService := NewDiagramService(orch)

	ctx := context.Background()
	req := dto.DiagramRequest{
		Concept:     "Thermodynamics Laws",
		DiagramType: "mindmap",
	}

	resp, err := diagService.GenerateDiagram(ctx, "user-1", req)
	if err != nil {
		t.Fatalf("unexpected error generating diagram: %v", err)
	}

	if resp.Concept != req.Concept {
		t.Errorf("expected concept %q, got %q", req.Concept, resp.Concept)
	}
	if resp.Root == nil {
		t.Fatal("expected non-nil root mind map node")
	}
	if len(resp.Root.Children) == 0 {
		t.Error("expected mind map root children")
	}
	if len(resp.Nodes) == 0 || len(resp.Edges) == 0 {
		t.Error("expected graph nodes and edges for CustomPainter")
	}
}
