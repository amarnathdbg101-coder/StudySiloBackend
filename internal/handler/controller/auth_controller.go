package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/repository"
	"goProjectStructure/internal/handler/services"
	"goProjectStructure/internal/middleware"
	"goProjectStructure/internal/reuse"
)

type AuthController struct {
	service *services.AuthService
}

func NewAuthController(service *services.AuthService) *AuthController {
	return &AuthController{service: service}
}

func (c *AuthController) Register(w http.ResponseWriter, r *http.Request) {
	var input dto.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := reuse.ValidateStruct(&input); err != nil {
		reuse.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	res, err := c.service.Register(r.Context(), input)
	if err != nil {
		if errors.Is(err, repository.ErrDuplicateEmail) {
			reuse.Error(w, http.StatusConflict, "this email address is already registered")
			return
		}
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Created(w, "Account registered successfully", res)
}

func (c *AuthController) Login(w http.ResponseWriter, r *http.Request) {
	var input dto.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := reuse.ValidateStruct(&input); err != nil {
		reuse.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	res, err := c.service.Login(r.Context(), input)
	if err != nil {
		reuse.Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	reuse.Success(w, "Login successful", res)
}

func (c *AuthController) Me(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := c.service.GetProfile(r.Context(), claims.UserID)
	if err != nil {
		reuse.Error(w, http.StatusNotFound, "user profile not found")
		return
	}

	reuse.Success(w, "Profile fetched", user)
}