package controller_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"goProjectStructure/internal/handler/controller"
)

func TestHealthCheck_DisconnectedMode(t *testing.T) {
	ctrl := controller.NewHealthController(nil)

	req, err := http.NewRequest(http.MethodGet, "/health", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rec := httptest.NewRecorder()
	ctrl.Check(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if success, ok := response["success"].(bool); !ok || !success {
		t.Errorf("Expected success=true, got %v", response["success"])
	}
}

func TestHealthLive(t *testing.T) {
	ctrl := controller.NewHealthController(nil)

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rec := httptest.NewRecorder()
	ctrl.Live(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestHealthReady_WithoutDB(t *testing.T) {
	ctrl := controller.NewHealthController(nil)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	ctrl.Ready(rec, req)

	// Since DB is nil, readiness probe must fail with 503 Service Unavailable
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503 when DB is nil, got %d", rec.Code)
	}
}
