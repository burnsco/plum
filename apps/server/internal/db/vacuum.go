package db

import (
	"context"
	"database/sql"
)

// RunSQLiteVacuum runs VACUUM on the open database pool. This requires a brief exclusive lock
// and can take noticeable time on large libraries; callers should run it from a background task.
func RunSQLiteVacuum(ctx context.Context, dbConn *sql.DB) error {
	if dbConn == nil {
		return nil
	}
	_, err := dbConn.ExecContext(ctx, "VACUUM")
	return err
}
