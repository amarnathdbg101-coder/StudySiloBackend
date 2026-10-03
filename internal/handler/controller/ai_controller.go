package controller

import (
	"encoding/json"
	"net/http"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/services"
	"goProjectStructure/internal/middleware"
	"goProjectStructure/internal/reuse"
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

	resp, err := c.aiTutorService.SolveDoubt(r.Context(), userID, req)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
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
