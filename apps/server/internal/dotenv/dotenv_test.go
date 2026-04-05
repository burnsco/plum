package dotenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpsertReplaceAppendAndRemove(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")

	if err := Upsert(p, map[string]string{
		"PLUM_ADDR":    ":9090",
		"TMDB_API_KEY": "secret",
		"OMDB_API_KEY": "omdb",
		"PLUM_DB_PATH": "./old.db",
	}); err != nil {
		t.Fatal(err)
	}

	if err := Upsert(p, map[string]string{
		"PLUM_ADDR":    ":8080",
		"TMDB_API_KEY": "",
		"NEW_KEY":      "x",
	}); err != nil {
		t.Fatal(err)
	}

	body, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "PLUM_ADDR=:8080") {
		t.Fatalf("expected PLUM_ADDR updated, got:\n%s", s)
	}
	if strings.Contains(s, "TMDB_API_KEY") {
		t.Fatalf("expected TMDB_API_KEY removed, got:\n%s", s)
	}
	if !strings.Contains(s, "NEW_KEY=x") {
		t.Fatalf("expected NEW_KEY appended, got:\n%s", s)
	}
}

func TestReadKeyValues(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	_ = os.WriteFile(p, []byte("TMDB_API_KEY=abc\n# comment\nTVDB_API_KEY=\"q\\\"t\"\n"), 0o600)

	want := map[string]struct{}{
		"TMDB_API_KEY": {},
		"TVDB_API_KEY": {},
	}
	got := ReadKeyValues(p, want)
	if got["TMDB_API_KEY"] != "abc" {
		t.Fatalf("tmdb: %q", got["TMDB_API_KEY"])
	}
	if got["TVDB_API_KEY"] != `q\"t` {
		t.Fatalf("tvdb: %q", got["TVDB_API_KEY"])
	}
}
