package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
