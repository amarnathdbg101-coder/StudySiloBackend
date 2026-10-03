package middleware

import "net/http"

// SecurityHeaders applies OWASP recommended security headers to all HTTP responses.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent browsers from MIME-sniffing the response
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking by forbidding embedding in iframes
		w.Header().Set("X-Frame-Options", "DENY")

		// Enable XSS filtering built into older browsers
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Control how much referrer information is sent with requests
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Disable unused browser features
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

		next.ServeHTTP(w, r)
	})
}
