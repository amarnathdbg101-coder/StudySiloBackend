package middleware

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestExtractIPIgnoresUntrustedForwardingHeaders(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "198.51.100.10:4321"
	req.Header.Set("CF-Connecting-IP", "203.0.113.1")
	req.Header.Set("X-Forwarded-For", "203.0.113.2")
	req.Header.Set("X-Real-IP", "203.0.113.3")

	if got := ExtractIP(req); got != "198.51.100.10" {
		t.Fatalf("ExtractIP() = %q, want direct peer IP", got)
	}
}

func TestRateLimiterUsesForwardedIPOnlyThroughTrustedProxy(t *testing.T) {
	limiter := NewIPRateLimiter(10, time.Minute, "10.0.0.0/8")
	defer limiter.Stop()

	trustedRequest := httptest.NewRequest("GET", "/", nil)
	trustedRequest.RemoteAddr = "10.0.0.8:4321"
	trustedRequest.Header.Set("X-Forwarded-For", "203.0.113.4, 10.0.0.7")
	if got := limiter.clientIP(trustedRequest); got != "203.0.113.4" {
		t.Fatalf("trusted proxy client IP = %q, want 203.0.113.4", got)
	}

	untrustedRequest := httptest.NewRequest("GET", "/", nil)
	untrustedRequest.RemoteAddr = "198.51.100.10:4321"
	untrustedRequest.Header.Set("X-Forwarded-For", "203.0.113.5")
	if got := limiter.clientIP(untrustedRequest); got != "198.51.100.10" {
		t.Fatalf("untrusted peer client IP = %q, want direct peer IP", got)
	}
}
