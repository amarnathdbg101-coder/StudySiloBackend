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
}
