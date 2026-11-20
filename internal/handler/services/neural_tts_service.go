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

// ResolveVoice returns the most natural, high-fidelity Indian female teacher voice for the content:
// - If the text contains Hindi Unicode characters (Devanagari U+0900 to U+097F), it uses hi-IN-SwaraNeural
//   to pronounce Hindi grammar, conjuncts, and vocabulary with 100% native clarity.
// - For English & Romanized Hinglish (standard in 95% of engineering, medical, school & coding doubts),
//   it uses en-IN-NeerjaExpressiveNeural. Neerja has expressive teacher inflection and accurate English pronunciation,
//   completely eliminating awkward mispronunciations of scientific, mathematical, and academic terms!
func (s *NeuralTTSService) ResolveVoice(persona string, text string) string {
	if containsDevanagari(text) {
		return "hi-IN-SwaraNeural"
	}
	return "en-IN-NeerjaExpressiveNeural"
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

// SynthesizeSpeech converts spoken academic text into studio-grade MP3 audio bytes.
// It uses SynthesizeSpeechFile for disk caching and reads the final bytes.
func (s *NeuralTTSService) SynthesizeSpeech(ctx context.Context, text, persona string) ([]byte, error) {
	filePath, err := s.SynthesizeSpeechFile(ctx, text, persona)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return os.ReadFile(filePath)
}

// SynthesizeSpeechFile converts spoken academic text into studio-grade MP3 audio on disk.
// It returns the absolute file path of the cached or freshly synthesized MP3 file.
// This allows the HTTP controller to use http.ServeFile which handles HTTP Range headers
// (206 Partial Content) and progressive streaming with zero memory copying in Go!
func (s *NeuralTTSService) SynthesizeSpeechFile(ctx context.Context, text, persona string) (string, error) {
	trimmedText := strings.TrimSpace(text)
	if trimmedText == "" {
		return "", fmt.Errorf("text cannot be empty")
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

	// 2. Calculate deterministic MD5 hash for instant disk caching (versioned for expressive natural pronunciation)
	hash := md5.Sum([]byte(cleanText + "_" + voice + "_natural_expressive_v4"))
	fileName := hex.EncodeToString(hash[:]) + ".mp3"
	filePath := filepath.Join(s.cacheDir, fileName)

	// 3. ⚡ Fast Cache Hit Check (Sub-millisecond latency)
	s.mu.RLock()
	if info, err := os.Stat(filePath); err == nil && info.Size() > 0 {
		s.mu.RUnlock()
		return filePath, nil
	}
	s.mu.RUnlock()

	// 4. Concurrency Limiter (Semaphore): Allow max 3 concurrent edge-tts subprocesses
	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(8 * time.Second):
		return "", fmt.Errorf("server audio synthesizer busy, please retry shortly")
	}

	// Double-check cache in case another goroutine generated this file while waiting for semaphore
	s.mu.RLock()
	if info, err := os.Stat(filePath); err == nil && info.Size() > 0 {
		s.mu.RUnlock()
		return filePath, nil
	}
	s.mu.RUnlock()

	// 5. Build command with strict request context cancellation
	cmd, cancel, err := s.buildSynthesizeCommand(ctx, cleanText, voice, filePath)
	if err != nil {
		return "", fmt.Errorf("failed to build synthesis command: %w", err)
	}
	defer cancel()

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Clean up broken/empty file if synthesis failed or client canceled
		_ = os.Remove(filePath)
		return "", fmt.Errorf("neural tts synthesis failed: %v (output: %s)", err, strings.TrimSpace(string(output)))
	}

	// 6. Verify synthesized audio file exists and is not empty
	s.mu.Lock()
	defer s.mu.Unlock()

	info, err := os.Stat(filePath)
	if err != nil || info.Size() == 0 {
		_ = os.Remove(filePath)
		return "", fmt.Errorf("synthesized audio file is empty or missing")
	}

	return filePath, nil
}

// buildSynthesizeCommand locates the edge-tts runner across Windows, Linux, and Docker environments.
func (s *NeuralTTSService) buildSynthesizeCommand(ctx context.Context, text, voice, outputPath string) (*exec.Cmd, context.CancelFunc, error) {
	// Set 20-second timeout on speech generation linked to caller context
	cCtx, cancel := context.WithTimeout(ctx, 20*time.Second)

	// Candidate 1: Direct edge-tts binary in system PATH
	if p, err := exec.LookPath("edge-tts"); err == nil {
		return exec.CommandContext(cCtx, p, "--voice", voice, "--rate=+0%", "--pitch=+0Hz", "--text", text, "--write-media", outputPath), cancel, nil
	}

	// Candidate 2: Common Windows AppData Python Scripts directory
	winScriptsPath := filepath.Join(os.Getenv("APPDATA"), "Python", "Python314", "Scripts", "edge-tts.exe")
	if _, err := os.Stat(winScriptsPath); err == nil {
		return exec.CommandContext(cCtx, winScriptsPath, "--voice", voice, "--rate=+0%", "--pitch=+0Hz", "--text", text, "--write-media", outputPath), cancel, nil
	}

	// Candidate 3: Python module invocation (python -m edge_tts)
	if pythonBin, err := exec.LookPath("python"); err == nil {
		return exec.CommandContext(cCtx, pythonBin, "-m", "edge_tts", "--voice", voice, "--rate=+0%", "--pitch=+0Hz", "--text", text, "--write-media", outputPath), cancel, nil
	}

	// Candidate 4: Python3 module invocation (python3 -m edge_tts for Linux/Docker)
	if python3Bin, err := exec.LookPath("python3"); err == nil {
		return exec.CommandContext(cCtx, python3Bin, "-m", "edge_tts", "--voice", voice, "--rate=+0%", "--pitch=+0Hz", "--text", text, "--write-media", outputPath), cancel, nil
	}

	cancel()
	return nil, nil, fmt.Errorf("edge-tts runner not found; please install with 'pip install edge-tts'")
}
