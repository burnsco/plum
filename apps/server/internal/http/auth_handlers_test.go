package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"plum/internal/auth"
	"plum/internal/db"
)

func TestAdminSetup_RollsBackUserWhenSessionInsertFails(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer dbConn.Close()

	if _, err := dbConn.Exec(`
CREATE TRIGGER fail_sessions_insert
BEFORE INSERT ON sessions
BEGIN
  SELECT RAISE(FAIL, 'session insert failed');
END;
`); err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	handler := &AuthHandler{DB: dbConn}
	req := httptest.NewRequest(http.MethodPost, "/api/auth/admin-setup", strings.NewReader(`{"email":"admin@example.com","password":"strong-password"}`))
	rec := httptest.NewRecorder()

	handler.AdminSetup(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}

	var count int
	if err := dbConn.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected rollback to remove inserted admin user, found %d users", count)
	}
}

func TestLogin_RateLimitsRepeatedAttempts(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer dbConn.Close()

	passwordHash, err := auth.HashPassword("correct-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if _, err := dbConn.Exec(
		`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?)`,
		"user@example.com",
		passwordHash,
		time.Now().UTC(),
	); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	handler := &AuthHandler{
		DB:      dbConn,
		Limiter: NewAuthRateLimiter(1, time.Hour),
	}

	firstReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"user@example.com","password":"wrong-password"}`))
	firstRec := httptest.NewRecorder()
	handler.Login(firstRec, firstReq)
	if firstRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected first attempt to fail with 401, got %d", firstRec.Code)
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"user@example.com","password":"wrong-password"}`))
	secondRec := httptest.NewRecorder()
	handler.Login(secondRec, secondReq)
	if secondRec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second attempt to be rate limited, got %d", secondRec.Code)
	}
}

func TestAdminSetup_RateLimitsRepeatedAttempts(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer dbConn.Close()

	handler := &AuthHandler{
		DB:      dbConn,
		Limiter: NewAuthRateLimiter(1, time.Hour),
	}

	firstReq := httptest.NewRequest(http.MethodPost, "/api/auth/admin-setup", strings.NewReader(`{"email":"admin@example.com","password":"short"}`))
	firstRec := httptest.NewRecorder()
	handler.AdminSetup(firstRec, firstReq)
	if firstRec.Code != http.StatusBadRequest {
		t.Fatalf("expected first admin setup attempt to fail validation, got %d", firstRec.Code)
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/api/auth/admin-setup", strings.NewReader(`{"email":"admin@example.com","password":"short"}`))
	secondRec := httptest.NewRecorder()
	handler.AdminSetup(secondRec, secondReq)
	if secondRec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second admin setup attempt to be rate limited, got %d", secondRec.Code)
	}
}
