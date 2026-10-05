package middleware

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type clientRecord struct {
	count     int
	windowEnd time.Time
}

// IPRateLimiter tracks request counts per client IP in memory
type IPRateLimiter struct {
	mu             sync.RWMutex
	records        map[string]*clientRecord
	limit          int
	window         time.Duration
	stopCleanup    chan struct{}
	trustedProxies []*net.IPNet
}

// NewIPRateLimiter creates a new rate limiter with background garbage collection
func NewIPRateLimiter(limit int, window time.Duration, trustedProxyCIDRs ...string) *IPRateLimiter {
	limiter := &IPRateLimiter{
		records:        make(map[string]*clientRecord),
		limit:          limit,
		window:         window,
		stopCleanup:    make(chan struct{}),
		trustedProxies: parseTrustedProxyCIDRs(trustedProxyCIDRs),
	}

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				limiter.cleanup()
			case <-limiter.stopCleanup:
				return
			}
		}
	}()

	return limiter
}

func parseTrustedProxyCIDRs(values []string) []*net.IPNet {
	networks := make([]*net.IPNet, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}

		if ip := net.ParseIP(value); ip != nil {
			bits := 128
			if ip4 := ip.To4(); ip4 != nil {
				ip = ip4
				bits = 32
			}
			networks = append(networks, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}

		_, network, err := net.ParseCIDR(value)
		if err == nil {
			networks = append(networks, network)
		}
	}
	return networks
}

func (l *IPRateLimiter) isTrustedProxy(ip net.IP) bool {
	for _, network := range l.trustedProxies {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func (l *IPRateLimiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	for ip, rec := range l.records {
		if now.After(rec.windowEnd) {
			delete(l.records, ip)
		}
	}
}

// Stop terminates the background cleanup routine
func (l *IPRateLimiter) Stop() {
	close(l.stopCleanup)
}

// CheckAndAllow checks if the given IP is within the rate limit and returns headers metadata
func (l *IPRateLimiter) CheckAndAllow(ip string) (allowed bool, remaining int, resetInSec int64) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	rec, exists := l.records[ip]

	if !exists || now.After(rec.windowEnd) {
		rec = &clientRecord{
			count:     1,
			windowEnd: now.Add(l.window),
		}
		l.records[ip] = rec
		remaining = l.limit - 1
		resetInSec = int64(time.Until(rec.windowEnd).Seconds())
		if resetInSec < 0 {
			resetInSec = 0
		}
		return true, remaining, resetInSec
	}

	resetInSec = int64(time.Until(rec.windowEnd).Seconds())
	if resetInSec < 0 {
		resetInSec = 0
	}

	if rec.count < l.limit {
		rec.count++
		remaining = l.limit - rec.count
		return true, remaining, resetInSec
	}

	remaining = 0
	return false, remaining, resetInSec
}

// ExtractIP extracts the client IP address from request headers or remote addr
func ExtractIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		if ip := net.ParseIP(host); ip != nil {
			return ip.String()
		}
		return host
	}
	if ip := net.ParseIP(r.RemoteAddr); ip != nil {
		return ip.String()
	}
	return r.RemoteAddr
}

func (l *IPRateLimiter) clientIP(r *http.Request) string {
	remoteIP := ExtractIP(r)
	parsedRemoteIP := net.ParseIP(remoteIP)
	if parsedRemoteIP == nil || !l.isTrustedProxy(parsedRemoteIP) {
		return remoteIP
	}

	if forwardedIP := net.ParseIP(strings.TrimSpace(r.Header.Get("CF-Connecting-IP"))); forwardedIP != nil {
		return forwardedIP.String()
	}

	forwarded := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for index := len(forwarded) - 1; index >= 0; index-- {
		ip := net.ParseIP(strings.TrimSpace(forwarded[index]))
		if ip == nil {
			continue
		}
		if !l.isTrustedProxy(ip) {
			return ip.String()
		}
	}

	if forwardedIP := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); forwardedIP != nil {
		return forwardedIP.String()
	}
	return remoteIP
}

// Middleware returns a Chi middleware handler function
func (l *IPRateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := l.clientIP(r)
			allowed, remaining, resetInSec := l.CheckAndAllow(ip)

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(l.limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetInSec, 10))

			if !allowed {
				w.Header().Set("Retry-After", strconv.FormatInt(resetInSec, 10))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"success":false,"error":"Too many requests. Please slow down and try again in a moment."}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Pre-configured standard limiters
var (
	// AuthRateLimiter limits sensitive login/register requests (10 requests per minute)
	AuthRateLimiter = NewIPRateLimiter(10, 1*time.Minute)

	// GlobalRateLimiter protects the API from aggressive scraping or DoS (120 requests per minute)
	GlobalRateLimiter = NewIPRateLimiter(120, 1*time.Minute)
)
