package httpapi

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultRequestBodyLimitBytes = 1 << 20
	defaultAuthRateLimitMax      = 5
	defaultAuthRateLimitWindow   = time.Minute
)

type AuthRateLimiter struct {
	mu          sync.Mutex
	attempts    map[string][]time.Time
	maxAttempts int
	window      time.Duration
}

var defaultAuthLimiter = NewAuthRateLimiter(defaultAuthRateLimitMax, defaultAuthRateLimitWindow)

func NewAuthRateLimiter(maxAttempts int, window time.Duration) *AuthRateLimiter {
	if maxAttempts <= 0 {
		maxAttempts = defaultAuthRateLimitMax
	}
	if window <= 0 {
		window = defaultAuthRateLimitWindow
	}
	return &AuthRateLimiter{
		attempts:    make(map[string][]time.Time),
		maxAttempts: maxAttempts,
		window:      window,
	}
}

func (l *AuthRateLimiter) Allow(key string, now time.Time) bool {
	if l == nil {
		return true
	}

	if key == "" {
		key = "unknown"
	}

	cutoff := now.Add(-l.window)

	l.mu.Lock()
	defer l.mu.Unlock()

	existing := l.attempts[key][:0]
	for _, attempt := range l.attempts[key] {
		if attempt.After(cutoff) {
			existing = append(existing, attempt)
		}
	}
	if len(existing) >= l.maxAttempts {
		l.attempts[key] = existing
		return false
	}

	l.attempts[key] = append(existing, now)
	return true
}

func RequestBodyLimitBytes() int64 {
	raw := strings.TrimSpace(os.Getenv("PLUM_MAX_REQUEST_BODY_BYTES"))
	if raw == "" {
		return defaultRequestBodyLimitBytes
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return defaultRequestBodyLimitBytes
	}
	return value
}

func RequestBodyLimitMiddleware(limit int64) func(http.Handler) http.Handler {
	if limit <= 0 {
		limit = defaultRequestBodyLimitBytes
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}

			if r.ContentLength > limit {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}

			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}

func OriginAllowed(r *http.Request, allowedOrigins map[string]struct{}) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return false
	}
	if _, ok := allowedOrigins[origin]; ok {
		return true
	}
	if _, ok := allowedOrigins[loopbackHTTPAnyPortOrigin]; !ok {
		return false
	}
	// Vite (and similar) may use any free port (e.g. 5174 when 5173 is taken).
	return isLoopbackHTTPOrigin(origin)
}

func isLoopbackHTTPOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "http" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(u.Hostname())) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func clientIP(r *http.Request) string {
	forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
