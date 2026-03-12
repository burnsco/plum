package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSetSessionCookie_UsesSecureFlagFromEnv(t *testing.T) {
	t.Setenv("PLUM_SECURE_COOKIES", "true")
	req := httptest.NewRequest(http.MethodGet, "http://localhost/api/auth/login", nil)
	rec := httptest.NewRecorder()

	setSessionCookie(rec, req, "session-id", mustTime(t, "2026-03-12T15:04:05Z"))

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if !cookies[0].Secure {
		t.Fatal("expected session cookie to be secure when PLUM_SECURE_COOKIES=true")
	}
}

func TestSetSessionCookie_DefaultsToInsecureForLocalHTTP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://localhost/api/auth/login", nil)
	rec := httptest.NewRecorder()

	setSessionCookie(rec, req, "session-id", mustTime(t, "2026-03-12T15:04:05Z"))

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].Secure {
		t.Fatal("expected localhost cookie to default to insecure over plain HTTP")
	}
}

func TestSetSessionCookie_UsesSecureFlagBehindHTTPSProxy(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://plum.example/api/auth/login", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()

	setSessionCookie(rec, req, "session-id", mustTime(t, "2026-03-12T15:04:05Z"))

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if !cookies[0].Secure {
		t.Fatal("expected proxied HTTPS cookie to default to secure")
	}
}

func TestSetSessionCookie_AllowsExplicitInsecureOverride(t *testing.T) {
	t.Setenv("PLUM_INSECURE_COOKIES", "true")
	req := httptest.NewRequest(http.MethodGet, "https://plum.example/api/auth/login", nil)
	rec := httptest.NewRecorder()

	setSessionCookie(rec, req, "session-id", mustTime(t, "2026-03-12T15:04:05Z"))

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].Secure {
		t.Fatal("expected explicit insecure override to disable secure cookies")
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

func TestRequestBodyLimitMiddleware_RejectsOversizedRequests(t *testing.T) {
	middleware := RequestBodyLimitMiddleware(8)
	next := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"too-long"}`))
	rec := httptest.NewRecorder()
	next.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", rec.Code)
	}
}

func TestRequestBodyLimitMiddleware_SkipsReadOnlyRequests(t *testing.T) {
	middleware := RequestBodyLimitMiddleware(8)
	next := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/stream/1", strings.NewReader(strings.Repeat("x", 64)))
	rec := httptest.NewRecorder()
	next.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected GET request to pass through, got %d", rec.Code)
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
