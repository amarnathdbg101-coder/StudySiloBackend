package controller

import (
	"encoding/json"
	"log"
	"net/http"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/services"
	"goProjectStructure/internal/middleware"
	"goProjectStructure/internal/reuse"

	"github.com/go-chi/chi/v5"
)

type AIController struct {
	aiTutorService       services.AITutorService
	notesAnalyzerService services.NotesAnalyzerService
	diagramService       services.DiagramService
}

func NewAIController(
	aiTutorService services.AITutorService,
	notesAnalyzerService services.NotesAnalyzerService,
	diagramService services.DiagramService,
) *AIController {
	return &AIController{
		aiTutorService:       aiTutorService,
		notesAnalyzerService: notesAnalyzerService,
		diagramService:       diagramService,
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
		} else if headerKey := r.Header.Get("X-API-Key"); headerKey != "" {
			req.APIKey = headerKey
		}
	}

	resp, err := c.aiTutorService.SolveDoubt(r.Context(), userID, req)
	if err != nil {
		log.Printf("AI doubt generation failed: %v", err)
		reuse.JSON(w, http.StatusServiceUnavailable, reuse.Response{
			Success: false,
			Error:   "AI service is unavailable. Add or verify a Gemini API key in the AI Tutor key vault, or configure GEMINI_API_KEY on the backend, then retry.",
		})
		return
	}

	reuse.Success(w, "doubt answered successfully", resp)
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
