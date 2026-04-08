package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"plum/internal/db"
	"plum/internal/metadata"
)

func TestServerEnvSettingsHandler_GetAndPut(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("PLUM_DOTENV_PATH", filepath.Join(tmp, ".env"))
	_ = os.WriteFile(filepath.Join(tmp, ".env"), []byte("PLUM_ADDR=:9090\nTMDB_API_KEY=oldkey\n"), 0o600)

	t.Setenv("PLUM_ADDR", "")
	t.Setenv("TMDB_API_KEY", "")

	p := metadata.NewPipeline("", "", "", "", "")
	h := &ServerEnvSettingsHandler{Pipeline: p}

	getReq := httptest.NewRequest(http.MethodGet, "/api/settings/server-env", nil)
	getReq = getReq.WithContext(withUser(getReq.Context(), &db.User{ID: 1, IsAdmin: true}))
	getRec := httptest.NewRecorder()
	h.Get(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status=%d %s", getRec.Code, getRec.Body.String())
	}
	var getBody struct {
		PLUMAddr       string `json:"plum_addr"`
		SecretsPresent struct {
			TMDB bool `json:"tmdb_api_key"`
		} `json:"secrets_present"`
		EnvFileWritable bool `json:"env_file_writable"`
	}
	if err := json.NewDecoder(getRec.Body).Decode(&getBody); err != nil {
		t.Fatal(err)
	}
	if getBody.PLUMAddr != ":9090" {
		t.Fatalf("plum_addr=%q", getBody.PLUMAddr)
	}
	if !getBody.SecretsPresent.TMDB {
		t.Fatal("expected tmdb secret present from file")
	}
	if !getBody.EnvFileWritable {
		t.Fatal("expected writable .env in temp dir")
	}

	putPayload := map[string]any{
		"plum_addr":          ":8080",
		"tmdb_api_key":       "newsecret",
		"omdb_api_key":       "omdbval",
		"tvdb_api_key_clear": true,
	}
	putRaw, _ := json.Marshal(putPayload)
	putReq := httptest.NewRequest(http.MethodPut, "/api/settings/server-env", bytes.NewReader(putRaw))
	putReq = putReq.WithContext(withUser(putReq.Context(), &db.User{ID: 1, IsAdmin: true}))
	putRec := httptest.NewRecorder()
	h.Put(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("put status=%d %s", putRec.Code, putRec.Body.String())
	}

	if os.Getenv("TMDB_API_KEY") != "newsecret" {
		t.Fatalf("process TMDB_API_KEY=%q", os.Getenv("TMDB_API_KEY"))
	}
	if os.Getenv("OMDB_API_KEY") != "omdbval" {
		t.Fatalf("process OMDB_API_KEY=%q", os.Getenv("OMDB_API_KEY"))
	}

	disk, _ := os.ReadFile(filepath.Join(tmp, ".env"))
	s := string(disk)
	if !strings.Contains(s, "PLUM_ADDR=:8080") || !strings.Contains(s, "TMDB_API_KEY=") {
		t.Fatalf("unexpected .env:\n%s", s)
	}
	if strings.Contains(s, "TVDB_API_KEY") {
		t.Fatalf("expected TVDB cleared from .env:\n%s", s)
	}
}
