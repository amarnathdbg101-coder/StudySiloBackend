package ai

import (
	"context"
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
