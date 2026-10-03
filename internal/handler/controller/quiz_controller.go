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

type QuizController struct {
	quizService services.QuizService
}

func NewQuizController(quizService services.QuizService) *QuizController {
	return &QuizController{quizService: quizService}
}

// GenerateQuiz generates an adaptive quiz focused on student's weak areas
func (c *QuizController) GenerateQuiz(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	var req dto.GenerateQuizRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := reuse.ValidateStruct(req); err != nil {
		reuse.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	quiz, err := c.quizService.GenerateQuiz(r.Context(), claims.UserID, req)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Created(w, "adaptive quiz generated successfully", quiz)
}

// GetQuiz retrieves a quiz with its questions
func (c *QuizController) GetQuiz(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	quizID := chi.URLParam(r, "id")
	quiz, err := c.quizService.GetQuiz(r.Context(), quizID, claims.UserID)
	if err != nil {
		reuse.Error(w, http.StatusNotFound, err.Error())
		return
	}

	reuse.Success(w, "quiz retrieved successfully", quiz)
}

// ListQuizzes lists all quizzes generated for the student
func (c *QuizController) ListQuizzes(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	quizzes, err := c.quizService.ListQuizzes(r.Context(), claims.UserID)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "quizzes retrieved successfully", quizzes)
}

// SubmitQuiz evaluates answers and automatically updates the student's cognitive memory profile
func (c *QuizController) SubmitQuiz(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	quizID := chi.URLParam(r, "id")
	var req dto.SubmitQuizRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := reuse.ValidateStruct(req); err != nil {
		reuse.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	result, err := c.quizService.SubmitQuiz(r.Context(), quizID, claims.UserID, req)
	if err != nil {
		reuse.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	reuse.Success(w, "quiz submitted and cognitive profile updated", result)
}

// GetCognitiveProfile returns the student's memory profile (weak topics, exam traps, streak)
func (c *QuizController) GetCognitiveProfile(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	profile, err := c.quizService.GetCognitiveProfile(r.Context(), claims.UserID)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "cognitive profile retrieved successfully", profile)
}

// UpdateCognitiveProfile updates student goals (target exam, academic level)
func (c *QuizController) UpdateCognitiveProfile(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	var req dto.UpdateCognitiveProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := reuse.ValidateStruct(req); err != nil {
		reuse.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	profile, err := c.quizService.UpdateCognitiveProfile(r.Context(), claims.UserID, req)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "cognitive profile updated successfully", profile)
}
