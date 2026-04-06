package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func pathLooksLikeMediaFile(absPath string) bool {
	ext := strings.ToLower(filepath.Ext(absPath))
	if ext == "" {
		return false
	}
	if _, ok := videoExtensions[ext]; ok {
		return true
	}
	_, ok := audioExtensions[ext]
	return ok
}

func escapeSQLLikePrefix(prefix string) string {
	s := strings.ReplaceAll(prefix, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// MarkMediaMissingForFilesystemPaths sets missing_since on library rows whose paths match
// filesystem paths that are already absent (deleted directories or files). This mirrors
// discovery's markMissingMedia outcome without waiting for a full scan.
//
// For paths that look like media files (known video/audio extension), only an exact path
// match is updated. For other paths (typical directory removes), rows matching the path
// prefix (path = p OR children under p/) are updated.
func MarkMediaMissingForFilesystemPaths(ctx context.Context, dbConn *sql.DB, libraryID int, libraryRoot string, absentPaths []string) (int, error) {
	if libraryID <= 0 || dbConn == nil {
		return 0, nil
	}
	libraryRoot = filepath.Clean(libraryRoot)
	if libraryRoot == "" {
		return 0, nil
	}

	var libType string
	if err := dbConn.QueryRowContext(ctx, `SELECT type FROM libraries WHERE id = ?`, libraryID).Scan(&libType); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}

	table := mediaTableForKind(libType)
	kind := libType
	now := time.Now().UTC().Format(time.RFC3339)

	seen := make(map[string]struct{})
	var refIDs []int
	seenRef := make(map[int]struct{})

	for _, raw := range absentPaths {
		p := filepath.Clean(strings.TrimSpace(raw))
		if p == "" || p == "." {
			continue
		}
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		if !pathWithinAnyRoot(p, []string{libraryRoot}) {
			continue
		}

		var rows *sql.Rows
		var err error
		if pathLooksLikeMediaFile(p) {
			rows, err = dbConn.QueryContext(ctx,
				fmt.Sprintf(`SELECT id FROM %s WHERE library_id = ? AND COALESCE(missing_since, '') = '' AND path = ?`, table),
				libraryID, p,
			)
		} else {
			likePat := escapeSQLLikePrefix(p+string(os.PathSeparator)) + "%"
			// SQLite requires a single-character ESCAPE; build `ESCAPE '\'` for backslash.
			likeClause := fmt.Sprintf(
				`SELECT id FROM %s WHERE library_id = ? AND COALESCE(missing_since, '') = '' AND (path = ? OR path LIKE ? ESCAPE '`+`\`+`')`,
				table,
			)
			rows, err = dbConn.QueryContext(ctx, likeClause, libraryID, p, likePat)
		}
		if err != nil {
			return len(refIDs), err
		}
		for rows.Next() {
			var id int
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return len(refIDs), err
			}
			if _, ok := seenRef[id]; ok {
				continue
			}
			seenRef[id] = struct{}{}
			refIDs = append(refIDs, id)
		}
		if err := rows.Close(); err != nil {
			return len(refIDs), err
		}
		if err := rows.Err(); err != nil {
			return len(refIDs), err
		}
	}

	if len(refIDs) == 0 {
		return 0, nil
	}
	if err := batchUpdateMissingMedia(ctx, dbConn, table, kind, refIDs, now); err != nil {
		return 0, err
	}
	return len(refIDs), nil
}
