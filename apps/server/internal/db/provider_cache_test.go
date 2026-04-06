package db

import (
	"context"
	"testing"
	"time"

	"plum/internal/metadata"
)

func TestMetadataProviderCacheStore_PutAndGet(t *testing.T) {
	dbConn := newTestDB(t)
	store := NewMetadataProviderCacheStore(dbConn)
	now := time.Now().UTC()
	key := metadata.ProviderCacheKey{
		Provider:  "tmdb",
		Method:    "GET",
		URLPath:   "/search/tv",
		QueryHash: "q1",
		BodyHash:  "b1",
	}
	entry := metadata.ProviderCacheEntry{
		ResponseJSON:  []byte(`{"results":[1]}`),
		StatusCode:    200,
		FetchedAt:     now,
		ExpiresAt:     now.Add(time.Hour),
		SchemaVersion: 1,
		ContentHash:   "hash-a",
	}
	if err := store.Put(context.Background(), key, entry, now); err != nil {
		t.Fatalf("put cache entry: %v", err)
	}
	got, found, err := store.Get(context.Background(), key, now.Add(5*time.Minute))
	if err != nil {
		t.Fatalf("get cache entry: %v", err)
	}
	if !found || got == nil {
		t.Fatalf("expected cache hit")
	}
	if got.StatusCode != entry.StatusCode || string(got.ResponseJSON) != string(entry.ResponseJSON) {
		t.Fatalf("unexpected cache entry: %#v", got)
	}
}

func TestCleanupMetadataProviderCache_RemovesExpiredRows(t *testing.T) {
	dbConn := newTestDB(t)
	ctx := context.Background()
	past := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339)
	future := time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339)
	_, err := dbConn.ExecContext(ctx, `INSERT INTO metadata_provider_cache (
provider, method, url_path, query_hash, body_hash, response_json, fetched_at, expires_at, schema_version, content_hash, status_code, last_accessed_at, hit_count
) VALUES ('tmdb', 'GET', '/x', 'q', 'b', ?, ?, ?, 1, 'h', 200, ?, 0)`,
		[]byte(`{}`), past, past, past)
	if err != nil {
		t.Fatalf("insert expired: %v", err)
	}
	_, err = dbConn.ExecContext(ctx, `INSERT INTO metadata_provider_cache (
provider, method, url_path, query_hash, body_hash, response_json, fetched_at, expires_at, schema_version, content_hash, status_code, last_accessed_at, hit_count
) VALUES ('tmdb', 'GET', '/y', 'q', 'b', ?, ?, ?, 1, 'h', 200, ?, 0)`,
		[]byte(`{}`), future, future, future)
	if err != nil {
		t.Fatalf("insert fresh: %v", err)
	}
	if err := cleanupMetadataProviderCache(ctx, dbConn); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	var n int
	if err := dbConn.QueryRowContext(ctx, `SELECT COUNT(*) FROM metadata_provider_cache`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 row after cleanup, got %d", n)
	}
}

func TestCleanupMetadataProviderCache_TrimByMaxRows(t *testing.T) {
	t.Setenv("PLUM_METADATA_CACHE_MAX_ROWS", "2")
	dbConn := newTestDB(t)
	ctx := context.Background()
	future := time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339)
	for i, accessed := range []string{"2020-01-01T00:00:00Z", "2021-01-01T00:00:00Z", "2022-01-01T00:00:00Z"} {
		path := "/p/" + string(rune('a'+i))
		_, err := dbConn.ExecContext(ctx, `INSERT INTO metadata_provider_cache (
provider, method, url_path, query_hash, body_hash, response_json, fetched_at, expires_at, schema_version, content_hash, status_code, last_accessed_at, hit_count
) VALUES ('tmdb', 'GET', ?, 'q', 'b', ?, ?, ?, 1, 'h', 200, ?, 0)`,
			path, []byte(`{}`), future, future, accessed)
		if err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}
	if err := cleanupMetadataProviderCache(ctx, dbConn); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	var n int
	if err := dbConn.QueryRowContext(ctx, `SELECT COUNT(*) FROM metadata_provider_cache`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 rows after trim, got %d", n)
	}
	var oldest string
	if err := dbConn.QueryRowContext(ctx, `SELECT MIN(last_accessed_at) FROM metadata_provider_cache`).Scan(&oldest); err != nil {
		t.Fatalf("min accessed: %v", err)
	}
	if oldest != "2021-01-01T00:00:00Z" {
		t.Fatalf("expected oldest remaining 2021-01-01, got %q", oldest)
	}
}
