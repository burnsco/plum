package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// app_settings keys for runtime server-env overrides (avoid os.Setenv races).
const appSettingsKeyServerEnvPrefix = "server_env:"

// ServerEnvAppSettingKey returns the app_settings row key for a process env var name.
func ServerEnvAppSettingKey(envVarName string) string {
	return appSettingsKeyServerEnvPrefix + envVarName
}

// GetServerEnvOverrides loads all persisted server-env overrides. Keys are env var names
// (e.g. "TMDB_API_KEY"); values are the stored strings (may be empty when cleared via UI).
func GetServerEnvOverrides(dbConn *sql.DB) (map[string]string, error) {
	if dbConn == nil {
		return nil, errors.New("nil db")
	}
	rows, err := dbConn.Query(`SELECT key, value FROM app_settings WHERE key LIKE ?`, appSettingsKeyServerEnvPrefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]string)
	prefix := appSettingsKeyServerEnvPrefix
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		envName := strings.TrimPrefix(k, prefix)
		if envName != "" {
			out[envName] = v
		}
	}
	return out, rows.Err()
}

// UpsertServerEnvOverrides persists override values for keys in updates. Empty string means
// the key was explicitly cleared in the UI (runtime effective value is empty).
func UpsertServerEnvOverrides(ctx context.Context, dbConn *sql.DB, updates map[string]string) error {
	if dbConn == nil {
		return errors.New("nil db")
	}
	if len(updates) == 0 {
		return nil
	}
	tx, err := dbConn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	now := time.Now().UTC().Format(time.RFC3339)
	const q = `
INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`
	for k, v := range updates {
		if _, err := tx.ExecContext(ctx, q, ServerEnvAppSettingKey(k), v, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}
