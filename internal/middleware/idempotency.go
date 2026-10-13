package middleware

import (
	"bytes"
	"net/http"
	"time"

	"goProjectStructure/internal/reuse"
)

type cachedResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

type bodyRecordingWriter struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
}

func (w *bodyRecordingWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *bodyRecordingWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *bodyRecordingWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// IdempotencyMiddleware prevents duplicate processing of POST/PUT/PATCH requests when X-Idempotency-Key is provided.
func IdempotencyMiddleware(ttl time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodPatch {
				next.ServeHTTP(w, r)
				return
			}

			idempotencyKey := r.Header.Get("X-Idempotency-Key")
			if idempotencyKey == "" {
				next.ServeHTTP(w, r)
				return
			}

			userID := "anonymous"
			if claims := GetUserFromContext(r.Context()); claims != nil && claims.UserID != "" {
				userID = claims.UserID
			}

			cacheKey := "idempotency:" + userID + ":" + idempotencyKey
			cache := reuse.GetCache()

			if val, found := cache.Get(cacheKey); found {
				if resp, ok := val.(*cachedResponse); ok {
					for k, v := range resp.Header {
						w.Header()[k] = v
					}
					w.Header().Set("X-Cache", "HIT")
					w.WriteHeader(resp.StatusCode)
					_, _ = w.Write(resp.Body)
					return
				}
			}

			recordingWriter := &bodyRecordingWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(recordingWriter, r)

			if recordingWriter.statusCode < 500 {
				savedHeader := make(http.Header)
				for k, v := range recordingWriter.Header() {
					savedHeader[k] = v
				}
				respToCache := &cachedResponse{
					StatusCode: recordingWriter.statusCode,
					Header:     savedHeader,
					Body:       recordingWriter.body.Bytes(),
				}
				cache.Set(cacheKey, respToCache, ttl)
			}
		})
	}
}
