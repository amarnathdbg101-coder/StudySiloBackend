// Package middleware provides JWT authentication middleware for HTTP handlers.
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"goProjectStructure/internal/reuse"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const ContextKeyUser = contextKey("user")

type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// JWTAuth verifies Bearer tokens from the Authorization header and attaches Claims to request context.
func JWTAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")

			if authHeader == "" {
				http.Error(w, `{"success":false,"error":"missing authorization header"}`, http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				http.Error(w, `{"success":false,"error":"invalid authorization header format"}`, http.StatusUnauthorized)
				return
			}

			tokenStr := parts[1]
			if reuse.IsTokenRevoked(tokenStr) {
				http.Error(w, `{"success":false,"error":"token has been revoked"}`, http.StatusUnauthorized)
				return
			}

			claims := &Claims{}

			token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return []byte(secret), nil
			})

			if err != nil || !token.Valid {
				http.Error(w, `{"success":false,"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), ContextKeyUser, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserFromContext retrieves authenticated Claims from request context.
func GetUserFromContext(ctx context.Context) *Claims {
	if claims, ok := ctx.Value(ContextKeyUser).(*Claims); ok {
		return claims
	}
	return nil
}

// OptionalJWTAuth validates tokens when present, attaching Claims to context.
// If no token is provided, it allows the request to proceed with claims = nil.
// If a token is provided but invalid/expired, it returns 401 Unauthorized.
func OptionalJWTAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")

			if authHeader == "" {
				next.ServeHTTP(w, r)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				http.Error(w, `{"success":false,"error":"invalid authorization header format"}`, http.StatusUnauthorized)
				return
			}

			tokenStr := parts[1]
			if reuse.IsTokenRevoked(tokenStr) {
				http.Error(w, `{"success":false,"error":"token has been revoked"}`, http.StatusUnauthorized)
				return
			}

			claims := &Claims{}

			token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return []byte(secret), nil
			})

			if err != nil || !token.Valid {
				http.Error(w, `{"success":false,"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), ContextKeyUser, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
