package httpapi

import (
	"encoding/json"
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

func TestDeviceLogin_ReturnsSessionTokenJSON(t *testing.T) {
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
		"device@example.com",
		passwordHash,
		time.Now().UTC(),
	); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	handler := &AuthHandler{DB: dbConn}
	req := httptest.NewRequest(http.MethodPost, "/api/auth/device-login", strings.NewReader(`{"email":"device@example.com","password":"correct-password"}`))
	rec := httptest.NewRecorder()
	handler.DeviceLogin(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", rec.Code, rec.Body.String())
	}

	var payload struct {
		SessionToken string    `json:"sessionToken"`
		ExpiresAt    time.Time `json:"expiresAt"`
		User         struct {
			Email string `json:"email"`
		} `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json: %v", err)
	}
	if payload.SessionToken == "" {
		t.Fatal("expected non-empty sessionToken")
	}
	if payload.User.Email != "device@example.com" {
		t.Fatalf("user email = %q", payload.User.Email)
	}
	if payload.ExpiresAt.IsZero() {
		t.Fatal("expected expiresAt")
	}

	var count int
	if err := dbConn.QueryRow(`SELECT COUNT(*) FROM sessions WHERE id = ?`, payload.SessionToken).Scan(&count); err != nil {
		t.Fatalf("session lookup: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 session row, got %d", count)
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

func TestQuickConnect_CreateAndRedeem(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer dbConn.Close()

	passwordHash, err := auth.HashPassword("passwordpassword")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	res, err := dbConn.Exec(
		`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?)`,
		"admin@example.com",
		passwordHash,
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	uid, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}

	handler := &AuthHandler{DB: dbConn}

	createReq := httptest.NewRequest(http.MethodPost, "/api/auth/quick-connect", nil)
	createReq = createReq.WithContext(withUser(createReq.Context(), &db.User{
		ID:      int(uid),
		Email:   "admin@example.com",
		IsAdmin: true,
	}))
	createRec := httptest.NewRecorder()
	handler.CreateQuickConnectCode(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("create code: %d body=%q", createRec.Code, createRec.Body.String())
	}

	var created struct {
		Code      string `json:"code"`
		ExpiresAt string `json:"expiresAt"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if len(created.Code) != 6 {
		t.Fatalf("expected 6-character code, got %q", created.Code)
	}

	redeemReq := httptest.NewRequest(http.MethodPost, "/api/auth/quick-connect/redeem", strings.NewReader(`{"code":"`+created.Code+`"}`))
	redeemReq.Header.Set("Content-Type", "application/json")
	redeemRec := httptest.NewRecorder()
	handler.RedeemQuickConnect(redeemRec, redeemReq)
	if redeemRec.Code != http.StatusOK {
		t.Fatalf("redeem: %d body=%q", redeemRec.Code, redeemRec.Body.String())
	}

	var sessionPayload struct {
		SessionToken string `json:"sessionToken"`
		User         struct {
			Email string `json:"email"`
		} `json:"user"`
	}
	if err := json.Unmarshal(redeemRec.Body.Bytes(), &sessionPayload); err != nil {
		t.Fatalf("decode redeem: %v", err)
	}
	if sessionPayload.SessionToken == "" {
		t.Fatal("expected session token")
	}
	if sessionPayload.User.Email != "admin@example.com" {
		t.Fatalf("user email = %q", sessionPayload.User.Email)
	}

	redeemAgain := httptest.NewRequest(http.MethodPost, "/api/auth/quick-connect/redeem", strings.NewReader(`{"code":"`+created.Code+`"}`))
	redeemAgain.Header.Set("Content-Type", "application/json")
	againRec := httptest.NewRecorder()
	handler.RedeemQuickConnect(againRec, redeemAgain)
	if againRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected second redeem to fail, got %d", againRec.Code)
	}
}
