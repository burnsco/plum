package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"plum/internal/db"
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

func envBoolEnabled(key string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	default:
		return false, false
	}
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
	if secure, ok := envBoolEnabled("PLUM_SECURE_COOKIES"); ok {
		return secure
	}
	if insecure, ok := envBoolEnabled("PLUM_INSECURE_COOKIES"); ok && insecure {
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
			type sessionCandidate struct {
				id          string
				viaBearer   bool
				clearCookie bool
			}
			candidates := make([]sessionCandidate, 0, 2)
			if cookieID != "" {
				candidates = append(candidates, sessionCandidate{id: cookieID, clearCookie: true})
			}
			if bearerID != "" && bearerID != cookieID {
				candidates = append(candidates, sessionCandidate{id: bearerID, viaBearer: true})
			}
			if len(candidates) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			for _, candidate := range candidates {
				var (
					userID    int
					expiresAt time.Time
				)
				err := dbConn.QueryRow(
					`SELECT user_id, expires_at FROM sessions WHERE id = ?`,
					candidate.id,
				).Scan(&userID, &expiresAt)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) && candidate.clearCookie {
						clearSessionCookie(w, r)
					}
					continue
				}
				if time.Now().After(expiresAt) {
					_, _ = dbConn.Exec(`DELETE FROM sessions WHERE id = ?`, candidate.id)
					if candidate.clearCookie {
						clearSessionCookie(w, r)
					}
					continue
				}

				var u db.User
				err = dbConn.QueryRow(
					`SELECT id, email, is_admin, created_at FROM users WHERE id = ?`,
					userID,
				).Scan(&u.ID, &u.Email, &u.IsAdmin, &u.CreatedAt)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						_, _ = dbConn.Exec(`DELETE FROM sessions WHERE id = ?`, candidate.id)
						if candidate.clearCookie {
							clearSessionCookie(w, r)
						}
					}
					continue
				}

				ctx := withSessionAuth(withUser(r.Context(), &u), candidate.id, candidate.viaBearer)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
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
