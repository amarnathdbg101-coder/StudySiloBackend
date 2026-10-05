package controller

import (
	"context"
	"net/http"
	"time"

	"goProjectStructure/internal/reuse"

	"github.com/jackc/pgx/v5/pgxpool"
)

var startTime = time.Now()

type HealthController struct {
	db *pgxpool.Pool
}

func NewHealthController(db *pgxpool.Pool) *HealthController {
	return &HealthController{db: db}
}

// Check returns overall system health, uptime, and database status
func (c *HealthController) Check(w http.ResponseWriter, r *http.Request) {
	dbStatus := "connected"
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if c.db == nil {
		dbStatus = "disconnected (offline mode)"
	} else if err := c.db.Ping(ctx); err != nil {
		dbStatus = "unreachable"
	}

	reuse.Success(w, "Server is operational", map[string]interface{}{
		"status":    "healthy",
		"database":  dbStatus,
		"uptime":    time.Since(startTime).String(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// Live is the Kubernetes/Cloud Liveness Probe. Returns 200 OK if server process is running.
func (c *HealthController) Live(w http.ResponseWriter, r *http.Request) {
	reuse.Success(w, "Server process is alive", map[string]interface{}{
		"status": "alive",
		"uptime": time.Since(startTime).String(),
	})
}

// Ready is the Kubernetes/Cloud Readiness Probe. Returns 200 OK only if database is connected and ready to accept traffic.
func (c *HealthController) Ready(w http.ResponseWriter, r *http.Request) {
	if c.db == nil {
		reuse.Error(w, http.StatusServiceUnavailable, "Database pool is not initialized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := c.db.Ping(ctx); err != nil {
		reuse.Error(w, http.StatusServiceUnavailable, "Database is unreachable: "+err.Error())
		return
	}

	reuse.Success(w, "Server and database are ready", map[string]interface{}{
		"status":   "ready",
		"database": "connected",
	})
}
