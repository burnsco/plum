package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSetSessionCookie_UsesSecureFlagFromEnv(t *testing.T) {
	t.Setenv("PLUM_SECURE_COOKIES", "true")
	rec := httptest.NewRecorder()

	setSessionCookie(rec, "session-id", mustTime(t, "2026-03-12T15:04:05Z"))

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if !cookies[0].Secure {
		t.Fatal("expected session cookie to be secure when PLUM_SECURE_COOKIES=true")
	}
}

func TestCORSMiddleware_ReflectsOnlyAllowedOrigins(t *testing.T) {
	middleware := CORSMiddleware(AllowedOriginsFromEnv("http://allowed.example"))
	next := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	allowedReq := httptest.NewRequest(http.MethodGet, "/health", nil)
	allowedReq.Header.Set("Origin", "http://allowed.example")
	allowedRec := httptest.NewRecorder()
	next.ServeHTTP(allowedRec, allowedReq)

	if got := allowedRec.Header().Get("Access-Control-Allow-Origin"); got != "http://allowed.example" {
		t.Fatalf("expected allowed origin to be reflected, got %q", got)
	}
	if got := allowedRec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("expected credentials header for allowed origin, got %q", got)
	}

	disallowedReq := httptest.NewRequest(http.MethodGet, "/health", nil)
	disallowedReq.Header.Set("Origin", "http://blocked.example")
	disallowedRec := httptest.NewRecorder()
	next.ServeHTTP(disallowedRec, disallowedReq)

	if got := disallowedRec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no allow-origin header for blocked origin, got %q", got)
	}
	if got := disallowedRec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("expected no credentials header for blocked origin, got %q", got)
	}
}

func mustTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}
	return parsed
}
