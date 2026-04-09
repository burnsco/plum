package db

import (
	"context"
	"testing"
)

func TestServerEnvOverridesRoundTrip(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	ctx := context.Background()
	updates := map[string]string{
		"TMDB_API_KEY": "secret",
		"TVDB_API_KEY": "",
	}
	if err := UpsertServerEnvOverrides(ctx, db, updates); err != nil {
		t.Fatal(err)
	}
	got, err := GetServerEnvOverrides(db)
	if err != nil {
		t.Fatal(err)
	}
	if got["TMDB_API_KEY"] != "secret" {
		t.Fatalf("tmdb: %q", got["TMDB_API_KEY"])
	}
	if v, ok := got["TVDB_API_KEY"]; !ok || v != "" {
		t.Fatalf("tvdb: ok=%v v=%q", ok, v)
	}
}
