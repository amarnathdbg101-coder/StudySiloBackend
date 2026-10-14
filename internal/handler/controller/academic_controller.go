// Package controller provides HTTP handlers for managing academic entities such as courses, folders, and notes. It defines the AcademicController struct, which contains methods for creating, retrieving, updating, and deleting these entities. The controller interacts with the AcademicService to perform business logic and uses middleware for authentication and authorization. Responses are standardized using the reuse package for consistent error handling and success messages.
package controller

import (
	"encoding/json"
	"net/http"
	"strings"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/services"
	"goProjectStructure/internal/middleware"
	"goProjectStructure/internal/reuse"

	"github.com/go-chi/chi/v5"
)

type AcademicController struct {
	academicService services.AcademicService
}

func NewAcademicController(academicService services.AcademicService) *AcademicController {
	return &AcademicController{academicService: academicService}
}

// ---------------------------------------------------------------------------
// Course Handlers
// ---------------------------------------------------------------------------

func (c *AcademicController) CreateCourse(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	var req dto.CreateCourseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := reuse.ValidateStruct(req); err != nil {
		reuse.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	course, err := c.academicService.CreateCourse(r.Context(), claims.UserID, req)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Created(w, "course created successfully", course)
}

func (c *AcademicController) ListCourses(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	courses, err := c.academicService.ListCourses(r.Context(), claims.UserID)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "courses retrieved successfully", courses)
}

func (c *AcademicController) GetCourse(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	courseID := chi.URLParam(r, "id")
	course, err := c.academicService.GetCourse(r.Context(), courseID, claims.UserID)
	if err != nil {
		reuse.Error(w, http.StatusNotFound, err.Error())
		return
	}

	reuse.Success(w, "course retrieved successfully", course)
}

func (c *AcademicController) UpdateCourse(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	courseID := chi.URLParam(r, "id")
	var req dto.UpdateCourseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := reuse.ValidateStruct(req); err != nil {
		reuse.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	course, err := c.academicService.UpdateCourse(r.Context(), courseID, claims.UserID, req)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "course updated successfully", course)
}

func (c *AcademicController) DeleteCourse(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	courseID := chi.URLParam(r, "id")
	if err := c.academicService.DeleteCourse(r.Context(), courseID, claims.UserID); err != nil {
		reuse.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	reuse.Success(w, "course deleted successfully", nil)
}

// ---------------------------------------------------------------------------
// Folder Handlers
// ---------------------------------------------------------------------------

func (c *AcademicController) CreateFolder(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	var req dto.CreateFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := reuse.ValidateStruct(req); err != nil {
		reuse.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	folder, err := c.academicService.CreateFolder(r.Context(), claims.UserID, req)
	if err != nil {
		reuse.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	reuse.Created(w, "folder created successfully", folder)
}

func (c *AcademicController) GetFolder(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	folderID := chi.URLParam(r, "id")
	folder, err := c.academicService.GetFolder(r.Context(), folderID, claims.UserID)
	if err != nil {
		reuse.Error(w, http.StatusNotFound, err.Error())
		return
	}

	reuse.Success(w, "folder retrieved successfully", folder)
}

func (c *AcademicController) GetFolderTree(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	courseID := chi.URLParam(r, "courseId")
	tree, err := c.academicService.GetFolderTree(r.Context(), courseID, claims.UserID)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "folder tree retrieved successfully", tree)
}

func (c *AcademicController) UpdateFolder(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	folderID := chi.URLParam(r, "id")
	var req dto.UpdateFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := reuse.ValidateStruct(req); err != nil {
		reuse.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	folder, err := c.academicService.UpdateFolder(r.Context(), folderID, claims.UserID, req)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "folder updated successfully", folder)
}

func (c *AcademicController) DeleteFolder(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	folderID := chi.URLParam(r, "id")
	if err := c.academicService.DeleteFolder(r.Context(), folderID, claims.UserID); err != nil {
		reuse.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	reuse.Success(w, "folder deleted successfully", nil)
}

// ---------------------------------------------------------------------------
// Note Handlers
// ---------------------------------------------------------------------------

func (c *AcademicController) CreateNote(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	var req dto.CreateNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := reuse.ValidateStruct(req); err != nil {
		reuse.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	note, err := c.academicService.CreateNote(r.Context(), claims.UserID, req)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Created(w, "note created successfully", note)
}

func (c *AcademicController) ListNotes(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	query := r.URL.Query()
	if q := strings.TrimSpace(query.Get("q")); q != "" {
		notes, err := c.academicService.SearchNotes(r.Context(), claims.UserID, q, 25)
		if err != nil {
			reuse.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		reuse.Success(w, "notes search results", notes)
		return
	} else if search := strings.TrimSpace(query.Get("search")); search != "" {
		notes, err := c.academicService.SearchNotes(r.Context(), claims.UserID, search, 25)
		if err != nil {
			reuse.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		reuse.Success(w, "notes search results", notes)
		return
	}

	var courseID *string
	if val := query.Get("course_id"); val != "" {
		courseID = &val
	}
	var folderID *string
	if val := query.Get("folder_id"); val != "" {
		folderID = &val
	}
	var tag *string
	if val := query.Get("tag"); val != "" {
		tag = &val
	}

	notes, err := c.academicService.ListNotes(r.Context(), claims.UserID, courseID, folderID, tag)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "notes retrieved successfully", notes)
}

func (c *AcademicController) GetNote(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	noteID := chi.URLParam(r, "id")
	note, err := c.academicService.GetNote(r.Context(), noteID, claims.UserID)
	if err != nil {
		reuse.Error(w, http.StatusNotFound, err.Error())
		return
	}

	reuse.Success(w, "note retrieved successfully", note)
}

func (c *AcademicController) UpdateNote(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	noteID := chi.URLParam(r, "id")
	var req dto.UpdateNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := reuse.ValidateStruct(req); err != nil {
		reuse.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	note, err := c.academicService.UpdateNote(r.Context(), noteID, claims.UserID, req)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "note updated successfully", note)
}

func (c *AcademicController) DeleteNote(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	noteID := chi.URLParam(r, "id")
	if err := c.academicService.DeleteNote(r.Context(), noteID, claims.UserID); err != nil {
		reuse.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	reuse.Success(w, "note deleted successfully", nil)
}
