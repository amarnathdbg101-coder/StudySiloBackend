package controller

import (
	"encoding/json"
	"net/http"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/services"
	"goProjectStructure/internal/middleware"
	"goProjectStructure/internal/reuse"
)

// CognitiveController handles HTTP requests for AI Cognitive Profile, Learning Memory, and Topic Mastery
type CognitiveController struct {
	cognitiveService services.CognitiveService
}

// NewCognitiveController initializes CognitiveController
func NewCognitiveController(cognitiveService services.CognitiveService) *CognitiveController {
	return &CognitiveController{
		cognitiveService: cognitiveService,
	}
}

// GetProfile returns the student's cognitive memory profile, readiness score, and mastery breakdown
func (c *CognitiveController) GetProfile(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	userID := "guest-user"
	if claims != nil && claims.UserID != "" {
		userID = claims.UserID
	}

	profile, err := c.cognitiveService.GetProfile(r.Context(), userID)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "cognitive profile retrieved successfully", profile)
}

// GetSmartContext returns dynamic server-synthesized smart prompts, hints, and exam syllabus recommendations
func (c *CognitiveController) GetSmartContext(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	userID := "guest-user"
	if claims != nil && claims.UserID != "" {
		userID = claims.UserID
	}

	targetExam := r.URL.Query().Get("exam")
	tutorMode := r.URL.Query().Get("mode")

	smartCtx, err := c.cognitiveService.GetSmartContext(r.Context(), userID, targetExam, tutorMode)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "smart context retrieved successfully", smartCtx)
}

// UpdateGoals updates target exam, academic level, and custom topics
func (c *CognitiveController) UpdateGoals(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	userID := "guest-user"
	if claims != nil && claims.UserID != "" {
		userID = claims.UserID
	}

	var req dto.UpdateCognitiveGoalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := reuse.ValidateStruct(req); err != nil {
		reuse.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	profile, err := c.cognitiveService.UpdateGoals(r.Context(), claims.UserID, req)
	if err != nil {
		reuse.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	reuse.Success(w, "cognitive goals updated successfully", profile)
}

// Diagnose triggers an on-demand AI diagnostic assessment
func (c *CognitiveController) Diagnose(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	userID := "guest-user"
	if claims != nil && claims.UserID != "" {
		userID = claims.UserID
	}

	diag, err := c.cognitiveService.DiagnoseProfile(r.Context(), userID)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "AI cognitive diagnosis completed", diag)
}

// PromoteTopic moves a topic from Needs Focus to Mastered
func (c *CognitiveController) PromoteTopic(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	var req dto.TopicActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := reuse.ValidateStruct(req); err != nil {
		reuse.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	profile, err := c.cognitiveService.PromoteTopic(r.Context(), claims.UserID, req.Topic)
	if err != nil {
		reuse.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	reuse.Success(w, "topic promoted to mastered", profile)
}

// DemoteTopic moves a topic from Mastered to Needs Focus
func (c *CognitiveController) DemoteTopic(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	var req dto.TopicActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := reuse.ValidateStruct(req); err != nil {
		reuse.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	profile, err := c.cognitiveService.DemoteTopic(r.Context(), claims.UserID, req.Topic)
	if err != nil {
		reuse.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	reuse.Success(w, "topic marked for focus", profile)
}

// DeleteTopic removes a topic from the student's mastery profile
func (c *CognitiveController) DeleteTopic(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	var req dto.TopicActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := reuse.ValidateStruct(req); err != nil {
		reuse.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	profile, err := c.cognitiveService.DeleteTopic(r.Context(), claims.UserID, req.Topic)
	if err != nil {
		reuse.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	reuse.Success(w, "topic deleted from profile", profile)
}

// RecordActivity logs a custom study/quiz/doubt activity
func (c *CognitiveController) RecordActivity(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	var req dto.RecordActivityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := reuse.ValidateStruct(req); err != nil {
		reuse.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	profile, err := c.cognitiveService.RecordActivity(r.Context(), claims.UserID, req)
	if err != nil {
		reuse.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	reuse.Success(w, "learning activity recorded", profile)
}
