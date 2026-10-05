package ai

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

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
		"":                       "gemini-3.5-flash-lite",
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

func TestOrchestratorDefaultsToFlashLite(t *testing.T) {
	orch := NewOrchestrator([]string{"key-1"}, nil, nil)
	provider, ok := orch.GetProvider("google", "").(*GeminiProvider)
	if !ok {
		t.Fatal("expected default Google provider to be Gemini")
	}
	if provider.model != "gemini-3.5-flash-lite" {
		t.Errorf("expected fast default model, got %q", provider.model)
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

func TestGeminiRetriesWithNextKeyAfterRateLimit(t *testing.T) {
	var usedKeys []string
	provider := NewGeminiProvider([]string{"key-1", "key-2"}, "gemini-3.8-flash")
	provider.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		key := req.Header.Get("x-goog-api-key")
		usedKeys = append(usedKeys, key)
		statusCode := http.StatusOK
		body := `{"candidates":[{"content":{"parts":[{"text":"fresh answer"}]}}]}`
		if key == "key-1" {
			statusCode = http.StatusTooManyRequests
			body = `{"error":{"message":"quota exceeded"}}`
		}
		return &http.Response{
			StatusCode: statusCode,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	answer, err := provider.GenerateText(context.Background(), "system", "question")
	if err != nil {
		t.Fatalf("expected the second key to succeed, got error: %v", err)
	}
	if answer != "fresh answer" {
		t.Fatalf("expected fresh answer, got %q", answer)
	}
	if len(usedKeys) != 2 || usedKeys[0] != "key-1" || usedKeys[1] != "key-2" {
		t.Fatalf("expected one attempt per key, got %v", usedKeys)
	}
}

func TestGeminiRetriesNextModelAfterServiceUnavailable(t *testing.T) {
	var requestedModels []string
	provider := NewGeminiProvider([]string{"key-1"}, "gemini-3.8-flash")
	provider.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestedModels = append(requestedModels, req.URL.Path)
		statusCode := http.StatusOK
		body := `{"candidates":[{"content":{"parts":[{"text":"fallback model answer"}]}}]}`
		if strings.Contains(req.URL.Path, "gemini-3.8-flash") {
			statusCode = http.StatusServiceUnavailable
			body = `{"error":{"message":"model temporarily unavailable"}}`
		}
		return &http.Response{
			StatusCode: statusCode,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	answer, err := provider.GenerateText(context.Background(), "system", "question")
	if err != nil {
		t.Fatalf("expected fallback model to succeed, got error: %v", err)
	}
	if answer != "fallback model answer" {
		t.Fatalf("expected fallback model answer, got %q", answer)
	}
	if len(requestedModels) != 2 || !strings.Contains(requestedModels[0], "gemini-3.8-flash") || !strings.Contains(requestedModels[1], "gemini-3.7-flash") {
		t.Fatalf("expected 3.8 then 3.7 Flash attempts, got %v", requestedModels)
	}
}

func TestGeminiTextStreamEmitsIncrementalChunks(t *testing.T) {
	provider := NewGeminiProvider([]string{"test-key"}, "gemini-3.5-flash-lite")
	provider.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if !strings.Contains(req.URL.String(), ":streamGenerateContent?alt=sse") {
			t.Errorf("expected Gemini SSE endpoint, got %s", req.URL)
		}
		if req.Header.Get("x-goog-api-key") != "test-key" {
			t.Errorf("expected API key header")
		}
		body := "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Hello \"}]}}]}\n\n" +
			"data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"world\"}]}}]}\n\n"
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	var chunks []string
	answer, err := provider.GenerateTextStream(context.Background(), "system", "question", func(chunk string) {
		chunks = append(chunks, chunk)
	})
	if err != nil {
		t.Fatalf("unexpected stream error: %v", err)
	}
	if answer != "Hello world" {
		t.Fatalf("expected combined answer, got %q", answer)
	}
	if len(chunks) != 2 || chunks[0] != "Hello " || chunks[1] != "world" {
		t.Fatalf("expected incremental text chunks, got %v", chunks)
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
