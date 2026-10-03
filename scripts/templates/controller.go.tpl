package controller

import (
	"encoding/json"
	"net/http"
	"strconv"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/services"
	"goProjectStructure/internal/reuse"

	"github.com/go-chi/chi/v5"
)

type __PASCAL__Controller struct {
	service *services.__PASCAL__Service
}

func New__PASCAL__Controller(service *services.__PASCAL__Service) *__PASCAL__Controller {
	return &__PASCAL__Controller{service: service}
}

func (c *__PASCAL__Controller) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.Create__PASCAL__Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		reuse.Error(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}
	if err := reuse.ValidateStruct(req); err != nil {
		reuse.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	item, err := c.service.Create(r.Context(), req)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	reuse.Created(w, "__PASCAL__ created successfully", item)
}

func (c *__PASCAL__Controller) GetByID(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		reuse.Error(w, http.StatusBadRequest, "Invalid ID parameter")
		return
	}

	item, err := c.service.GetByID(r.Context(), id)
	if err != nil {
		reuse.Error(w, http.StatusNotFound, err.Error())
		return
	}
	reuse.Success(w, "__PASCAL__ retrieved successfully", item)
}

func (c *__PASCAL__Controller) List(w http.ResponseWriter, r *http.Request) {
	pagination := reuse.GetPagination(r)
	items, total, err := c.service.List(r.Context(), pagination)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	reuse.PaginatedResponse(w, items, total, pagination)
}

func (c *__PASCAL__Controller) Delete(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		reuse.Error(w, http.StatusBadRequest, "Invalid ID parameter")
		return
	}

	if err := c.service.Delete(r.Context(), id); err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	reuse.Message(w, http.StatusOK, "__PASCAL__ deleted successfully")
}
