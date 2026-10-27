package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNeuralTTSService_VoiceResolution(t *testing.T) {
	svc := NewNeuralTTSService(t.TempDir())

	tests := []struct {
		persona  string
		text     string
		expected string
	}{
		{"priya", "Namaste", "hi-IN-SwaraNeural"},
		{"academic", "Doubt", "hi-IN-SwaraNeural"},
		{"aryan", "Complex question", "hi-IN-MadhurNeural"},
		{"professor", "PYQ question", "hi-IN-MadhurNeural"},
		{"mentor", "English explanation", "en-IN-NeerjaNeural"},
		{"mentor", "नमस्ते", "hi-IN-SwaraNeural"},
		{"unknown", "Random", "hi-IN-SwaraNeural"},
	}

	for _, tt := range tests {
		t.Run(tt.persona, func(t *testing.T) {
			got := svc.ResolveVoice(tt.persona, tt.text)
			if got != tt.expected {
				t.Errorf("ResolveVoice(%q, %q) = %q, want %q", tt.persona, tt.text, got, tt.expected)
			}
		})
	}
}

func TestNeuralTTSService_SynthesizeAndCache(t *testing.T) {
	tempDir := t.TempDir()
	svc := NewNeuralTTSService(tempDir)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	testText := "नमस्ते! यह स्टडीसाइलो का टेस्ट ऑडियो है।"
	audioBytes, err := svc.SynthesizeSpeech(ctx, testText, "priya")
	if err != nil {
		t.Fatalf("SynthesizeSpeech failed: %v", err)
	}

	if len(audioBytes) == 0 {
		t.Fatal("Expected non-empty audio bytes")
	}

	// Verify that the file was cached in tempDir
	entries, err := os.ReadDir(tempDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("Expected cached files in %s, got err: %v", tempDir, err)
	}

	cachedFilePath := filepath.Join(tempDir, entries[0].Name())
	if info, err := os.Stat(cachedFilePath); err != nil || info.Size() == 0 {
		t.Fatalf("Cached file %s is empty or invalid", cachedFilePath)
	}

	// Test instant cache hit
	cachedBytes, err := svc.SynthesizeSpeech(ctx, testText, "priya")
	if err != nil {
		t.Fatalf("Cache hit failed: %v", err)
	}

	if len(cachedBytes) != len(audioBytes) {
		t.Errorf("Cache bytes mismatch: got %d, want %d", len(cachedBytes), len(audioBytes))
	}
}
