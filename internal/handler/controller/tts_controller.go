// Package controller handles incoming HTTP requests and delegates to appropriate services.
package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"goProjectStructure/internal/handler/services"
	"goProjectStructure/internal/reuse"
	"goProjectStructure/internal/reuse/ai"
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
	Text      string `json:"text"`
	Persona   string `json:"persona"`
	IsSummary bool   `json:"is_summary,omitempty"`
}

// Synthesize handles GET and POST /api/v1/tts/synthesize and streams studio-grade MP3 audio bytes.
func (c *TTSController) Synthesize(w http.ResponseWriter, r *http.Request) {
	var text, persona string
	var isSummary bool

	if r.Method == http.MethodGet {
		text = r.URL.Query().Get("text")
		persona = r.URL.Query().Get("persona")
		isSummary = r.URL.Query().Get("summary") == "true" || r.URL.Query().Get("is_summary") == "true"
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
		isSummary = req.IsSummary
	}

	trimmedText := strings.TrimSpace(text)
	if trimmedText == "" {
		reuse.Error(w, http.StatusBadRequest, "Field 'text' cannot be empty")
		return
	}

	// If requested as an audio summary, extract a crisp 20-30s note (~300 chars)
	if isSummary {
		trimmedText = ai.ExtractSpokenSummary(trimmedText)
	}

	// Limit single audio synthesis track to 1,500 characters to protect server CPU
	if len(trimmedText) > 1500 {
		trimmedText = trimmedText[:1500]
	}

	filePath, err := c.ttsService.SynthesizeSpeechFile(r.Context(), trimmedText, persona)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, fmt.Sprintf("Speech synthesis error: %v", err))
		return
	}

	// Stream audio response with HTTP range request support (206 Partial Content)
	// and progressive streaming with zero Go memory allocation
	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Cache-Control", "public, max-age=604800, immutable") // 7 days disk cache
	http.ServeFile(w, r, filePath)
}
