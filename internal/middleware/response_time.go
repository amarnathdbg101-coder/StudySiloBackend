package middleware

import (
	"fmt"
	"net/http"
	"time"
)

// ResponseTimeHeader injects X-Response-Time header into every HTTP response for client latency telemetry
func ResponseTimeHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		duration := time.Since(start)
		w.Header().Set("X-Response-Time", fmt.Sprintf("%.2fms", float64(duration.Microseconds())/1000.0))
	})
}
