package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"goProjectStructure/internal/reuse"
	"go.uber.org/zap"
)

// Recoverer catches runtime panics, logs stack traces, and returns a 500 JSON response instead of crashing the server.
func Recoverer(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rvr := recover(); rvr != nil {
					stack := string(debug.Stack())
					reqID := GetRequestID(r.Context())

					if logger != nil {
						logger.Error("PANIC RECOVERED",
							zap.Any("error", rvr),
							zap.String("request_id", reqID),
							zap.String("path", r.URL.Path),
							zap.String("method", r.Method),
							zap.String("stack", stack),
						)
					} else {
						fmt.Printf("[PANIC] %v (RequestID: %s)\n%s\n", rvr, reqID, stack)
					}

					reuse.Error(w, http.StatusInternalServerError, "Internal Server Error. Please contact support.")
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
