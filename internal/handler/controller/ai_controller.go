package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/repository"
	"goProjectStructure/internal/handler/services"
	"goProjectStructure/internal/middleware"
	"goProjectStructure/internal/reuse"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type AIController struct {
	aiTutorService       services.AITutorService
	notesAnalyzerService services.NotesAnalyzerService
	diagramService       services.DiagramService
	flashcardService     services.FlashcardService
}

func NewAIController(
	aiTutorService services.AITutorService,
	notesAnalyzerService services.NotesAnalyzerService,
	diagramService services.DiagramService,
	flashcardService services.FlashcardService,
) *AIController {
	return &AIController{
		aiTutorService:       aiTutorService,
		notesAnalyzerService: notesAnalyzerService,
		diagramService:       diagramService,
		flashcardService:     flashcardService,
	}
}

// SolveDoubt handles step-by-step multi-modal doubt resolution
func (c *AIController) SolveDoubt(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	userID := ""
	if claims != nil {
		userID = claims.UserID
	}

	var req dto.DoubtRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := reuse.ValidateStruct(req); err != nil {
		reuse.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	if req.APIKey == "" {
		if headerKey := r.Header.Get("X-Gemini-API-Key"); headerKey != "" {
			req.APIKey = headerKey
		} else if headerKey := r.Header.Get("X-Google-API-Key"); headerKey != "" {
			req.APIKey = headerKey
		} else if headerKey := r.Header.Get("X-OpenAI-API-Key"); headerKey != "" {
			req.APIKey = headerKey
		} else if headerKey := r.Header.Get("X-API-Key"); headerKey != "" {
			req.APIKey = headerKey
		}
	}

	resp, err := c.aiTutorService.SolveDoubt(r.Context(), userID, req)
	if err != nil {
		log.Printf("AI doubt generation failed: %v", err)
		code, userMsg := parseAIError(err, req.APIKey)
		reuse.ErrorWithCode(w, http.StatusServiceUnavailable, code, userMsg)
		return
	}

	reuse.Success(w, "doubt answered successfully", resp)
}

type aiTutorStreamService interface {
	SolveDoubtStream(ctx context.Context, userID string, req dto.DoubtRequest, onChunk func(string)) (*dto.DoubtResponse, error)
}

type aiTutorStreamEvent struct {
	Type               string   `json:"type"`
	Text               string   `json:"text,omitempty"`
	Message            string   `json:"message,omitempty"`
	ErrorCode          string   `json:"error_code,omitempty"`
	SessionID          string   `json:"session_id,omitempty"`
	SessionTitle       string   `json:"session_title,omitempty"`
	AssistantMessageID string   `json:"assistant_message_id,omitempty"`
	FollowUpChips      []string `json:"follow_up_chips,omitempty"`
	SpokenTranscript   string   `json:"spoken_transcript,omitempty"`
}

func (c *AIController) SolveDoubtStream(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	userID := ""
	if claims != nil {
		userID = claims.UserID
	}

	var req dto.DoubtRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := reuse.ValidateStruct(req); err != nil {
		reuse.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if req.APIKey == "" {
		if headerKey := r.Header.Get("X-Gemini-API-Key"); headerKey != "" {
			req.APIKey = headerKey
		} else if headerKey := r.Header.Get("X-Google-API-Key"); headerKey != "" {
			req.APIKey = headerKey
		} else if headerKey := r.Header.Get("X-OpenAI-API-Key"); headerKey != "" {
			req.APIKey = headerKey
		} else if headerKey := r.Header.Get("X-API-Key"); headerKey != "" {
			req.APIKey = headerKey
		}
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		reuse.Error(w, http.StatusInternalServerError, "streaming is not supported by this server")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	// Concurrency protection: Mutex ensures heartbeat ticker and stream tokens
	// do not write to ResponseWriter concurrently (prevents Go data races).
	var writeMu sync.Mutex

	writeRaw := func(format string, a ...interface{}) {
		writeMu.Lock()
		defer writeMu.Unlock()
		_, _ = fmt.Fprintf(w, format, a...)
		flusher.Flush()
	}

	writeEvent := func(event aiTutorStreamEvent) {
		payload, err := json.Marshal(event)
		if err != nil {
			return
		}
		writeRaw("data: %s\n\n", payload)
	}

	// Active Heartbeat Ticker: Sends an SSE comment every 5 seconds.
	// W3C SSE standard: Lines starting with ":" are comments and ignored by clients.
	// This prevents Cloudflare, Render, and Mobile network timeouts during long generations.
	heartbeatDone := make(chan struct{})
	defer close(heartbeatDone)

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-heartbeatDone:
				return
			case <-ticker.C:
				writeRaw(": keep-alive\n\n")
			}
		}
	}()

	var response *dto.DoubtResponse
	var err error
	if streamService, supportsStreaming := c.aiTutorService.(aiTutorStreamService); supportsStreaming {
		response, err = streamService.SolveDoubtStream(r.Context(), userID, req, func(text string) {
			writeEvent(aiTutorStreamEvent{Type: "chunk", Text: text})
		})
	} else {
		response, err = c.aiTutorService.SolveDoubt(r.Context(), userID, req)
		if err == nil && response != nil {
			writeEvent(aiTutorStreamEvent{Type: "chunk", Text: response.SolutionMarkdown})
		}
	}
	if err != nil {
		log.Printf("AI doubt streaming failed: %v", err)
		code, userMsg := parseAIError(err, req.APIKey)
		writeEvent(aiTutorStreamEvent{Type: "error", Message: userMsg, ErrorCode: code})
		return
	}
	writeEvent(aiTutorStreamEvent{
		Type:               "done",
		SessionID:          response.SessionID,
		SessionTitle:       response.SessionTitle,
		AssistantMessageID: response.AssistantMessageID,
		FollowUpChips:      response.FollowUpChips,
		SpokenTranscript:   response.SpokenTranscript,
	})
}

