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

func TestNormalizeGeminiModelPreservesCurrentModelIDs(t *testing.T) {
	tests := map[string]string{
		"":                       "gemini-3.8-flash",
		"gemini-flash":           "gemini-3.8-flash",
		"gemini-flash-lite":      "gemini-3.5-flash-lite",
		"gemini-pro":             "gemini-3.1-pro-preview",
		"gemini-3.8-flash":       "gemini-3.8-flash",
		"gemini-3.7-flash":       "gemini-3.7-flash",
		"gemini-3.6-flash":       "gemini-3.6-flash",
		"gemini-3.5-flash-lite":  "gemini-3.5-flash-lite",
		"gemini-3.1-pro-preview": "gemini-3.1-pro-preview",
	}

	for input, expected := range tests {
		t.Run(input, func(t *testing.T) {
			if actual := normalizeGeminiModel(input); actual != expected {
				t.Errorf("normalizeGeminiModel(%q) = %q, want %q", input, actual, expected)
			}
		})
	}
}

func TestGetGeminiCandidatesExcludeShutdownModels(t *testing.T) {
	shutdownModels := map[string]bool{
		"gemini-1.5-pro":        true,
		"gemini-1.5-flash":      true,
		"gemini-2.0-flash":      true,
		"gemini-2.0-flash-lite": true,
	}

	for _, model := range []string{"gemini-3.8-flash", "gemini-3.5-flash-lite", "gemini-3.1-pro-preview"} {
		candidates := getGeminiCandidates(model)
		if len(candidates) == 0 || candidates[0] != model {
			t.Errorf("first candidate for %q = %v, want requested model first", model, candidates)
		}
		for _, candidate := range candidates {
			if shutdownModels[candidate] {
				t.Errorf("candidate list for %q contains shut-down model %q", model, candidate)
			}
		}
	}
}

func TestOrchestrator_RequiresConfiguredProvider(t *testing.T) {
	orch := NewOrchestrator([]string{}, []string{}, []string{})
	ctx := context.Background()

	resp, err := orch.GenerateText(ctx, "You are an AI Tutor", "Explain Newton's First Law")
	if err == nil || !strings.Contains(err.Error(), "provider unavailable") {
		t.Fatalf("expected unavailable-provider error, got answer %q and error %v", resp, err)
	}
	if resp != "" {
		t.Fatalf("expected no fabricated response without a provider, got %q", resp)
	}

	resp, err = orch.GenerateVision(ctx, "You are an AI Tutor", "Solve this image", []byte{1}, "image/jpeg")
	if err == nil || !strings.Contains(err.Error(), "provider unavailable") {
		t.Fatalf("expected unavailable-provider error for vision request, got answer %q and error %v", resp, err)
	}
	if resp != "" {
		t.Fatalf("expected no fabricated vision response without a provider, got %q", resp)
	}
}
