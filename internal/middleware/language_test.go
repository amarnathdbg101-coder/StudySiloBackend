package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAILanguageMiddleware(t *testing.T) {
	tests := []struct {
		name         string
		headerValue  string
		queryValue   string
		expectedLang string
	}{
		{
			name:         "Header provided with Hinglish",
			headerValue:  "hinglish",
			queryValue:   "",
			expectedLang: "hinglish",
		},
		{
			name:         "Header provided with uppercase English",
			headerValue:  "ENGLISH",
			queryValue:   "",
			expectedLang: "english",
		},
		{
			name:         "Query parameter provided with Hindi",
			headerValue:  "",
			queryValue:   "hindi",
			expectedLang: "hindi",
		},
		{
			name:         "Neither provided defaults to auto",
			headerValue:  "",
			queryValue:   "",
			expectedLang: "auto",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			url := "/api/v1/ai/tutor"
			if tc.queryValue != "" {
				url += "?lang=" + tc.queryValue
			}
			req := httptest.NewRequest(http.MethodPost, url, nil)
			if tc.headerValue != "" {
				req.Header.Set("X-AI-Language", tc.headerValue)
			}
			rec := httptest.NewRecorder()

			var capturedLang string
			handler := AILanguage(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedLang = GetAILanguage(r.Context())
				w.WriteHeader(http.StatusOK)
			}))

			handler.ServeHTTP(rec, req)

			if capturedLang != tc.expectedLang {
				t.Errorf("expected language '%s', got '%s'", tc.expectedLang, capturedLang)
			}
		})
	}
}
