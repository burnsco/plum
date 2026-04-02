package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"plum/internal/auth"
	"plum/internal/db"
)

type AuthHandler struct {
	DB      *sql.DB
	Limiter *AuthRateLimiter
}

type setupStatusResponse struct {
	HasAdmin        bool               `json:"hasAdmin"`
	LibraryDefaults setupLibraryConfig `json:"libraryDefaults"`
}

type setupLibraryConfig struct {
	TV    string `json:"tv"`
	Movie string `json:"movie"`
	Anime string `json:"anime"`
	Music string `json:"music"`
}

func setupLibraryDefaults() setupLibraryConfig {
	return setupLibraryConfig{
		TV:    envOrDefault("PLUM_MEDIA_TV_PATH", "/tv"),
		Movie: envOrDefault("PLUM_MEDIA_MOVIES_PATH", "/movies"),
		Anime: envOrDefault("PLUM_MEDIA_ANIME_PATH", "/anime"),
		Music: envOrDefault("PLUM_MEDIA_MUSIC_PATH", "/music"),
	}
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func (h *AuthHandler) SetupStatus(w http.ResponseWriter, r *http.Request) {
	var count int
	err := h.DB.QueryRow(`SELECT COUNT(1) FROM users WHERE is_admin = 1`).Scan(&count)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(setupStatusResponse{
		HasAdmin:        count > 0,
		LibraryDefaults: setupLibraryDefaults(),
	})
}

type adminSetupRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func normalizeEmail(e string) string {
	return strings.ToLower(strings.TrimSpace(e))
}

func validatePassword(pw string) error {
	if len(pw) < 10 {
		return errors.New("password too short")
	}
	return nil
}

func (h *AuthHandler) AdminSetup(w http.ResponseWriter, r *http.Request) {
	if !h.rateLimiter().Allow(clientIP(r), time.Now()) {
		http.Error(w, "too many attempts", http.StatusTooManyRequests)
		return
	}

	var existing int
	if err := h.DB.QueryRow(`SELECT COUNT(1) FROM users WHERE is_admin = 1`).Scan(&existing); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if existing > 0 {
		http.Error(w, "admin already exists", http.StatusConflict)
		return
	}

	var payload adminSetupRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	email := normalizeEmail(payload.Email)
	if email == "" {
		http.Error(w, "email required", http.StatusBadRequest)
		return
	}
	if err := validatePassword(payload.Password); err != nil {
		http.Error(w, "weak password", http.StatusBadRequest)
		return
	}

	pwHash, err := auth.HashPassword(payload.Password)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	var userID int
	err = tx.QueryRowContext(
		r.Context(),
		`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`,
		email, pwHash, now,
	).Scan(&userID)
	if err != nil {
		_ = tx.Rollback()
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	sessID, err := auth.NewSessionID()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	expires := now.Add(auth.SessionLifetime())
	if _, err := tx.ExecContext(
		r.Context(),
		`INSERT INTO sessions (id, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		sessID, userID, now, expires,
	); err != nil {
		_ = tx.Rollback()
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	setSessionCookie(w, r, sessID, expires)

	resp := db.User{
		ID:        userID,
		Email:     email,
		IsAdmin:   true,
		CreatedAt: now,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponse struct {
	ID      int    `json:"id"`
	Email   string `json:"email"`
	IsAdmin bool   `json:"is_admin"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if !h.rateLimiter().Allow(clientIP(r), time.Now()) {
		http.Error(w, "too many attempts", http.StatusTooManyRequests)
		return
	}

	var payload loginRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	email := normalizeEmail(payload.Email)
	if email == "" || payload.Password == "" {
		http.Error(w, "email and password required", http.StatusBadRequest)
		return
	}

	var u db.User
	err := h.DB.QueryRow(
		`SELECT id, email, password_hash, is_admin, created_at FROM users WHERE email = ?`,
		email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.IsAdmin, &u.CreatedAt)
	if err != nil {
		time.Sleep(500 * time.Millisecond)
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	if err := auth.CheckPasswordHash(payload.Password, u.PasswordHash); err != nil {
		time.Sleep(500 * time.Millisecond)
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	sessID, err := auth.NewSessionID()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	now := time.Now().UTC()
	expires := now.Add(auth.SessionLifetime())
	if _, err := h.DB.Exec(
		`INSERT INTO sessions (id, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		sessID, u.ID, now, expires,
	); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	setSessionCookie(w, r, sessID, expires)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(userResponse{
		ID:      u.ID,
		Email:   u.Email,
		IsAdmin: u.IsAdmin,
	})
}

type deviceLoginResponse struct {
	User         userResponse `json:"user"`
	SessionToken string       `json:"sessionToken"`
	ExpiresAt    time.Time    `json:"expiresAt"`
}

// DeviceLogin creates a session and returns the token in JSON for native clients (no Set-Cookie).
func (h *AuthHandler) DeviceLogin(w http.ResponseWriter, r *http.Request) {
	if !h.rateLimiter().Allow(clientIP(r), time.Now()) {
		http.Error(w, "too many attempts", http.StatusTooManyRequests)
		return
	}

	var payload loginRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	email := normalizeEmail(payload.Email)
	if email == "" || payload.Password == "" {
		http.Error(w, "email and password required", http.StatusBadRequest)
		return
	}

	var u db.User
	err := h.DB.QueryRow(
		`SELECT id, email, password_hash, is_admin, created_at FROM users WHERE email = ?`,
		email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.IsAdmin, &u.CreatedAt)
	if err != nil {
		time.Sleep(500 * time.Millisecond)
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	if err := auth.CheckPasswordHash(payload.Password, u.PasswordHash); err != nil {
		time.Sleep(500 * time.Millisecond)
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	sessID, err := auth.NewSessionID()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	now := time.Now().UTC()
	expires := now.Add(auth.SessionLifetime())
	if _, err := h.DB.Exec(
		`INSERT INTO sessions (id, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		sessID, u.ID, now, expires,
	); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(deviceLoginResponse{
		User: userResponse{
			ID:      u.ID,
			Email:   u.Email,
			IsAdmin: u.IsAdmin,
		},
		SessionToken: sessID,
		ExpiresAt:    expires,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	sessID := SessionIDFromContext(r.Context())
	if sessID == "" {
		sessID = effectiveSessionID(r)
	}
	if sessID != "" {
		_, _ = h.DB.Exec(`DELETE FROM sessions WHERE id = ?`, sessID)
	}
	clearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(userResponse{
		ID:      user.ID,
		Email:   user.Email,
		IsAdmin: user.IsAdmin,
	})
}

func (h *AuthHandler) rateLimiter() *AuthRateLimiter {
	if h != nil && h.Limiter != nil {
		return h.Limiter
	}
	return defaultAuthLimiter
}
