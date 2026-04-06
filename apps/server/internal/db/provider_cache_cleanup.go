package db

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"strings"
	"time"
)

const metadataProviderCacheCleanupInterval = 6 * time.Hour

// StartMetadataProviderCacheCleanup periodically deletes expired rows from metadata_provider_cache.
// If PLUM_METADATA_CACHE_MAX_ROWS is set to a positive integer, after expiry cleanup the store is
// trimmed by deleting least-recently-accessed rows until the row count is at most that limit.
func StartMetadataProviderCacheCleanup(ctx context.Context, dbConn *sql.DB, logger func(string, ...any)) {
	if dbConn == nil {
		return
	}
	run := func() {
		if err := cleanupMetadataProviderCache(ctx, dbConn); err != nil && logger != nil && ctx.Err() == nil {
			logger("cleanup metadata provider cache: %v", err)
		}
	}
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Minute):
		}
		run()
		ticker := time.NewTicker(metadataProviderCacheCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run()
			}
		}
	}()
}

// RunMetadataProviderCacheCleanup deletes expired metadata_provider_cache rows and applies
// PLUM_METADATA_CACHE_MAX_ROWS trimming. Safe to call manually from admin maintenance.
func RunMetadataProviderCacheCleanup(ctx context.Context, dbConn *sql.DB) error {
	return cleanupMetadataProviderCache(ctx, dbConn)
}

func cleanupMetadataProviderCache(ctx context.Context, dbConn *sql.DB) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if err := RetryOnBusy(ctx, 4, 500*time.Millisecond, func() error {
		_, err := dbConn.ExecContext(ctx, `DELETE FROM metadata_provider_cache WHERE expires_at < ?`, now)
		return err
	}); err != nil {
		return err
	}
	maxRows := metadataProviderCacheMaxRowsFromEnv()
	if maxRows <= 0 {
		return nil
	}
	var cnt int
	if err := dbConn.QueryRowContext(ctx, `SELECT COUNT(*) FROM metadata_provider_cache`).Scan(&cnt); err != nil {
		return err
	}
	if cnt <= maxRows {
		return nil
	}
	toDelete := cnt - maxRows
	return RetryOnBusy(ctx, 4, 500*time.Millisecond, func() error {
		_, err := dbConn.ExecContext(ctx, `
DELETE FROM metadata_provider_cache WHERE rowid IN (
  SELECT rowid FROM metadata_provider_cache ORDER BY last_accessed_at ASC LIMIT ?
)`, toDelete)
		return err
	})
}

func metadataProviderCacheMaxRowsFromEnv() int {
	raw := strings.TrimSpace(os.Getenv("PLUM_METADATA_CACHE_MAX_ROWS"))
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0
	}
	return n
}
