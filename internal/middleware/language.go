package middleware

import (
	"context"
	"net/http"
	"strings"
)

type aiLangContextKey string

const AILanguageContextKey aiLangContextKey = "ai_language"

// AILanguage extracts the student's selected AI response language from the incoming request.
// It prioritizes the "X-AI-Language" HTTP header, then "?lang=" query parameter, defaulting to "auto".
func AILanguage(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := strings.TrimSpace(r.Header.Get("X-AI-Language"))
		if lang == "" {
			lang = strings.TrimSpace(r.URL.Query().Get("lang"))
		}
		if lang == "" {
			lang = "auto"
		}
		ctx := context.WithValue(r.Context(), AILanguageContextKey, strings.ToLower(lang))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetAILanguage retrieves the target AI response language from request context.
// Returns "auto" if not set or context is nil.
func GetAILanguage(ctx context.Context) string {
	if ctx == nil {
		return "auto"
	}
	if val, ok := ctx.Value(AILanguageContextKey).(string); ok && val != "" {
		return strings.ToLower(strings.TrimSpace(val))
	}
	return "auto"
}
