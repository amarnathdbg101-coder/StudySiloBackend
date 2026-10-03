package controller

import (
	"net/http"
	"strconv"
	"time"

	"goProjectStructure/internal/handler/services"
	"goProjectStructure/internal/middleware"
	"goProjectStructure/internal/reuse"
)

type SyncController struct {
	syncService services.SyncService
}

func NewSyncController(syncService services.SyncService) *SyncController {
	return &SyncController{syncService: syncService}
}

// Sync performs delta synchronization for offline-first Flutter mobile clients
func (c *SyncController) Sync(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r.Context())
	if claims == nil {
		reuse.Error(w, http.StatusUnauthorized, "unauthorized access")
		return
	}

	sinceStr := r.URL.Query().Get("since")
	var since time.Time

	if sinceStr != "" {
		// Try parsing RFC3339 timestamp first
		parsed, err := time.Parse(time.RFC3339, sinceStr)
		if err == nil {
			since = parsed
		} else {
			// Try parsing unix seconds or milliseconds
			if unixVal, err := strconv.ParseInt(sinceStr, 10, 64); err == nil {
				if unixVal > 1e11 { // Milliseconds
					since = time.UnixMilli(unixVal)
				} else {
					since = time.Unix(unixVal, 0)
				}
			}
		}
	}

	result, err := c.syncService.GetDelta(r.Context(), claims.UserID, since)
	if err != nil {
		reuse.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	reuse.Success(w, "delta synchronized successfully", result)
}
