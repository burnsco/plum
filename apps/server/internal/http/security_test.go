package httpapi

import (
	"net/http/httptest"
	"testing"
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
