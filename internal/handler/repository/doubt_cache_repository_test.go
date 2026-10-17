package repository

import (
	"testing"

	"goProjectStructure/internal/handler/dto"
)

func TestComputeDoubtCacheKey_StrictDifferentiation(t *testing.T) {
	req1 := dto.DoubtRequest{
		Question: "What is Newton's first law of motion?",
		Mode:     "academic",
		Model:    "gemini-3.5-flash-lite",
	}

	req2 := dto.DoubtRequest{
		Question: "What is Newton's second law of motion?",
		Mode:     "academic",
		Model:    "gemini-3.5-flash-lite",
	}

	req3 := dto.DoubtRequest{
		Question: "What is Newton's third law of motion?",
		Mode:     "academic",
		Model:    "gemini-3.5-flash-lite",
	}

	hash1 := ComputeDoubtCacheKey(req1)
	hash2 := ComputeDoubtCacheKey(req2)
	hash3 := ComputeDoubtCacheKey(req3)

	if hash1 == "" || hash2 == "" || hash3 == "" {
		t.Fatalf("Expected valid non-empty hashes, got %q, %q, %q", hash1, hash2, hash3)
	}

	if hash1 == hash2 {
		t.Errorf("CRITICAL BUG: Newton's First Law and Second Law matched the same hash: %q", hash1)
	}
	if hash2 == hash3 {
		t.Errorf("CRITICAL BUG: Newton's Second Law and Third Law matched the same hash: %q", hash2)
	}
	if hash1 == hash3 {
		t.Errorf("CRITICAL BUG: Newton's First Law and Third Law matched the same hash: %q", hash1)
	}
}

func TestComputeDoubtCacheKey_ModeSeparation(t *testing.T) {
	reqAcademic := dto.DoubtRequest{
		Question: "Derive quadratic formula using completing the square",
		Mode:     "academic",
		Model:    "gemini-3.5-flash-lite",
	}

	reqFeynman := dto.DoubtRequest{
		Question: "Derive quadratic formula using completing the square",
		Mode:     "feynman",
		Model:    "gemini-3.5-flash-lite",
	}

	hashAcademic := ComputeDoubtCacheKey(reqAcademic)
	hashFeynman := ComputeDoubtCacheKey(reqFeynman)

	if hashAcademic == hashFeynman {
		t.Errorf("CRITICAL BUG: Academic mode and Feynman mode produced the same hash: %q", hashAcademic)
	}
}

func TestComputeDoubtCacheKey_SafetyBypasses(t *testing.T) {
	// Guard 1: Conversational follow-ups must NEVER be cached
	reqConversation := dto.DoubtRequest{
		Question: "Why did you use substitution in step 2?",
		ConversationHistory: []dto.ConversationTurn{
			{Role: "user", Content: "Solve integral of x e^x"},
			{Role: "assistant", Content: "Step 1..."},
		},
	}
	if key := ComputeDoubtCacheKey(reqConversation); key != "" {
		t.Errorf("Expected empty cache key for conversational follow-up, got: %q", key)
	}

	// Guard 2: Handwritten image attached
	reqImage := dto.DoubtRequest{
		Question:    "Solve question 4 from this sheet",
		ImageBase64: "data:image/jpeg;base64,12345",
	}
	if key := ComputeDoubtCacheKey(reqImage); key != "" {
		t.Errorf("Expected empty cache key for image query, got: %q", key)
	}

	// Guard 3: User private note context attached
	reqNotes := dto.DoubtRequest{
		Question:     "Explain this paragraph in depth",
		ContextNotes: "Professor said this will be on the final exam tomorrow",
	}
	if key := ComputeDoubtCacheKey(reqNotes); key != "" {
		t.Errorf("Expected empty cache key for custom notes query, got: %q", key)
	}

	// Guard 4: Short non-academic queries
	reqShort := dto.DoubtRequest{
		Question: "Hi there!",
	}
	if key := ComputeDoubtCacheKey(reqShort); key != "" {
		t.Errorf("Expected empty cache key for short non-academic query, got: %q", key)
	}
}
