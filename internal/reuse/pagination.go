package reuse

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// PaginationParams contains extracted query parameters for paginated queries
type PaginationParams struct {
	Page    int    `json:"page"`
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
	SortBy  string `json:"sort_by"`
	Order   string `json:"order"`   // "asc" or "desc"
	Search  string `json:"search"`  // sanitized search keyword
}

// PaginationMeta contains metadata returned in paginated API responses
type PaginationMeta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}

// PaginatedEnvelope is the standard wrapper for paginated lists
type PaginatedEnvelope struct {
	Success bool           `json:"success"`
	Data    any            `json:"data"`
	Meta    PaginationMeta `json:"meta"`
}

// GetPagination parses query parameters from an HTTP request with safe defaults.
// Default: page=1, limit=10 (max limit 100), order=desc
func GetPagination(r *http.Request) PaginationParams {
	q := r.URL.Query()

	page, err := strconv.Atoi(q.Get("page"))
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(q.Get("limit"))
	if err != nil || limit < 1 {
		limit = 10
	} else if limit > 100 {
		limit = 100 // Prevent fetching excessive rows
	}

	offset := (page - 1) * limit

	order := strings.ToLower(strings.TrimSpace(q.Get("order")))
	if order != "asc" {
		order = "desc"
	}

	sortBy := strings.TrimSpace(q.Get("sort_by"))
	if sortBy == "" {
		sortBy = "created_at"
	}

	search := strings.TrimSpace(q.Get("search"))

	return PaginationParams{
		Page:   page,
		Limit:  limit,
		Offset: offset,
		SortBy: sortBy,
		Order:  order,
		Search: search,
	}
}

// BuildMeta calculates pagination metadata
func BuildMeta(totalItems int64, params PaginationParams) PaginationMeta {
	totalPages := 0
	if params.Limit > 0 {
		totalPages = int((totalItems + int64(params.Limit) - 1) / int64(params.Limit))
	}
	if totalPages == 0 {
		totalPages = 1
	}

	return PaginationMeta{
		Page:       params.Page,
		Limit:      params.Limit,
		TotalItems: totalItems,
		TotalPages: totalPages,
		HasNext:    params.Page < totalPages,
		HasPrev:    params.Page > 1,
	}
}

// PaginatedResponse sends a standardized 200 OK JSON response with pagination metadata
func PaginatedResponse(w http.ResponseWriter, items any, totalItems int64, params PaginationParams) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	meta := BuildMeta(totalItems, params)

	_ = json.NewEncoder(w).Encode(PaginatedEnvelope{
		Success: true,
		Data:    items,
		Meta:    meta,
	})
}