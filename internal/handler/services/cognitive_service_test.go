package services

import (
	"context"
	"testing"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/model"
	"goProjectStructure/internal/reuse/ai"

	"go.uber.org/zap"
)

func TestCognitiveService_Flow(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	mockRepo := &mockCognitiveRepo{
		profiles: make(map[string]*model.CognitiveProfile),
	}
	orch := ai.NewOrchestrator([]string{}, []string{}, []string{})

	svc := NewCognitiveService(mockRepo, nil, orch, logger)

	// 1. Get default profile for new user
	resp, err := svc.GetProfile(ctx, "test-user-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.UserID != "test-user-1" {
		t.Errorf("expected userID test-user-1, got %s", resp.UserID)
	}
	if resp.TargetExam != "JEE Advanced" { // seeded from mockCognitiveRepo
		t.Errorf("expected JEE Advanced, got %s", resp.TargetExam)
	}

	// 2. Update Goals
	newExam := "NEET PG"
	newLevel := "Postgraduate"
	upResp, err := svc.UpdateGoals(ctx, "test-user-1", dto.UpdateCognitiveGoalRequest{
		TargetExam:    &newExam,
		AcademicLevel: &newLevel,
	})
	if err != nil {
		t.Fatalf("failed to update goals: %v", err)
	}
	if upResp.TargetExam != "NEET PG" {
		t.Errorf("expected NEET PG, got %s", upResp.TargetExam)
	}

	// 3. Promote topic
	promoted, err := svc.PromoteTopic(ctx, "test-user-1", "Rotational Dynamics")
	if err != nil {
		t.Fatalf("failed to promote topic: %v", err)
	}
	found := false
	for _, st := range promoted.StrongTopics {
		if st == "Rotational Dynamics" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected Rotational Dynamics in strong topics, got %v", promoted.StrongTopics)
	}

	// 4. Run Diagnosis (Deterministic fallback tested here)
	diag, err := svc.DiagnoseProfile(ctx, "test-user-1")
	if err != nil {
		t.Fatalf("failed to run diagnosis: %v", err)
	}
	if diag.ReadinessScore <= 0 {
		t.Errorf("expected positive readiness score, got %d", diag.ReadinessScore)
	}
	if len(diag.RecommendedActions) == 0 {
		t.Errorf("expected recommended actions in diagnosis")
	}

	// 5. Test SmartContext caching & invalidation
	ctxResp1, err := svc.GetSmartContext(ctx, "test-user-1", "GATE Computer Science", "socratic")
	if err != nil {
		t.Fatalf("failed to get smart context: %v", err)
	}
	if len(ctxResp1.QuickChips) == 0 {
		t.Errorf("expected quick chips in smart context")
	}

	// Repeat call should hit memory cache instantly
	ctxResp2, err := svc.GetSmartContext(ctx, "test-user-1", "GATE Computer Science", "socratic")
	if err != nil {
		t.Fatalf("failed to get smart context second time: %v", err)
	}
	if ctxResp1.FocusTopic != ctxResp2.FocusTopic {
		t.Errorf("expected cached focus topic to match: %s vs %s", ctxResp1.FocusTopic, ctxResp2.FocusTopic)
	}

	// 6. Test Custom Instructions in UpdateGoals
	customInstr := "Explain strictly in comprehensive paragraphs with diagrams and real-world examples."
	upCustom, err := svc.UpdateGoals(ctx, "test-user-1", dto.UpdateCognitiveGoalRequest{
		CustomInstructions: &customInstr,
	})
	if err != nil {
		t.Fatalf("failed to update custom instructions: %v", err)
	}
	if upCustom.CustomInstructions != customInstr {
		t.Errorf("expected custom instructions %q, got %q", customInstr, upCustom.CustomInstructions)
	}
}

func TestExtractCanonicalConcept(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"What is cache memory?", "Cache Memory"},
		{"Explain difference between TCP and UDP in detail", "TCP And UDP"},
		{"kya hota hai binary search tree samjhao", "Binary Search Tree"},
		{"Define Von Neumann architecture with diagrams", "Von Neumann Architecture"},
		{"Photosynthesis explain karo", "Photosynthesis"},
		{"What do you mean by deadlocks in OS?", "Deadlocks In OS"},
		{"hi", "General Concept"},
		{"   ", "General Concept"},
	}

	for _, tc := range tests {
		actual := ExtractCanonicalConcept(tc.input)
		if actual != tc.expected {
			t.Errorf("ExtractCanonicalConcept(%q) = %q, expected %q", tc.input, actual, tc.expected)
		}
	}
}
