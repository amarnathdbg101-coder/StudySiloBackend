// Package controller handles incoming HTTP requests and delegates to appropriate services.
package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"goProjectStructure/internal/handler/services"
	"goProjectStructure/internal/reuse"
)

// TTSController coordinates HTTP requests for speech synthesis.
type TTSController struct {
	ttsService *services.NeuralTTSService
}

// NewTTSController creates a new TTSController instance.
func NewTTSController(ttsService *services.NeuralTTSService) *TTSController {
	return &TTSController{ttsService: ttsService}
}

// SynthesizeRequest payload for speech generation.
type SynthesizeRequest struct {
	Text    string `json:"text"`
	Persona string `json:"persona"`
}

// Synthesize handles GET and POST /api/v1/tts/synthesize and streams studio-grade MP3 audio bytes.
func (c *TTSController) Synthesize(w http.ResponseWriter, r *http.Request) {
	var text, persona string

	if r.Method == http.MethodGet {
		text = r.URL.Query().Get("text")
		persona = r.URL.Query().Get("persona")
	} else {
		// Enforce 1MB max body size to protect against abusive payloads
		r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
		var req SynthesizeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			reuse.Error(w, http.StatusBadRequest, "Invalid JSON payload in request body")
			return
		}
		text = req.Text
		persona = req.Persona
	}

	trimmedText := strings.TrimSpace(text)
	if trimmedText == "" {
		reuse.Error(w, http.StatusBadRequest, "Field 'text' cannot be empty")
		return
	}

	// Limit single chunk to 1,500 characters for optimal prosody and latency
	if len(trimmedText) > 1500 {
		trimmedText = trimmedText[:1500]
	}

	audioBytes, err := c.ttsService.SynthesizeSpeech(r.Context(), trimmedText, persona)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, fmt.Sprintf("Speech synthesis error: %v", err))
		return
	}

	// Stream audio response with HTTP caching headers
	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(audioBytes)))
	w.Header().Set("Cache-Control", "public, max-age=604800, immutable") // 7 days disk cache
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(audioBytes)
}
