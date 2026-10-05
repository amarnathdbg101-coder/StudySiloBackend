package services

import (
	"context"
	"strings"
	"testing"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/model"
	"goProjectStructure/internal/reuse/ai"
)

type mockChatRepo struct {
	messages                []*model.ChatMessage
	listSessionMessagesCall int
}

func (m *mockChatRepo) SaveMessage(ctx context.Context, msg *model.ChatMessage) error {
	msg.ID = "msg-1"
	m.messages = append(m.messages, msg)
	return nil
}

func (m *mockChatRepo) ListMessages(ctx context.Context, userID, scopeType, scopeID string, limit int) ([]*model.ChatMessage, error) {
	return m.messages, nil
}

func (m *mockChatRepo) CreateSession(ctx context.Context, session *model.ChatSession) error {
	session.ID = "session-1"
	return nil
}

func (m *mockChatRepo) GetSession(ctx context.Context, userID, sessionID string) (*model.ChatSession, error) {
	return &model.ChatSession{ID: sessionID, UserID: userID, Title: "Test Session"}, nil
}

func (m *mockChatRepo) ListSessions(ctx context.Context, userID string, limit int) ([]*model.ChatSession, error) {
	return []*model.ChatSession{{ID: "session-1", UserID: userID, Title: "Test Session"}}, nil
}

func (m *mockChatRepo) TouchSession(ctx context.Context, userID, sessionID string) error {
	return nil
}

func (m *mockChatRepo) UpdateSession(ctx context.Context, userID, sessionID, title string) error {
	return nil
}

func (m *mockChatRepo) DeleteSession(ctx context.Context, userID, sessionID string) error {
	return nil
}

func (m *mockChatRepo) ListSessionMessages(ctx context.Context, userID, sessionID string, limit int) ([]*model.ChatMessage, error) {
	m.listSessionMessagesCall++
	return m.messages, nil
}

func TestAITutorService_SolveDoubt(t *testing.T) {
	orch := ai.NewOrchestrator([]string{}, []string{}, []string{})
	chatRepo := &mockChatRepo{}
	tutor := NewAITutorService(orch, chatRepo)

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

	_, err = tutor.SolveDoubt(ctx, "user-1", dto.DoubtRequest{
		Question:  "Explain this question in a different way.",
		ScopeType: "chat",
		SessionID: "session-1",
	})
	if err != nil {
		t.Fatalf("unexpected error solving session doubt: %v", err)
	}
	if chatRepo.listSessionMessagesCall != 0 {
		t.Errorf("expected session answers not to load stored messages, got %d history reads", chatRepo.listSessionMessagesCall)
	}

	// Verify computer architecture doubt receives accurate CS architecture explanation
	csReq := dto.DoubtRequest{
		Question:     "explain computer architecture",
		ContextNotes: "[STUDENT COGNITIVE PROFILE]\n• Target Exam: University Semester Exams",
		ScopeType:    "global",
	}
	csResp, err := tutor.SolveDoubt(ctx, "user-1", csReq)
	if err != nil {
		t.Fatalf("unexpected error solving CS doubt: %v", err)
	}
	if !strings.Contains(csResp.SolutionMarkdown, "Von Neumann") {
		t.Errorf("expected CS architecture solution to mention Von Neumann, got: %s", csResp.SolutionMarkdown)
	}
	if strings.Contains(csResp.SolutionMarkdown, `\lim_{\Delta t \to 0}`) {
		t.Errorf("CS architecture solution should NOT contain velocity derivative formula!")
	}

	greetingResp, err := tutor.SolveDoubt(ctx, "user-1", dto.DoubtRequest{Question: "Hi", ScopeType: "global"})
	if err != nil {
		t.Fatalf("unexpected error answering greeting: %v", err)
	}
	if greetingResp.CoreConcept != "" || greetingResp.CommonPitfalls != "" {
		t.Errorf("greeting should not receive fabricated study sections: core=%q pitfalls=%q", greetingResp.CoreConcept, greetingResp.CommonPitfalls)
	}
	if len(greetingResp.FollowUpChips) != 0 {
		t.Errorf("greeting should not receive academic follow-up chips: %v", greetingResp.FollowUpChips)
	}
}

func TestBuildTutorPromptAdaptsToRequestedDepth(t *testing.T) {
	shortPrompt := buildTutorPrompt("", "What is photosynthesis?", "")
	if !strings.Contains(shortPrompt, "a long sentence alone does not mean the user wants a long answer") {
		t.Fatal("expected default prompt to avoid inferring depth from question length")
	}
	if strings.Contains(shortPrompt, "### Foundation") {
		t.Fatal("expected default prompt not to require fixed academic headings")
	}

	deepPrompt := buildTutorPrompt("", "What is photosynthesis? Explain in detail", "")
	if !strings.Contains(deepPrompt, "IN-DEPTH CONCEPTUAL REQUEST") {
		t.Fatal("expected explicit detail request to receive in-depth guidance")
	}
	if !strings.Contains(deepPrompt, "use headings, examples, equations, or bullets only when they help") {
		t.Fatal("expected in-depth response format to remain flexible")
	}

	generalPrompt := buildTutorPrompt("", "Can you tell me a fun fact about the moon?", "")
	if !strings.Contains(generalPrompt, "answer naturally, not like an exam worksheet") {
		t.Fatal("expected general questions to use natural conversational guidance")
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
