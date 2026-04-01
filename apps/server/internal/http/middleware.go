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

const userContextKey contextKey = "plum.user"

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

func sessionCookieName() string {
	if v := os.Getenv("PLUM_SESSION_COOKIE"); v != "" {
		return v
	}
	return "plum_session"
}

func sessionIDFromRequest(r *http.Request) string {
	c, err := r.Cookie(sessionCookieName())
	if err != nil {
		return ""
	}
	return c.Value
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
			sessID := sessionIDFromRequest(r)
			if sessID == "" {
				next.ServeHTTP(w, r)
				return
			}

			var (
				userID    int
				expiresAt time.Time
			)
			err := dbConn.QueryRow(
				`SELECT user_id, expires_at FROM sessions WHERE id = ?`,
				sessID,
			).Scan(&userID, &expiresAt)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					clearSessionCookie(w, r)
				}
				next.ServeHTTP(w, r)
				return
			}
			if time.Now().After(expiresAt) {
				_, _ = dbConn.Exec(`DELETE FROM sessions WHERE id = ?`, sessID)
				clearSessionCookie(w, r)
				next.ServeHTTP(w, r)
				return
			}

			var u db.User
			err = dbConn.QueryRow(
				`SELECT id, email, is_admin, created_at FROM users WHERE id = ?`,
				userID,
			).Scan(&u.ID, &u.Email, &u.IsAdmin, &u.CreatedAt)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					_, _ = dbConn.Exec(`DELETE FROM sessions WHERE id = ?`, sessID)
					clearSessionCookie(w, r)
				}
				next.ServeHTTP(w, r)
				return
			}

			ctx := withUser(r.Context(), &u)
			next.ServeHTTP(w, r.WithContext(ctx))
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
