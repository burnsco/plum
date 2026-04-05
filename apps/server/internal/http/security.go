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
	mu             sync.Mutex
	attempts       map[string][]time.Time
	maxAttempts    int
	window         time.Duration
	lastFullPrune  time.Time
	pruneInterval  time.Duration
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
		attempts:      make(map[string][]time.Time),
		maxAttempts:   maxAttempts,
		window:        window,
		pruneInterval: time.Minute,
	}
}

func (l *AuthRateLimiter) pruneAllExpiredLocked(now time.Time) {
	cutoff := now.Add(-l.window)
	for key, stamps := range l.attempts {
		kept := stamps[:0]
		for _, t := range stamps {
			if t.After(cutoff) {
				kept = append(kept, t)
			}
		}
		if len(kept) == 0 {
			delete(l.attempts, key)
		} else {
			l.attempts[key] = kept
		}
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

	interval := l.pruneInterval
	if interval <= 0 {
		interval = time.Minute
	}
	if l.lastFullPrune.IsZero() || now.Sub(l.lastFullPrune) >= interval {
		l.pruneAllExpiredLocked(now)
		l.lastFullPrune = now
	}

	existing := l.attempts[key][:0]
	for _, attempt := range l.attempts[key] {
		if attempt.After(cutoff) {
			existing = append(existing, attempt)
		}
	}
	// Prune the map key when all prior attempts have expired.
	if len(existing) == 0 {
		delete(l.attempts, key)
		existing = nil
	}
	if len(existing) >= l.maxAttempts {
		l.attempts[key] = existing
		return false
	}

	existing = append(existing, now)
	l.attempts[key] = existing
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

var (
	trustProxyOnce      sync.Once
	trustProxyEnabled  bool
	trustProxyNetworks []*net.IPNet
)

func loadTrustProxySettings() {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("PLUM_TRUST_PROXY")))
	switch raw {
	case "1", "true", "yes", "on":
		trustProxyEnabled = true
	default:
		return
	}
	cidrList := strings.TrimSpace(os.Getenv("PLUM_TRUSTED_PROXY_CIDRS"))
	if cidrList == "" {
		cidrList = "127.0.0.1/32,::1/128"
	}
	for _, part := range strings.Split(cidrList, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		_, network, err := net.ParseCIDR(part)
		if err != nil {
			continue
		}
		trustProxyNetworks = append(trustProxyNetworks, network)
	}
	if len(trustProxyNetworks) == 0 {
		trustProxyEnabled = false
	}
}

func remoteAddrIP(r *http.Request) net.IP {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return net.ParseIP(strings.TrimSpace(r.RemoteAddr))
	}
	return net.ParseIP(host)
}

func ipInTrustedNets(ip net.IP) bool {
	for _, n := range trustProxyNetworks {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func forwardedClientIP(r *http.Request) string {
	xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if xff == "" {
		return ""
	}
	parts := strings.Split(xff, ",")
	return strings.TrimSpace(parts[0])
}

// clientIP returns the client IP for rate limiting and logging.
// By default only RemoteAddr is used so clients cannot spoof X-Forwarded-For.
// Set PLUM_TRUST_PROXY=1 (and optionally PLUM_TRUSTED_PROXY_CIDRS, default 127.0.0.1/32,::1/128)
// when the server is behind reverse proxies that terminate TLS and append X-Forwarded-For.
func clientIP(r *http.Request) string {
	trustProxyOnce.Do(loadTrustProxySettings)

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	remoteHost := host
	if err != nil || remoteHost == "" {
		remoteHost = strings.TrimSpace(r.RemoteAddr)
	}

	if trustProxyEnabled {
		if ip := remoteAddrIP(r); ip != nil && ipInTrustedNets(ip) {
			if client := forwardedClientIP(r); client != "" {
				return client
			}
		}
	}

	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
