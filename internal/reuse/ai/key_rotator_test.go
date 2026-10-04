package ai

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestKeyRotator_RoundRobin(t *testing.T) {
	keys := []string{"key-1", "key-2", "key-3"}
	rotator := NewKeyRotator(keys)

	k1, err := rotator.GetNextKey()
	if err != nil || k1 != "key-1" {
		t.Fatalf("expected key-1, got %v (err: %v)", k1, err)
	}

	k2, err := rotator.GetNextKey()
	if err != nil || k2 != "key-2" {
		t.Fatalf("expected key-2, got %v (err: %v)", k2, err)
	}

	k3, err := rotator.GetNextKey()
	if err != nil || k3 != "key-3" {
		t.Fatalf("expected key-3, got %v (err: %v)", k3, err)
	}

	// Should wrap around to key-1
	k4, err := rotator.GetNextKey()
	if err != nil || k4 != "key-1" {
		t.Fatalf("expected wrapped key-1, got %v (err: %v)", k4, err)
	}
}

func TestKeyRotator_RateLimitCooldown(t *testing.T) {
	keys := []string{"key-a", "key-b"}
	rotator := NewKeyRotator(keys)

	// Mark key-a as rate limited for 500ms
	rotator.MarkRateLimited("key-a", 500*time.Millisecond)

	// Next key should skip key-a and return key-b
	k, err := rotator.GetNextKey()
	if err != nil || k != "key-b" {
		t.Fatalf("expected key-b when key-a is rate limited, got %v (err: %v)", k, err)
	}

	// If key-b is also rate limited
	rotator.MarkRateLimited("key-b", 500*time.Millisecond)
	_, err = rotator.GetNextKey()
	if err != ErrAllKeysExhausted {
		t.Fatalf("expected ErrAllKeysExhausted, got %v", err)
	}

	// Wait for cooldown to expire
	time.Sleep(550 * time.Millisecond)

	recoveredKey, err := rotator.GetNextKey()
	if err != nil || recoveredKey == "" {
		t.Fatalf("expected key recovery after cooldown, got %v (err: %v)", recoveredKey, err)
	}
}

func TestKeyRotator_EmptyKeys(t *testing.T) {
	rotator := NewKeyRotator([]string{})
	_, err := rotator.GetNextKey()
	if err != ErrNoKeysConfigured {
		t.Fatalf("expected ErrNoKeysConfigured, got %v", err)
	}
}

func TestOrchestrator_MockFallback(t *testing.T) {
	orch := NewOrchestrator([]string{}, []string{}, []string{})
	ctx := context.Background()

	resp, err := orch.GenerateText(ctx, "Explain Newton's First Law", "Concept query")
	if err != nil {
		t.Fatalf("expected fallback mock response without error, got %v", err)
	}
	if resp == "" {
		t.Fatal("expected non-empty mock response")
	}
}

func TestOrchestrator_DistinctResponsesForArchitectureVsMemoryHierarchy(t *testing.T) {
	orch := NewOrchestrator([]string{}, []string{}, []string{})
	ctx := context.Background()

	// Query 1: General Computer Architecture
	resp1, err1 := orch.GenerateText(ctx, "You are an AI Tutor", "Explain computer architecture")
	if err1 != nil {
		t.Fatalf("unexpected error on query 1: %v", err1)
	}

	// Query 2: Memory Hierarchy
	resp2, err2 := orch.GenerateText(ctx, "You are an AI Tutor", "computer architecture ke andar ke hierarchy ko samjhao, memory hierarchy")
	if err2 != nil {
		t.Fatalf("unexpected error on query 2: %v", err2)
	}

	if resp1 == resp2 {
		t.Fatal("expected distinct responses for 'computer architecture' vs 'memory hierarchy', but got identical response!")
	}

	if !strings.Contains(resp2, "AMAT") && !strings.Contains(resp2, "Average Memory Access Time") {
		t.Errorf("expected memory hierarchy response to contain AMAT formulation, got: %s", resp2[:min(len(resp2), 200)])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestOrchestrator_FlexibleConversationalResponse(t *testing.T) {
	orch := NewOrchestrator([]string{}, []string{}, []string{})
	ctx := context.Background()

	// Question: "Tum kya PDF generate kar sakte ho?"
	resp, err := orch.GenerateText(ctx, "You are an AI Tutor", "Tum kya PDF generate kar sakte ho?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Must directly address PDF export
	if !strings.Contains(resp, "PDF") && !strings.Contains(resp, "pdf") {
		t.Errorf("expected response to address PDF capabilities, got: %s", resp)
	}

	// Must NOT contain artificial Core Problem or SI unit table
	if strings.Contains(resp, "The Core Problem") {
		t.Errorf("conversational response should NOT contain 'The Core Problem', got: %s", resp)
	}
	if strings.Contains(resp, "Exam Weightage") || strings.Contains(resp, "3-mark conceptual reasoning") {
		t.Errorf("conversational response should NOT contain exam weightage blueprints, got: %s", resp)
	}
	if strings.Contains(resp, "Phi_{net}") || strings.Contains(resp, "\\Phi") {
		t.Errorf("conversational response should NOT contain fake equations, got: %s", resp)
	}
}