// parseAIError categorizes AI errors into machine-readable error codes and safe student messages
func parseAIError(err error, apiKey string) (string, string) {
	if err == nil {
		return "UNKNOWN_ERROR", "Unknown AI error occurred."
	}
	msg := strings.TrimSpace(err.Error())

	if apiKey != "" {
		msg = strings.ReplaceAll(msg, apiKey, "[redacted]")
	}

	lower := strings.ToLower(msg)

	if strings.Contains(lower, "api_key_invalid") || strings.Contains(lower, "api key not valid") || strings.Contains(lower, "invalid_api_key") || strings.Contains(lower, "incorrect api key") {
		return "AI_KEY_INVALID", "Invalid API Key: The API key provided is invalid or inactive. Please check your Gemini or OpenAI API key in settings or .env file."
	}

	if strings.Contains(lower, "resource_exhausted") || strings.Contains(lower, "rate limit") || strings.Contains(lower, "429") || strings.Contains(lower, "quota") {
		return "AI_QUOTA_EXCEEDED", "Quota / Rate Limit Exceeded: Your API key has exceeded its rate limit or free quota. Please wait 1 minute or use a new key."
	}

	if strings.Contains(lower, "no configured api key") || strings.Contains(lower, "no keys configured") {
		return "AI_NO_KEY_CONFIGURED", "No API Key Configured: Neither GEMINI_API_KEYS nor OPENAI_API_KEYS are set in server environment, and no custom API key was passed in request."
	}

	if strings.Contains(lower, "not found") || strings.Contains(lower, "404") {
		return "AI_MODEL_NOT_FOUND", fmt.Sprintf("AI Model Not Found (404): The requested AI model ID was not found on provider API. Details: %s", truncateError(msg, 300))
	}

	return "AI_GENERATION_FAILED", "AI Generation Error: " + truncateError(msg, 400)
}

func publicAIError(err error, apiKey string) string {
	_, msg := parseAIError(err, apiKey)
	return msg
}

func truncateError(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

// GetChatHistory returns multi-modal conversation history
func (c *AIController) GetChatHistory(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	scopeType := r.URL.Query().Get("scope_type")
	scopeID := r.URL.Query().Get("scope_id")

	messages, err := c.aiTutorService.GetChatHistory(r.Context(), claims.UserID, scopeType, scopeID)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "chat history retrieved successfully", messages)
}

// ListChatSessions returns all conversations for the student (ChatGPT-style)
func (c *AIController) ListChatSessions(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	sessions, err := c.aiTutorService.ListSessions(r.Context(), claims.UserID)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "chat sessions retrieved successfully", sessions)
}

// CreateChatSession explicitly creates a new conversation thread
func (c *AIController) CreateChatSession(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	var req dto.CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	session, err := c.aiTutorService.CreateSession(r.Context(), claims.UserID, req.Title)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "chat session created successfully", session)
}

