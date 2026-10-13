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
