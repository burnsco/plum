package httpapi

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"plum/internal/auth"
	"plum/internal/db"
	"plum/internal/env"
)

const (
	quickConnectCodeLength       = 6
	quickConnectMaxFailedGuesses = 5
	// Alphabet avoids ambiguous 0/O and 1/I (similar to many TV pairing UIs).
	quickConnectAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
)

var quickConnectFailureCounts sync.Map // normalized code -> *quickConnectFailSlot

type quickConnectFailSlot struct {
	n atomic.Int32
}

func quickConnectRegisterFailedGuess(code string) bool {
	v, _ := quickConnectFailureCounts.LoadOrStore(code, &quickConnectFailSlot{})
	return v.(*quickConnectFailSlot).n.Add(1) > int32(quickConnectMaxFailedGuesses)
}

func quickConnectClearGuessFailures(code string) {
	quickConnectFailureCounts.Delete(code)
}

func randomQuickConnectCode() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	b := make([]byte, 0, quickConnectCodeLength)
	for i := 0; i < quickConnectCodeLength; i++ {
		b = append(b, quickConnectAlphabet[int(raw[i])%len(quickConnectAlphabet)])
	}
	return string(b), nil
}

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
		TV:    env.String("PLUM_MEDIA_TV_PATH", "/tv"),
		Movie: env.String("PLUM_MEDIA_MOVIES_PATH", "/movies"),
		Anime: env.String("PLUM_MEDIA_ANIME_PATH", "/anime"),
		Music: env.String("PLUM_MEDIA_MUSIC_PATH", "/music"),
	}
}

func (h *AuthHandler) SetupStatus(w http.ResponseWriter, r *http.Request) {
	var count int
	err := h.DB.QueryRow(`SELECT COUNT(1) FROM users WHERE is_admin = 1`).Scan(&count)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(setupStatusResponse{
		HasAdmin:        count > 0,
		LibraryDefaults: setupLibraryDefaults(),
	}); err != nil {
		slog.Error("json encode error", "error", err)
	}
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

	var payload adminSetupRequest
	if !decodeRequestJSON(w, r, &payload) {
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
		`INSERT INTO users (email, password_hash, is_admin, created_at)
		 SELECT ?, ?, 1, ? WHERE NOT EXISTS (SELECT 1 FROM users WHERE is_admin = 1)
		 RETURNING id`,
		email, pwHash, now,
	).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		http.Error(w, "admin already exists", http.StatusConflict)
		return
	}
	if err != nil {
		_ = tx.Rollback()
		if isSQLiteUniqueConstraintError(err) {
			http.Error(w, "admin already exists", http.StatusConflict)
			return
		}
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
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("json encode error", "error", err)
	}
}

func isSQLiteUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint failed")
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
	if !decodeRequestJSON(w, r, &payload) {
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
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	hash := auth.LoginTimingMitigationHash
	if err == nil {
		hash = u.PasswordHash
	}
	if err := auth.CheckPasswordHash(payload.Password, hash); err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
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
	if err := json.NewEncoder(w).Encode(userResponse{
		ID:      u.ID,
		Email:   u.Email,
		IsAdmin: u.IsAdmin,
	}); err != nil {
		slog.Error("json encode error", "error", err)
	}
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
	if !decodeRequestJSON(w, r, &payload) {
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
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	hash := auth.LoginTimingMitigationHash
	if err == nil {
		hash = u.PasswordHash
	}
	if err := auth.CheckPasswordHash(payload.Password, hash); err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
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
	if err := json.NewEncoder(w).Encode(deviceLoginResponse{
		User: userResponse{
			ID:      u.ID,
			Email:   u.Email,
			IsAdmin: u.IsAdmin,
		},
		SessionToken: sessID,
		ExpiresAt:    expires,
	}); err != nil {
		slog.Error("json encode error", "error", err)
	}
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
	sessID := SessionIDFromContext(r.Context())
	if sessID != "" {
		now := time.Now().UTC()
		expires := now.Add(auth.SessionLifetime())
		if _, err := h.DB.Exec(`UPDATE sessions SET expires_at = ? WHERE id = ?`, expires, sessID); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if !AuthViaBearerFromContext(r.Context()) {
			setSessionCookie(w, r, sessID, expires)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(userResponse{
		ID:      user.ID,
		Email:   user.Email,
		IsAdmin: user.IsAdmin,
	}); err != nil {
		slog.Error("json encode error", "error", err)
	}
}

type quickConnectCodeResponse struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// CreateQuickConnectCode issues a short-lived alphanumeric code so a TV (or other device) can sign in as
// the current user without typing a password. Requires an authenticated session (same as the web UI).
func (h *AuthHandler) CreateQuickConnectCode(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	now := time.Now().UTC()
	_, _ = h.DB.Exec(`DELETE FROM quick_connect_codes WHERE expires_at < ?`, now.Unix())
	if _, err := h.DB.Exec(`DELETE FROM quick_connect_codes WHERE user_id = ?`, user.ID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	const maxTries = 32
	const ttl = 15 * time.Minute
	expires := now.Add(ttl)

	var code string
	for range maxTries {
		var err error
		code, err = randomQuickConnectCode()
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		res, err := h.DB.Exec(
			`INSERT OR IGNORE INTO quick_connect_codes (code, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`,
			code, user.ID, expires.Unix(), now.Unix(),
		)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if n, _ := res.RowsAffected(); n == 1 {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(quickConnectCodeResponse{Code: code, ExpiresAt: expires}); err != nil {
				slog.Error("json encode error", "error", err)
			}
			return
		}
	}
	http.Error(w, "could not allocate code", http.StatusServiceUnavailable)
}

type quickConnectRedeemRequest struct {
	Code string `json:"code"`
}

func normalizeQuickConnectCode(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') {
			b.WriteRune(r)
			if b.Len() >= quickConnectCodeLength {
				break
			}
		}
	}
	if b.Len() < quickConnectCodeLength {
		return ""
	}
	return b.String()
}

// RedeemQuickConnect exchanges a valid quick-connect code for a bearer session (same JSON as device-login).
func (h *AuthHandler) RedeemQuickConnect(w http.ResponseWriter, r *http.Request) {
	if !h.rateLimiter().Allow(clientIP(r), time.Now()) {
		http.Error(w, "too many attempts", http.StatusTooManyRequests)
		return
	}

	var payload quickConnectRedeemRequest
	if !decodeRequestJSON(w, r, &payload) {
		return
	}
	code := normalizeQuickConnectCode(payload.Code)
	if code == "" {
		http.Error(w, "invalid code", http.StatusBadRequest)
		return
	}

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC()
	// Best-effort purge of expired rows before lookup; ignore errors so redemption still proceeds.
	if _, err := tx.Exec(`DELETE FROM quick_connect_codes WHERE expires_at < ?`, now.Unix()); err != nil {
		slog.Debug("quick connect cleanup expired codes", "error", err)
	}

	var userID int
	var expUnix int64
	err = tx.QueryRow(
		`SELECT user_id, expires_at FROM quick_connect_codes WHERE code = ?`,
		code,
	).Scan(&userID, &expUnix)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if quickConnectRegisterFailedGuess(code) {
				http.Error(w, "too many invalid attempts for this code", http.StatusTooManyRequests)
				return
			}
			time.Sleep(300 * time.Millisecond)
			http.Error(w, "invalid or expired code", http.StatusUnauthorized)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	quickConnectClearGuessFailures(code)
	if expUnix <= 0 || now.Unix() > expUnix {
		_, _ = tx.Exec(`DELETE FROM quick_connect_codes WHERE code = ?`, code)
		_ = tx.Commit()
		time.Sleep(300 * time.Millisecond)
		http.Error(w, "invalid or expired code", http.StatusUnauthorized)
		return
	}

	if _, err := tx.Exec(`DELETE FROM quick_connect_codes WHERE code = ?`, code); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var u db.User
	err = tx.QueryRow(
		`SELECT id, email, password_hash, is_admin, created_at FROM users WHERE id = ?`,
		userID,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.IsAdmin, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "invalid or expired code", http.StatusUnauthorized)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	sessID, err := auth.NewSessionID()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	sessExpires := now.Add(auth.SessionLifetime())
	if _, err := tx.Exec(
		`INSERT INTO sessions (id, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		sessID, u.ID, now, sessExpires,
	); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(deviceLoginResponse{
		User: userResponse{
			ID:      u.ID,
			Email:   u.Email,
			IsAdmin: u.IsAdmin,
		},
		SessionToken: sessID,
		ExpiresAt:    sessExpires,
	}); err != nil {
		slog.Error("json encode error", "error", err)
	}
}

func (h *AuthHandler) rateLimiter() *AuthRateLimiter {
	if h != nil && h.Limiter != nil {
		return h.Limiter
	}
	return defaultAuthLimiter
}