// GetChatSessionMessages retrieves messages belonging to a specific session
func (c *AIController) GetChatSessionMessages(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		reuse.Error(w, http.StatusBadRequest, "session id required")
		return
	}

	messages, err := c.aiTutorService.GetSessionMessages(r.Context(), claims.UserID, sessionID)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "session messages retrieved successfully", messages)
}

// UpdateChatSession renames a conversation thread
func (c *AIController) UpdateChatSession(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		reuse.Error(w, http.StatusBadRequest, "session id required")
		return
	}

	var req dto.UpdateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := c.aiTutorService.UpdateSession(r.Context(), claims.UserID, sessionID, req.Title); err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "chat session updated successfully", nil)
}

// DeleteChatSession deletes a conversation thread and its messages
func (c *AIController) DeleteChatSession(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		reuse.Error(w, http.StatusBadRequest, "session id required")
		return
	}

	if err := c.aiTutorService.DeleteSession(r.Context(), claims.UserID, sessionID); err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "chat session deleted successfully", nil)
}

// DeleteChatTurn deletes a user question and its assistant response.
func (c *AIController) DeleteChatTurn(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	sessionID := chi.URLParam(r, "id")
	messageID := chi.URLParam(r, "messageID")
	if sessionID == "" || messageID == "" {
		reuse.Error(w, http.StatusBadRequest, "session and message ids are required")
		return
	}

	if err := c.aiTutorService.DeleteChatTurn(r.Context(), claims.UserID, sessionID, messageID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			reuse.Error(w, http.StatusNotFound, "chat turn not found")
			return
		}
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "chat turn deleted successfully", nil)
}

// AnalyzeNotes converts lecture notes into summaries, flashcards, and cheat sheets
func (c *AIController) AnalyzeNotes(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	var req dto.NotesAnalysisRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := c.notesAnalyzerService.AnalyzeNotes(r.Context(), claims.UserID, req)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "notes analyzed successfully", resp)
}

// GenerateDiagram generates visual mind maps and flowcharts for Flutter CustomPainter
func (c *AIController) GenerateDiagram(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	userID := ""
	if claims != nil {
		userID = claims.UserID
	}

	var req dto.DiagramRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := reuse.ValidateStruct(req); err != nil {
		reuse.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	resp, err := c.diagramService.GenerateDiagram(r.Context(), userID, req)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "diagram generated successfully", resp)
}

// ExpandMindMapNode generates deep-dive sub-branches for a specific node in an interactive canvas
func (c *AIController) ExpandMindMapNode(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	userID := ""
	if claims != nil {
		userID = claims.UserID
	}

	var req dto.ExpandNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := reuse.ValidateStruct(req); err != nil {
		reuse.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	resp, err := c.diagramService.ExpandNode(r.Context(), userID, req)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "node expanded successfully", resp)
}

// GenerateQuizFromMindMap converts mind map nodes and exam traps into an active recall quiz
func (c *AIController) GenerateQuizFromMindMap(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	userID := ""
	if claims != nil {
		userID = claims.UserID
	}

	var req dto.MindMapToQuizRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := reuse.ValidateStruct(req); err != nil {
		reuse.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	quiz, err := c.diagramService.GenerateQuizFromMindMap(r.Context(), userID, req)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "quiz generated successfully from mind map", quiz)
}

// GenerateFlashcardsFromMindMap converts mind map nodes into active recall spaced repetition flashcards
func (c *AIController) GenerateFlashcardsFromMindMap(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	userID := ""
	if claims != nil {
		userID = claims.UserID
	}

	var req dto.MindMapToFlashcardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := reuse.ValidateStruct(req); err != nil {
		reuse.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	deck, err := c.diagramService.GenerateFlashcardsFromMindMap(r.Context(), userID, req)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "flashcards generated successfully from mind map", deck)
}

// ExportMindMapPDF exports the mind map as a high-resolution printable vector PDF poster
func (c *AIController) ExportMindMapPDF(w http.ResponseWriter, r *http.Request) {
	var req dto.ExportMindMapPDFRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Nodes) == 0 {
		reuse.Error(w, http.StatusUnprocessableEntity, "mind map must contain at least one node to export")
		return
	}

	pdfBytes, err := c.diagramService.ExportMindMapPDF(r.Context(), req)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, fmt.Sprintf("failed to export mind map PDF: %v", err))
		return
	}

	safeTitle := strings.ReplaceAll(req.Title, " ", "_")
	if safeTitle == "" {
		safeTitle = "StudySilo_MindMap"
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s_Poster.pdf\"", safeTitle))
	w.Header().Set("Content-Length", strconv.Itoa(len(pdfBytes)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdfBytes)
}

