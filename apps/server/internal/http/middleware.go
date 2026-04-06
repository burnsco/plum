package httpapi

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"plum/internal/db"
	"plum/internal/env"
)

type contextKey string

const (
	userContextKey          contextKey = "plum.user"
	sessionIDContextKey     contextKey = "plum.sessionID"
	authViaBearerContextKey contextKey = "plum.authViaBearer"
)

func UserFromContext(ctx context.Context) *db.User {
	val := ctx.Value(userContextKey)
	if val == nil {
		return nil
	}
	if u, ok := val.(*db.User); ok {
		return u
	}
	return nil
}

func withUser(ctx context.Context, u *db.User) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}

// SessionIDFromContext returns the authenticated session id when AuthMiddleware ran successfully.
func SessionIDFromContext(ctx context.Context) string {
	if s, ok := ctx.Value(sessionIDContextKey).(string); ok {
		return s
	}
	return ""
}

// AuthViaBearerFromContext is true when the session was established via Authorization: Bearer.
func AuthViaBearerFromContext(ctx context.Context) bool {
	if b, ok := ctx.Value(authViaBearerContextKey).(bool); ok {
		return b
	}
	return false
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status       int
	bytesWritten int
}

func (w *loggingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hj.Hijack()
}

func (w *loggingResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *loggingResponseWriter) Push(target string, opts *http.PushOptions) error {
	if pusher, ok := w.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, opts)
	}
	return http.ErrNotSupported
}

func (w *loggingResponseWriter) ReadFrom(r io.Reader) (int64, error) {
	if readerFrom, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		n, err := readerFrom.ReadFrom(r)
		w.bytesWritten += int(n)
		return n, err
	}
	n, err := io.Copy(w.ResponseWriter, r)
	w.bytesWritten += int(n)
	return n, err
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	if w.status != 0 {
		// Headers already sent (e.g. streaming handler wrote body then failed).
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *loggingResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytesWritten += n
	return n, err
}

func RequestLoggingMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &loggingResponseWriter{ResponseWriter: w}
			next.ServeHTTP(rw, r)
			status := rw.status
			if status == 0 {
				status = http.StatusOK
			}
			route := r.URL.Path
			if rc := chi.RouteContext(r.Context()); rc != nil {
				if pattern := strings.TrimSpace(rc.RoutePattern()); pattern != "" {
					route = pattern
				}
			}
			attrs := []any{
				"component", "server",
				"event", "request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", status,
				"duration_ms", time.Since(start).Milliseconds(),
				"bytes_written", rw.bytesWritten,
			}
			if route != r.URL.Path {
				attrs = append(attrs, "route", route)
			}
			if rip := clientIP(r); rip != "" {
				attrs = append(attrs, "remote_ip", rip)
			}
			if user := UserFromContext(r.Context()); user != nil {
				attrs = append(attrs, "user_id", user.ID)
			}
			if AuthViaBearerFromContext(r.Context()) {
				attrs = append(attrs, "auth_via_bearer", true)
			}
			slog.Info("request", attrs...)
		})
	}
}

func withSessionAuth(ctx context.Context, sessionID string, viaBearer bool) context.Context {
	ctx = context.WithValue(ctx, sessionIDContextKey, sessionID)
	ctx = context.WithValue(ctx, authViaBearerContextKey, viaBearer)
	return ctx
}

func sessionCookieName() string {
	if v := os.Getenv("PLUM_SESSION_COOKIE"); v != "" {
		return v
	}
	return "plum_session"
}

func sessionIDFromCookie(r *http.Request) string {
	c, err := r.Cookie(sessionCookieName())
	if err != nil {
		return ""
	}
	return c.Value
}

func bearerSessionToken(r *http.Request) string {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if h == "" {
		return ""
	}
	const prefix = "Bearer "
	if len(h) < len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}

// effectiveSessionID returns cookie session id if present, otherwise Bearer token session id.
func effectiveSessionID(r *http.Request) string {
	if id := sessionIDFromCookie(r); id != "" {
		return id
	}
	return bearerSessionToken(r)
}

func requestHost(r *http.Request) string {
	host := strings.TrimSpace(r.Host)
	if host == "" {
		return ""
	}
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		return parsedHost
	}
	return host
}

func isLocalhostHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func secureCookiesEnabled(r *http.Request) bool {
	if secure, ok := env.Bool("PLUM_SECURE_COOKIES"); ok {
		return secure
	}
	if insecure, ok := env.Bool("PLUM_INSECURE_COOKIES"); ok && insecure {
		return false
	}
	if r != nil {
		if r.TLS != nil {
			return true
		}
		if proto := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))); proto == "https" {
			return true
		}
		if isLocalhostHost(requestHost(r)) {
			return false
		}
	}
	return true
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, sessionID string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName(),
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secureCookiesEnabled(r),
		Expires:  expires,
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName(),
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secureCookiesEnabled(r),
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}

func AuthMiddleware(dbConn *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookieID := sessionIDFromCookie(r)
			bearerID := bearerSessionToken(r)

			trySession := func(sessionID string, viaBearer, clearCookie bool) bool {
				var u db.User
				var expiresAt time.Time
				err := dbConn.QueryRow(
					`SELECT u.id, u.email, u.is_admin, u.created_at, s.expires_at
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.id = ?`,
					sessionID,
				).Scan(&u.ID, &u.Email, &u.IsAdmin, &u.CreatedAt, &expiresAt)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) && clearCookie {
						clearSessionCookie(w, r)
					}
					return false
				}
				if time.Now().After(expiresAt) {
					_, _ = dbConn.Exec(`DELETE FROM sessions WHERE id = ?`, sessionID)
					if clearCookie {
						clearSessionCookie(w, r)
					}
					return false
				}

				ctx := withSessionAuth(withUser(r.Context(), &u), sessionID, viaBearer)
				next.ServeHTTP(w, r.WithContext(ctx))
				return true
			}

			// If a session cookie was sent but is invalid or expired, do not fall back to Bearer:
			// avoids authenticating as a different principal when the browser still has a stale cookie.
			if cookieID != "" {
				if trySession(cookieID, false, true) {
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			if bearerID != "" {
				if trySession(bearerID, true, false) {
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if UserFromContext(r.Context()) == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := UserFromContext(r.Context())
		if u == nil || !u.IsAdmin {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
