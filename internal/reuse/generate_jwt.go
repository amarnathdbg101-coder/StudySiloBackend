package reuse

import (
	"errors"
	"fmt"
	"goProjectStructure/internal/utils"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// GenerateJwt creates a signed JWT token with standard claims valid for 30 days
func GenerateJwt(userID, email, role string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"role":    role,
		"exp":     time.Now().Add(30 * 24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(utils.MustLoad().JwtSecret))
}

// ParseTokenForRefresh validates the token signature and extracts user_id
func ParseTokenForRefresh(tokenStr string) (string, error) {
	secret := []byte(utils.MustLoad().JwtSecret)
	claims := jwt.MapClaims{}

	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	}, jwt.WithoutClaimsValidation())

	if err != nil || token == nil || !token.Valid {
		return "", errors.New("invalid token signature")
	}

	userID, ok := claims["user_id"].(string)
	if !ok || userID == "" {
		return "", errors.New("missing user_id in token")
	}

	return userID, nil
}

// RevokeToken marks a JWT token string as revoked in memory until TTL expires
func RevokeToken(tokenStr string, ttl time.Duration) {
	if tokenStr == "" {
		return
	}
	if ttl <= 0 {
		ttl = 30 * 24 * time.Hour
	}
	GetCache().Set("revoked_token:"+tokenStr, true, ttl)
}

// IsTokenRevoked returns true if the JWT token has been explicitly revoked via logout
func IsTokenRevoked(tokenStr string) bool {
	if tokenStr == "" {
		return false
	}
	_, found := GetCache().Get("revoked_token:" + tokenStr)
	return found
}