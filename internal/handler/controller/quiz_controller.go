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
	userID := "guest-user"
	if claims != nil && claims.UserID != "" {
		userID = claims.UserID
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

	quiz, err := c.quizService.GenerateQuiz(r.Context(), userID, req)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "adaptive quiz generated successfully", quiz)
}

// GetQuiz retrieves a quiz with its questions
func (c *QuizController) GetQuiz(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	userID := "guest-user"
	if claims != nil && claims.UserID != "" {
		userID = claims.UserID
	}

	quizID := chi.URLParam(r, "id")
	quiz, err := c.quizService.GetQuiz(r.Context(), quizID, userID)
	if err != nil {
		reuse.Error(w, http.StatusNotFound, err.Error())
		return
	}

	reuse.Success(w, "quiz retrieved successfully", quiz)
}

// ListQuizzes lists all quizzes generated for the student
func (c *QuizController) ListQuizzes(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	userID := "guest-user"
	if claims != nil && claims.UserID != "" {
		userID = claims.UserID
	}

	quizzes, err := c.quizService.ListQuizzes(r.Context(), userID)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "quizzes retrieved successfully", quizzes)
}

// DeleteQuiz deletes a quiz by ID
func (c *QuizController) DeleteQuiz(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	userID := "guest-user"
	if claims != nil && claims.UserID != "" {
		userID = claims.UserID
	}

	quizID := chi.URLParam(r, "id")
	if err := c.quizService.DeleteQuiz(r.Context(), quizID, userID); err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "quiz deleted successfully", map[string]string{"id": quizID})
}

// ListAttempts retrieves past attempts for a quiz or for all quizzes by student
func (c *QuizController) ListAttempts(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	userID := "guest-user"
	if claims != nil && claims.UserID != "" {
		userID = claims.UserID
	}

	quizID := chi.URLParam(r, "id")
	if quizID == "" {
		quizID = r.URL.Query().Get("quiz_id")
	}

	attempts, err := c.quizService.ListAttempts(r.Context(), userID, quizID)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "quiz attempts retrieved successfully", attempts)
}


// SubmitQuiz evaluates answers and automatically updates the student's cognitive memory profile
func (c *QuizController) SubmitQuiz(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	userID := "guest-user"
	if claims != nil && claims.UserID != "" {
		userID = claims.UserID
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

	result, err := c.quizService.SubmitQuiz(r.Context(), quizID, userID, req)
	if err != nil {
		reuse.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	reuse.Success(w, "quiz submitted and cognitive profile updated", result)
}

// GetCognitiveProfile returns the student's memory profile (weak topics, exam traps, streak)
func (c *QuizController) GetCognitiveProfile(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	userID := "guest-user"
	if claims != nil && claims.UserID != "" {
		userID = claims.UserID
	}

	profile, err := c.quizService.GetCognitiveProfile(r.Context(), userID)
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
