package db

import (
	"context"
	"database/sql"
	"time"
)

const expiredSessionCleanupInterval = time.Hour

func StartSessionCleanup(ctx context.Context, dbConn *sql.DB, logger func(string, ...any)) {
	if dbConn == nil {
		return
	}

	run := func() {
		if err := deleteExpiredSessions(ctx, dbConn); err != nil && logger != nil && ctx.Err() == nil {
			logger("cleanup expired sessions: %v", err)
		}
	}

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(30 * time.Second):
		}
		run()
		ticker := time.NewTicker(expiredSessionCleanupInterval)
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

func deleteExpiredSessions(ctx context.Context, dbConn *sql.DB) error {
	return RetryOnBusy(ctx, 4, 500*time.Millisecond, func() error {
		_, err := dbConn.ExecContext(
			ctx,
			`DELETE FROM sessions WHERE expires_at < ?`,
			time.Now().UTC(),
		)
		return err
	})
}
