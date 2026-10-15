package controller

import (
	"encoding/json"
	"net/http"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/services"
	"goProjectStructure/internal/middleware"
	"goProjectStructure/internal/reuse"

	"github.com/go-chi/chi/v5"
)

type StudyPlanController struct {
	service services.StudyPlanService
}

func NewStudyPlanController(service services.StudyPlanService) *StudyPlanController {
	return &StudyPlanController{service: service}
}

func (c *StudyPlanController) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	var req dto.CreateStudyPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := reuse.ValidateStruct(req); err != nil {
		reuse.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	plan, err := c.service.CreateStudyPlan(r.Context(), claims.UserID, req)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Created(w, "study goal task created successfully", plan)
}

func (c *StudyPlanController) List(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	plans, err := c.service.ListStudyPlans(r.Context(), claims.UserID)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "study goals retrieved successfully", plans)
}

func (c *StudyPlanController) Update(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	planID := chi.URLParam(r, "id")
	if planID == "" {
		reuse.Error(w, http.StatusBadRequest, "plan id required")
		return
	}

	var req dto.UpdateStudyPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	plan, err := c.service.UpdateStudyPlan(r.Context(), planID, claims.UserID, req)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "study goal updated successfully", plan)
}

func (c *StudyPlanController) Delete(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	planID := chi.URLParam(r, "id")
	if planID == "" {
		reuse.Error(w, http.StatusBadRequest, "plan id required")
		return
	}

	if err := c.service.DeleteStudyPlan(r.Context(), planID, claims.UserID); err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "study goal deleted successfully", nil)
}

func (c *StudyPlanController) RecordStudyTime(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	var req dto.RecordStudyTimeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := reuse.ValidateStruct(req); err != nil {
		reuse.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	profile, err := c.service.RecordStudyTime(r.Context(), claims.UserID, req)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "study focus minutes and streak recorded successfully", profile)
}

func (c *StudyPlanController) GenerateAIPlan(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	userID := "guest-user"
	if claims != nil && claims.UserID != "" {
		userID = claims.UserID
	}

	var req dto.GenerateAIStudyPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := reuse.ValidateStruct(req); err != nil {
		reuse.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	plan, err := c.service.GenerateAIStudyPlan(r.Context(), userID, req)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "AI study roadmap generated successfully", plan)
}

// ─── AI Roadmap CRUD Handlers ──────────────────────────────────────────────

// SaveRoadmap - generated roadmap ko DB mein save karo (POST /roadmaps)
func (c *StudyPlanController) SaveRoadmap(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	var req dto.SaveRoadmapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := reuse.ValidateStruct(req); err != nil {
		reuse.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	roadmap, err := c.service.SaveRoadmap(r.Context(), claims.UserID, req)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Created(w, "AI roadmap saved to database successfully", roadmap)
}

// GetRoadmap - ek roadmap fetch karo by ID (GET /roadmaps/{id})
func (c *StudyPlanController) GetRoadmap(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	roadmapID := chi.URLParam(r, "id")
	if roadmapID == "" {
		reuse.Error(w, http.StatusBadRequest, "roadmap id required")
		return
	}

	roadmap, err := c.service.GetRoadmap(r.Context(), roadmapID, claims.UserID)
	if err != nil {
		reuse.Error(w, http.StatusNotFound, err.Error())
		return
	}

	reuse.Success(w, "roadmap retrieved successfully", roadmap)
}

// ListRoadmaps - user ke saare saved roadmaps (GET /roadmaps)
func (c *StudyPlanController) ListRoadmaps(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	roadmaps, err := c.service.ListRoadmaps(r.Context(), claims.UserID)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "roadmaps retrieved successfully", roadmaps)
}

// UpdateRoadmap - existing roadmap update karo (PATCH /roadmaps/{id})
func (c *StudyPlanController) UpdateRoadmap(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	roadmapID := chi.URLParam(r, "id")
	if roadmapID == "" {
		reuse.Error(w, http.StatusBadRequest, "roadmap id required")
		return
	}

	var req dto.UpdateRoadmapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	roadmap, err := c.service.UpdateRoadmap(r.Context(), roadmapID, claims.UserID, req)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "roadmap updated successfully", roadmap)
}

// DeleteRoadmap - roadmap delete karo (DELETE /roadmaps/{id})
func (c *StudyPlanController) DeleteRoadmap(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	roadmapID := chi.URLParam(r, "id")
	if roadmapID == "" {
		reuse.Error(w, http.StatusBadRequest, "roadmap id required")
		return
	}

	if err := c.service.DeleteRoadmap(r.Context(), roadmapID, claims.UserID); err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "roadmap deleted successfully", map[string]string{"id": roadmapID})
}

