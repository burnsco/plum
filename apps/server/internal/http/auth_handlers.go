package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"plum/internal/auth"
	"plum/internal/db"
	"plum/internal/logging"
)

type AuthHandler struct {
	DB      *sql.DB
	Limiter *AuthRateLimiter
}

type setupStatusResponse struct {
	HasAdmin bool `json:"hasAdmin"`
}

func (h *AuthHandler) SetupStatus(w http.ResponseWriter, r *http.Request) {
	var count int
	err := h.DB.QueryRow(`SELECT COUNT(1) FROM users WHERE is_admin = 1 OR role = ?`, db.UserRoleAdmin).Scan(&count)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(setupStatusResponse{HasAdmin: count > 0})
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
	if err := h.DB.QueryRow(`SELECT COUNT(1) FROM users WHERE is_admin = 1 OR role = ?`, db.UserRoleAdmin).Scan(&existing); err != nil {
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
		`INSERT INTO users (email, password_hash, role, is_admin, created_at) VALUES (?, ?, ?, 1, ?) RETURNING id`,
		email, pwHash, db.UserRoleAdmin, now,
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
		Role:      db.UserRoleAdmin,
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
	Role    string `json:"role"`
	IsAdmin bool   `json:"is_admin"`
}

type managedUserResponse struct {
	ID         int    `json:"id"`
	Email      string `json:"email"`
	Role       string `json:"role"`
	IsAdmin    bool   `json:"is_admin"`
	LibraryIDs []int  `json:"libraryIds"`
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
		`SELECT id, email, password_hash, role, is_admin, created_at FROM users WHERE email = ?`,
		email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.IsAdmin, &u.CreatedAt)
	if err != nil {
		logging.Event("auth", "login_failed", logging.Fields{"email": email, "reason": "not_found"})
		time.Sleep(500 * time.Millisecond)
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	u.Role = db.NormalizeUserRole(u.Role)
	u.IsAdmin = db.IsAdminRole(u.Role) || u.IsAdmin

	if err := auth.CheckPasswordHash(payload.Password, u.PasswordHash); err != nil {
		logging.Event("auth", "login_failed", logging.Fields{"email": email, "reason": "invalid_password"})
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
		Role:    u.Role,
		IsAdmin: u.IsAdmin,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	sessID := sessionIDFromRequest(r)
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
		Role:    db.NormalizeUserRole(user.Role),
		IsAdmin: user.IsAdmin,
	})
}

func (h *AuthHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := db.ListManagedUsers(h.DB)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	response := make([]managedUserResponse, 0, len(users))
	for _, user := range users {
		response = append(response, managedUserResponse{
			ID:         user.ID,
			Email:      user.Email,
			Role:       db.NormalizeUserRole(user.Role),
			IsAdmin:    user.IsAdmin,
			LibraryIDs: append([]int(nil), user.LibraryIDs...),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

type createUserRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	Role       string `json:"role"`
	LibraryIDs []int  `json:"libraryIds"`
}

func (h *AuthHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	currentUser := UserFromContext(r.Context())
	if currentUser == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var payload createUserRequest
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

	role := db.NormalizeUserRole(payload.Role)
	if role != payload.Role && !strings.EqualFold(strings.TrimSpace(payload.Role), db.UserRoleViewer) {
		http.Error(w, "invalid role", http.StatusBadRequest)
		return
	}

	libraryIDs := append([]int(nil), payload.LibraryIDs...)
	slices.Sort(libraryIDs)
	libraryIDs = slices.Compact(libraryIDs)
	for _, libraryID := range libraryIDs {
		var exists int
		if err := h.DB.QueryRow(`SELECT 1 FROM libraries WHERE id = ?`, libraryID).Scan(&exists); err != nil {
			http.Error(w, "invalid libraryIds", http.StatusBadRequest)
			return
		}
		allowed, err := db.UserHasLibraryAccess(h.DB, currentUser.ID, libraryID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if !allowed {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	passwordHash, err := auth.HashPassword(payload.Password)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	createdAt := time.Now().UTC()
	user, err := db.CreateUserWithLibraries(r.Context(), h.DB, email, passwordHash, role, libraryIDs, createdAt)
	if err != nil {
		logging.Event("auth", "create_user_failed", logging.Fields{
			"email":       email,
			"role":        role,
			"library_ids": strings.Trim(strings.Join(strings.Fields(fmt.Sprint(libraryIDs)), ","), "[]"),
			"error":       err.Error(),
		})
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			http.Error(w, "email already exists", http.StatusConflict)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	logging.Event("auth", "user_created", logging.Fields{
		"user_id":     user.ID,
		"email":       user.Email,
		"role":        user.Role,
		"library_ids": fmt.Sprint(user.LibraryIDs),
	})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(managedUserResponse{
		ID:         user.ID,
		Email:      user.Email,
		Role:       user.Role,
		IsAdmin:    user.IsAdmin,
		LibraryIDs: append([]int(nil), user.LibraryIDs...),
	})
}

func (h *AuthHandler) rateLimiter() *AuthRateLimiter {
	if h != nil && h.Limiter != nil {
		return h.Limiter
	}
	return defaultAuthLimiter
}
