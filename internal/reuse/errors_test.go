package reuse

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestErrorSanitizesServerErrors(t *testing.T) {
	recorder := httptest.NewRecorder()
	Error(recorder, http.StatusInternalServerError, "database password or SQL details")

	var response Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if recorder.Code != http.StatusInternalServerError || response.Error != "Internal Server Error" {
		t.Fatalf("unexpected sanitized response: status=%d error=%q", recorder.Code, response.Error)
	}
}

func TestErrorPreservesClientErrors(t *testing.T) {
	recorder := httptest.NewRecorder()
	Error(recorder, http.StatusUnprocessableEntity, "question is required")

	var response Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if recorder.Code != http.StatusUnprocessableEntity || response.Error != "question is required" {
		t.Fatalf("unexpected client error response: status=%d error=%q", recorder.Code, response.Error)
	}
}