// ExportMindMapPDFByID loads a saved mind map by ID and exports it as a printable PDF poster
func (c *AIController) ExportMindMapPDFByID(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		reuse.Error(w, http.StatusBadRequest, "missing mind map id")
		return
	}

	mindMap, err := c.diagramService.GetMindMap(r.Context(), claims.UserID, id)
	if err != nil {
		reuse.Error(w, http.StatusNotFound, "mind map not found")
		return
	}

	var data dto.DiagramResponse
	if err := json.Unmarshal(mindMap.Data, &data); err != nil {
		reuse.Error(w, http.StatusInternalServerError, "failed to parse saved mind map data")
		return
	}

	paperSize := r.URL.Query().Get("paper_size")
	if paperSize == "" {
		paperSize = "A4"
	}

	req := dto.ExportMindMapPDFRequest{
		Title:     mindMap.Title,
		Concept:   mindMap.Concept,
		Archetype: data.Archetype,
		Summary:   data.Summary,
		Nodes:     data.Nodes,
		Edges:     data.Edges,
		PaperSize: paperSize,
	}

	pdfBytes, err := c.diagramService.ExportMindMapPDF(r.Context(), req)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, fmt.Sprintf("failed to export mind map PDF: %v", err))
		return
	}

	safeTitle := strings.ReplaceAll(mindMap.Title, " ", "_")
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s_Poster.pdf\"", safeTitle))
	w.Header().Set("Content-Length", strconv.Itoa(len(pdfBytes)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdfBytes)
}

// ReviewFlashcard calculates the next Spaced Repetition interval using SuperMemo SM-2 algorithm
func (c *AIController) ReviewFlashcard(w http.ResponseWriter, r *http.Request) {
	var req dto.FlashcardReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := reuse.ValidateStruct(req); err != nil {
		reuse.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	item := reuse.SM2Item{
		Repetitions: req.Repetitions,
		Interval:    req.Interval,
		EaseFactor:  req.EaseFactor,
	}

	nextSM2 := reuse.CalculateSM2(item, req.Quality)

	resp := dto.FlashcardReviewResponse{
		CardID:      req.CardID,
		NextReview:  nextSM2.NextReview.Format(time.RFC3339),
		Interval:    nextSM2.Interval,
		EaseFactor:  nextSM2.EaseFactor,
		Repetitions: nextSM2.Repetitions,
	}

	reuse.Success(w, "flashcard spaced repetition review calculated", resp)
}

// GenerateFlashcardDeck generates an Anki-compatible Active Recall Flashcard deck from a topic
func (c *AIController) GenerateFlashcardDeck(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	userID := "guest-user"
	if claims != nil && claims.UserID != "" {
		userID = claims.UserID
	}

	var req dto.GenerateFlashcardDeckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := reuse.ValidateStruct(req); err != nil {
		reuse.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	resp, err := c.flashcardService.GenerateDeck(r.Context(), userID, req)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "active recall flashcard deck generated successfully", resp)
}

// SaveFlashcardDeck saves a flashcard deck into PostgreSQL
func (c *AIController) SaveFlashcardDeck(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	var req dto.SaveFlashcardDeckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := reuse.ValidateStruct(req); err != nil {
		reuse.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	resp, err := c.flashcardService.SaveDeck(r.Context(), claims.UserID, req)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "flashcard deck saved successfully", resp)
}

// ListFlashcardDecks returns all saved flashcard decks for the authenticated user
func (c *AIController) ListFlashcardDecks(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	search := r.URL.Query().Get("search")
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	decks, total, err := c.flashcardService.ListDecks(r.Context(), claims.UserID, search, limit, offset)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "flashcard decks retrieved successfully", map[string]interface{}{
		"items":  decks,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetFlashcardDeck retrieves a specific saved flashcard deck by its ID
func (c *AIController) GetFlashcardDeck(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		reuse.Error(w, http.StatusBadRequest, "flashcard deck id required")
		return
	}

	resp, err := c.flashcardService.GetDeck(r.Context(), claims.UserID, id)
	if err != nil {
		if errors.Is(err, repository.ErrFlashcardDeckNotFound) {
			reuse.Error(w, http.StatusNotFound, "flashcard deck not found")
			return
		}
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "flashcard deck retrieved successfully", resp)
}

// UpdateFlashcardDeck updates cards review status, title, or pin status of a saved deck
func (c *AIController) UpdateFlashcardDeck(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		reuse.Error(w, http.StatusBadRequest, "flashcard deck id required")
		return
	}

	var req dto.UpdateFlashcardDeckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := c.flashcardService.UpdateDeck(r.Context(), claims.UserID, id, req)
	if err != nil {
		if errors.Is(err, repository.ErrFlashcardDeckNotFound) {
			reuse.Error(w, http.StatusNotFound, "flashcard deck not found")
			return
		}
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "flashcard deck updated successfully", resp)
}

// DeleteFlashcardDeck removes a saved flashcard deck from database
func (c *AIController) DeleteFlashcardDeck(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		reuse.Error(w, http.StatusBadRequest, "flashcard deck id required")
		return
	}

	if err := c.flashcardService.DeleteDeck(r.Context(), claims.UserID, id); err != nil {
		if errors.Is(err, repository.ErrFlashcardDeckNotFound) {
			reuse.Error(w, http.StatusNotFound, "flashcard deck not found")
			return
		}
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "flashcard deck deleted successfully", map[string]string{"id": id})
}

