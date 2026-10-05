package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWTAuthRejectsQueryStringTokens(t *testing.T) {
	for _, queryKey := range []string{"token", "access_token"} {
		t.Run(queryKey, func(t *testing.T) {
			nextCalled := false
			handler := JWTAuth("test-secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
			}))
			req := httptest.NewRequest(http.MethodGet, "/protected?"+queryKey+"=some-token", nil)
			res := httptest.NewRecorder()

			handler.ServeHTTP(res, req)

			if res.Code != http.StatusUnauthorized || nextCalled {
				t.Fatalf("query token must not authenticate: status=%d nextCalled=%t", res.Code, nextCalled)
			}
			if !strings.Contains(res.Body.String(), "missing authorization header") {
				t.Fatalf("expected missing-header response, got %q", res.Body.String())
			}
		})
	}
}

func TestOptionalJWTAuthIgnoresQueryStringToken(t *testing.T) {
	const secret = "test-secret"
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{UserID: "user-1"}).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign test token: %v", err)
	}

	handler := OptionalJWTAuth(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if claims := GetUserFromContext(r.Context()); claims != nil {
			t.Errorf("query token unexpectedly authenticated user %q", claims.UserID)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/optional?access_token="+token, nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected anonymous optional-auth request to continue, got %d", res.Code)
	}
}

func TestJWTAuthAcceptsAuthorizationBearerToken(t *testing.T) {
	const secret = "test-secret"
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{UserID: "user-1"}).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign test token: %v", err)
	}

	handler := JWTAuth(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := GetUserFromContext(r.Context())
		if claims == nil || claims.UserID != "user-1" {
			t.Errorf("expected authenticated user context, got %#v", claims)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected bearer token to authenticate, got %d", res.Code)
	}
}
