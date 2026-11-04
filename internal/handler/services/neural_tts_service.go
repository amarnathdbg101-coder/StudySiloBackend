// Package services provides core business domain logic for StudySilo.
package services

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"goProjectStructure/internal/reuse/ai"
)

// NeuralTTSService coordinates studio-quality neural text-to-speech generation and disk caching.
// It uses Microsoft Edge's Neural Speech Engine (hi-IN-SwaraNeural, hi-IN-MadhurNeural)
// to deliver lifelike teacher voices with zero API key cost.
type NeuralTTSService struct {
	cacheDir string
	mu       sync.RWMutex
	sem      chan struct{} // limits concurrent edge-tts subprocesses to protect server CPU
}

// NewNeuralTTSService initializes the neural speech service and ensures the cache directory exists.
func NewNeuralTTSService(cacheDir string) *NeuralTTSService {
	if cacheDir == "" {
		cacheDir = filepath.Join("uploads", "tts_cache")
	}
	_ = os.MkdirAll(cacheDir, 0755)

	return &NeuralTTSService{
		cacheDir: cacheDir,
		// Max 3 concurrent speech synthesis processes at any time
		sem: make(chan struct{}, 3),
	}
}

// ResolveVoice maps academic personas and language requirements to the optimal neural voice.
// Priya: Sweet, warm, encouraging Indian female teacher (hi-IN-SwaraNeural)
// Aryan: Deep, articulate, calm Indian male professor (hi-IN-MadhurNeural)
// Mentor: Bilingual peer tutor (en-IN-NeerjaNeural / hi-IN-SwaraNeural)
func (s *NeuralTTSService) ResolveVoice(persona string, text string) string {
	clean := strings.ToLower(strings.TrimSpace(persona))
	switch clean {
	case "aryan", "professor", "pyq", "pyqstrategist", "derivation":
		return "hi-IN-MadhurNeural"
	case "mentor", "socratic":
		// If text contains Devanagari script, use Hindi voice; else Indian English
		if containsDevanagari(text) {
			return "hi-IN-SwaraNeural"
		}
		return "en-IN-NeerjaNeural"
	case "priya", "feynman", "academic", "traphunter":
		fallthrough
	default:
		return "hi-IN-SwaraNeural"
	}
}

// containsDevanagari checks if the text contains Hindi Unicode characters (U+0900 to U+097F).
func containsDevanagari(text string) bool {
	for _, r := range text {
		if r >= 0x0900 && r <= 0x097F {
			return true
		}
	}
	return false
}

// SynthesizeSpeech converts spoken academic text into studio-grade MP3 audio.
// It checks local cache first for sub-millisecond replay latency.
func (s *NeuralTTSService) SynthesizeSpeech(ctx context.Context, text, persona string) ([]byte, error) {
	trimmedText := strings.TrimSpace(text)
	if trimmedText == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	// 1. Sanitize raw LaTeX, math equations, scientific notation, and markdown into natural human prose
	cleanText := ai.GenerateSpokenTranscript(trimmedText)
	if cleanText == "" {
		cleanText = trimmedText
	}

	// Limit single audio synthesis track to 1,500 characters to prevent server CPU choke
	if len(cleanText) > 1500 {
		cleanText = cleanText[:1500]
	}

	voice := s.ResolveVoice(persona, cleanText)

	// 2. Calculate deterministic MD5 hash for instant disk caching
	hash := md5.Sum([]byte(cleanText + "_" + voice))
	fileName := hex.EncodeToString(hash[:]) + ".mp3"
	filePath := filepath.Join(s.cacheDir, fileName)

	// 3. ⚡ Fast Cache Hit Check (Sub-millisecond latency)
	s.mu.RLock()
	if info, err := os.Stat(filePath); err == nil && info.Size() > 0 {
		data, err := os.ReadFile(filePath)
		s.mu.RUnlock()
		if err == nil && len(data) > 0 {
			return data, nil
		}
	} else {
		s.mu.RUnlock()
	}

	// 4. Concurrency Limiter (Semaphore): Allow max 3 concurrent edge-tts subprocesses
	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(8 * time.Second):
		return nil, fmt.Errorf("server audio synthesizer busy, please retry shortly")
	}

	// Double-check cache in case another goroutine generated this file while waiting for semaphore
	s.mu.RLock()
	if info, err := os.Stat(filePath); err == nil && info.Size() > 0 {
		data, err := os.ReadFile(filePath)
		s.mu.RUnlock()
		if err == nil && len(data) > 0 {
			return data, nil
		}
	} else {
		s.mu.RUnlock()
	}

	// 5. Build command with strict request context cancellation
	cmd, cancel, err := s.buildSynthesizeCommand(ctx, cleanText, voice, filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to build synthesis command: %w", err)
	}
	defer cancel()

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Clean up broken/empty file if synthesis failed or client canceled
		_ = os.Remove(filePath)
		return nil, fmt.Errorf("neural tts synthesis failed: %v (output: %s)", err, strings.TrimSpace(string(output)))
	}

	// 6. Safely read and return the synthesized audio bytes
	s.mu.Lock()
	defer s.mu.Unlock()

	audioBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read synthesized audio file: %w", err)
	}

	if len(audioBytes) == 0 {
		_ = os.Remove(filePath)
		return nil, fmt.Errorf("synthesized audio file is empty")
	}

	return audioBytes, nil
}

// buildSynthesizeCommand locates the edge-tts runner across Windows, Linux, and Docker environments.
func (s *NeuralTTSService) buildSynthesizeCommand(ctx context.Context, text, voice, outputPath string) (*exec.Cmd, context.CancelFunc, error) {
	// Set 20-second timeout on speech generation linked to caller context
	cCtx, cancel := context.WithTimeout(ctx, 20*time.Second)

	// Candidate 1: Direct edge-tts binary in system PATH
	if p, err := exec.LookPath("edge-tts"); err == nil {
		return exec.CommandContext(cCtx, p, "--voice", voice, "--text", text, "--write-media", outputPath), cancel, nil
	}

	// Candidate 2: Common Windows AppData Python Scripts directory
	winScriptsPath := filepath.Join(os.Getenv("APPDATA"), "Python", "Python314", "Scripts", "edge-tts.exe")
	if _, err := os.Stat(winScriptsPath); err == nil {
		return exec.CommandContext(cCtx, winScriptsPath, "--voice", voice, "--text", text, "--write-media", outputPath), cancel, nil
	}

	// Candidate 3: Python module invocation (python -m edge_tts)
	if pythonBin, err := exec.LookPath("python"); err == nil {
		return exec.CommandContext(cCtx, pythonBin, "-m", "edge_tts", "--voice", voice, "--text", text, "--write-media", outputPath), cancel, nil
	}

	// Candidate 4: Python3 module invocation (python3 -m edge_tts for Linux/Docker)
	if python3Bin, err := exec.LookPath("python3"); err == nil {
		return exec.CommandContext(cCtx, python3Bin, "-m", "edge_tts", "--voice", voice, "--text", text, "--write-media", outputPath), cancel, nil
	}

	cancel()
	return nil, nil, fmt.Errorf("edge-tts runner not found; please install with 'pip install edge-tts'")
}
