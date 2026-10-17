package ai

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestStreamChunkBuffer_SplitsMultibyteCleanly(t *testing.T) {
	var collected []string
	buf := NewStreamChunkBuffer(func(chunk string) {
		collected = append(collected, chunk)
	})

	// "नमस्ते" is in Devanagari script (multi-byte UTF-8)
	// Let's deliberately feed bytes in fragments:
	fullText := "नमस्ते θ ≈ ∫ x dx"
	raw := []byte(fullText)

	// Feed 2 bytes at a time
	for i := 0; i < len(raw); i += 2 {
		end := i + 2
		if end > len(raw) {
			end = len(raw)
		}
		buf.Write(string(raw[i:end]))
	}
	buf.Flush()

	// Every chunk flushed must be 100% valid UTF-8
	for i, c := range collected {
		if !utf8.ValidString(c) {
			t.Fatalf("chunk %d is not valid UTF-8: %q", i, c)
		}
	}

	reconstructed := strings.Join(collected, "")
	if reconstructed != fullText {
		t.Fatalf("expected reconstructed %q, got %q", fullText, reconstructed)
	}
}
