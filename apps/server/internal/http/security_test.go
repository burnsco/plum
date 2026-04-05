package httpapi

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestOriginAllowed_LoopbackHTTPAnyPort(t *testing.T) {
	allowed := AllowedOriginsFromEnv("")
	cases := []struct {
		origin string
		want   bool
	}{
		{"http://localhost:5174", true},
		{"http://127.0.0.1:5174", true},
		{"http://[::1]:9999", true},
		{"https://localhost:5174", false},
		{"http://192.168.1.1:5174", false},
		{"http://evil.example", false},
	}
	for _, tc := range cases {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Origin", tc.origin)
		if got := OriginAllowed(req, allowed); got != tc.want {
			t.Fatalf("OriginAllowed(%q, default): got %v want %v", tc.origin, got, tc.want)
		}
	}
}

func TestOriginAllowed_ExplicitListStillWorks(t *testing.T) {
	allowed := map[string]struct{}{"http://app.example": {}}
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "http://app.example")
	if !OriginAllowed(req, allowed) {
		t.Fatal("expected explicit origin to match")
	}
}

func TestOriginAllowed_ExplicitListDoesNotAllowLoopbackWildcard(t *testing.T) {
	allowed := AllowedOriginsFromEnv("http://app.example")
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "http://localhost:5174")
	if OriginAllowed(req, allowed) {
		t.Fatal("expected explicit allowlist to reject loopback wildcard origin")
	}
}

func TestAuthRateLimiter_FullPruneRemovesExpiredKeys(t *testing.T) {
	l := NewAuthRateLimiter(10, time.Minute)
	t0 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	if !l.Allow("alpha", t0) {
		t.Fatal("alpha")
	}
	t1 := t0.Add(2 * time.Minute)
	if !l.Allow("beta", t1) {
		t.Fatal("beta")
	}
	l.mu.Lock()
	_, hasAlpha := l.attempts["alpha"]
	l.mu.Unlock()
	if hasAlpha {
		t.Fatal("expected full prune to drop expired key alpha")
	}
}
