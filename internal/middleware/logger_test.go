package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoggerPreservesStreamingFlush(t *testing.T) {
	handler := Logger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("logging middleware removed http.Flusher")
		}
		flusher.Flush()
	}))

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/stream", nil))
	if !response.Flushed {
		t.Fatal("expected logging middleware to flush the underlying response")
	}
}