// TogglePinFlashcardDeck toggles the pinned status of a flashcard deck
func (c *AIController) TogglePinFlashcardDeck(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		reuse.Error(w, http.StatusBadRequest, "flashcard deck id required")
		return
	}

	resp, err := c.flashcardService.TogglePin(r.Context(), claims.UserID, id)
	if err != nil {
		if errors.Is(err, repository.ErrFlashcardDeckNotFound) {
			reuse.Error(w, http.StatusNotFound, "flashcard deck not found")
			return
		}
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "flashcard deck pin toggled successfully", resp)
}

// SaveMindMap saves a generated or customized visual mind map into PostgreSQL
func (c *AIController) SaveMindMap(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	var req dto.SaveMindMapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := reuse.ValidateStruct(req); err != nil {
		reuse.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	resp, err := c.diagramService.SaveMindMap(r.Context(), claims.UserID, req)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "mind map saved successfully", resp)
}

// ListMindMaps returns all saved mind maps for the authenticated user
func (c *AIController) ListMindMaps(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	search := r.URL.Query().Get("search")
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	maps, total, err := c.diagramService.ListMindMaps(r.Context(), claims.UserID, search, limit, offset)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "mind maps retrieved successfully", map[string]interface{}{
		"items":  maps,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetMindMap retrieves a specific saved mind map by its ID
func (c *AIController) GetMindMap(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		reuse.Error(w, http.StatusBadRequest, "mind map id required")
		return
	}

	resp, err := c.diagramService.GetMindMap(r.Context(), claims.UserID, id)
	if err != nil {
		if errors.Is(err, repository.ErrMindMapNotFound) {
			reuse.Error(w, http.StatusNotFound, "mind map not found")
			return
		}
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "mind map retrieved successfully", resp)
}

// UpdateMindMap updates title, canvas data, or pin status of a saved mind map
func (c *AIController) UpdateMindMap(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		reuse.Error(w, http.StatusBadRequest, "mind map id required")
		return
	}

	var req dto.UpdateMindMapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := reuse.ValidateStruct(req); err != nil {
		reuse.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	resp, err := c.diagramService.UpdateMindMap(r.Context(), claims.UserID, id, req)
	if err != nil {
		if errors.Is(err, repository.ErrMindMapNotFound) {
			reuse.Error(w, http.StatusNotFound, "mind map not found")
			return
		}
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "mind map updated successfully", resp)
}

// DeleteMindMap deletes a saved mind map
func (c *AIController) DeleteMindMap(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		reuse.Error(w, http.StatusBadRequest, "mind map id required")
		return
	}

	if err := c.diagramService.DeleteMindMap(r.Context(), claims.UserID, id); err != nil {
		if errors.Is(err, repository.ErrMindMapNotFound) {
			reuse.Error(w, http.StatusNotFound, "mind map not found")
			return
		}
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "mind map deleted successfully", nil)
}

// TogglePinMindMap toggles pinned state for quick access
func (c *AIController) TogglePinMindMap(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		reuse.Error(w, http.StatusBadRequest, "mind map id required")
		return
	}

	resp, err := c.diagramService.TogglePinMindMap(r.Context(), claims.UserID, id)
	if err != nil {
		if errors.Is(err, repository.ErrMindMapNotFound) {
			reuse.Error(w, http.StatusNotFound, "mind map not found")
			return
		}
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "mind map pin toggled successfully", resp)
}

