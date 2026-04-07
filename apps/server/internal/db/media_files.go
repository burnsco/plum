package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"
)

// UpdateMediaFileIntroFromProbe persists chapter-derived intro range on the primary media_files row.
// Call after a successful ffprobe when intro columns exist; clears intro columns when the probe found no intro chapter.
// Always sets intro_probed_at so "never probed" can be distinguished from "probed, no intro chapters".
func UpdateMediaFileIntroFromProbe(ctx context.Context, dbConn *sql.DB, mediaID int, path string, probed VideoProbeResult) error {
	_ = path // retained for API stability; row is targeted by media_id + primary selection
	if mediaID <= 0 {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	var start, end interface{}
	if probed.IntroStartSeconds != nil {
		start = *probed.IntroStartSeconds
	}
	if probed.IntroEndSeconds != nil {
		end = *probed.IntroEndSeconds
	}
	res, err := dbConn.ExecContext(ctx,
		`UPDATE media_files SET intro_start_sec = ?, intro_end_sec = ?, intro_probed_at = ?, updated_at = ? WHERE media_id = ? AND is_primary = 1`,
		start, end, now, now, mediaID,
	)
	if err != nil {
		if isMissingMediaFilesSchemaError(err) {
			return updateMediaFileIntroFromProbeWithoutProbedAtColumn(ctx, dbConn, mediaID, start, end, now)
		}
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	res2, err := dbConn.ExecContext(ctx, `
UPDATE media_files SET intro_start_sec = ?, intro_end_sec = ?, intro_probed_at = ?, updated_at = ?
WHERE id = (
  SELECT id FROM media_files WHERE media_id = ?
  ORDER BY is_primary DESC, COALESCE(missing_since, '') = '', id ASC
  LIMIT 1
)`, start, end, now, now, mediaID)
	if err != nil {
		return err
	}
	n2, err := res2.RowsAffected()
	if err != nil {
		return err
	}
	if n2 == 0 {
		slog.Warn("persist intro chapters matched no media_files row", "media_id", mediaID)
	}
	return nil
}

func updateMediaFileIntroFromProbeWithoutProbedAtColumn(ctx context.Context, dbConn *sql.DB, mediaID int, start, end interface{}, now string) error {
	res, err := dbConn.ExecContext(ctx,
		`UPDATE media_files SET intro_start_sec = ?, intro_end_sec = ?, updated_at = ? WHERE media_id = ? AND is_primary = 1`,
		start, end, now, mediaID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	res2, err := dbConn.ExecContext(ctx, `
UPDATE media_files SET intro_start_sec = ?, intro_end_sec = ?, updated_at = ?
WHERE id = (
  SELECT id FROM media_files WHERE media_id = ?
  ORDER BY is_primary DESC, COALESCE(missing_since, '') = '', id ASC
  LIMIT 1
)`, start, end, now, mediaID)
	if err != nil {
		return err
	}
	n2, err := res2.RowsAffected()
	if err != nil {
		return err
	}
	if n2 == 0 {
		slog.Warn("persist intro chapters matched no media_files row", "media_id", mediaID)
	}
	return nil
}

// posterURLRevisionQuery returns a v=… query so poster URLs change when poster_source changes,
// busting browser caches (see serveEntityArtwork Cache-Control).
func posterURLRevisionQuery(posterSource string) string {
	s := strings.TrimSpace(posterSource)
	if s == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(s))
	return "v=" + hex.EncodeToString(sum[:8])
}

// MediaItemPosterURL is the proxied poster URL for a media_global row (movies and episodes).
func MediaItemPosterURL(mediaGlobalID int, posterSource string) string {
	if mediaGlobalID <= 0 || strings.TrimSpace(posterSource) == "" {
		return ""
	}
	base := fmt.Sprintf("/api/media/%d/artwork/poster", mediaGlobalID)
	q := posterURLRevisionQuery(posterSource)
	if q == "" {
		return base
	}
	return base + "?" + q
}

// ShowPosterURL is the proxied show-level poster URL for library browse and search.
func ShowPosterURL(libraryID int, showKey string, posterSource string) string {
	showKey = strings.TrimSpace(showKey)
	if libraryID <= 0 || showKey == "" || strings.TrimSpace(posterSource) == "" {
		return ""
	}
	base := fmt.Sprintf("/api/libraries/%d/shows/%s/artwork/poster", libraryID, url.PathEscape(showKey))
	q := posterURLRevisionQuery(posterSource)
	if q == "" {
		return base
	}
	return base + "?" + q
}

func isMissingMediaFilesSchemaError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "no such table: media_files") || strings.Contains(text, "no such column:")
}

type mediaFileRow struct {
	MediaID         int
	Path            string
	FileSizeBytes   int64
	FileModTime     string
	FileHash        string
	FileHashKind    string
	Duration        int
	MissingSince    string
	LastSeenAt      string
	IsPrimary       bool
	IntroStartSec   sql.NullFloat64
	IntroEndSec     sql.NullFloat64
}

func decorateMediaItemURLs(item *MediaItem) {
	if item == nil || item.ID <= 0 {
		return
	}
	if item.PosterPath != "" {
		item.PosterURL = MediaItemPosterURL(item.ID, item.PosterPath)
	}
	if item.BackdropPath != "" {
		item.BackdropURL = fmt.Sprintf("/api/media/%d/artwork/backdrop", item.ID)
	}
	if item.Type == LibraryTypeTV || item.Type == LibraryTypeAnime {
		item.ThumbnailURL = fmt.Sprintf("/api/media/%d/thumbnail", item.ID)
		if item.ShowPosterPath != "" {
			item.ShowPosterURL = ShowPosterURL(
				item.LibraryID,
				showKeyFromItem(item.TMDBID, item.Title),
				item.ShowPosterPath,
			)
		}
	}
}

func attachMediaFilesBatch(dbConn *sql.DB, items []MediaItem) ([]MediaItem, error) {
	if len(items) == 0 {
		return items, nil
	}
	ids := make([]string, 0, len(items))
	index := make(map[int]int, len(items))
	args := make([]any, 0, len(items))
	for i := range items {
		index[items[i].ID] = i
		ids = append(ids, "?")
		args = append(args, items[i].ID)
	}
	query := `SELECT media_id, path, COALESCE(file_size_bytes, 0), COALESCE(file_mod_time, ''), COALESCE(file_hash, ''),
COALESCE(file_hash_kind, ''), COALESCE(duration, 0), COALESCE(missing_since, ''), COALESCE(last_seen_at, ''), COALESCE(is_primary, 0),
intro_start_sec, intro_end_sec
FROM media_files
WHERE media_id IN (` + strings.Join(ids, ",") + `)
ORDER BY is_primary DESC, COALESCE(missing_since, '') = '', id ASC`
	rows, err := dbConn.Query(query, args...)
	if err != nil {
		if isMissingMediaFilesSchemaError(err) {
			return attachMediaFilesBatchLegacy(dbConn, items)
		}
		return nil, err
	}
	defer rows.Close()

	seen := make(map[int]struct{}, len(items))
	for rows.Next() {
		var row mediaFileRow
		var isPrimary int
		if err := rows.Scan(
			&row.MediaID,
			&row.Path,
			&row.FileSizeBytes,
			&row.FileModTime,
			&row.FileHash,
			&row.FileHashKind,
			&row.Duration,
			&row.MissingSince,
			&row.LastSeenAt,
			&isPrimary,
			&row.IntroStartSec,
			&row.IntroEndSec,
		); err != nil {
			return nil, err
		}
		if _, ok := seen[row.MediaID]; ok {
			continue
		}
		seen[row.MediaID] = struct{}{}
		idx, ok := index[row.MediaID]
		if !ok {
			continue
		}
		items[idx].Path = row.Path
		if row.Duration > 0 {
			items[idx].Duration = row.Duration
		}
		items[idx].FileSizeBytes = row.FileSizeBytes
		items[idx].FileModTime = row.FileModTime
		items[idx].FileHash = row.FileHash
		items[idx].FileHashKind = row.FileHashKind
		items[idx].MissingSince = row.MissingSince
		items[idx].Missing = row.MissingSince != ""
		if row.IntroStartSec.Valid {
			v := row.IntroStartSec.Float64
			items[idx].IntroStartSeconds = &v
		}
		if row.IntroEndSec.Valid {
			v := row.IntroEndSec.Float64
			items[idx].IntroEndSeconds = &v
		}
	}
	for i := range items {
		decorateMediaItemURLs(&items[i])
	}
	return items, rows.Err()
}

func attachMediaFilesBatchLegacy(dbConn *sql.DB, items []MediaItem) ([]MediaItem, error) {
	if len(items) == 0 {
		return items, nil
	}
	ids := make([]string, 0, len(items))
	index := make(map[int]int, len(items))
	args := make([]any, 0, len(items))
	for i := range items {
		index[items[i].ID] = i
		ids = append(ids, "?")
		args = append(args, items[i].ID)
	}
	query := `SELECT media_id, path, COALESCE(file_size_bytes, 0), COALESCE(file_mod_time, ''), COALESCE(file_hash, ''),
COALESCE(file_hash_kind, ''), COALESCE(duration, 0), COALESCE(missing_since, ''), COALESCE(last_seen_at, ''), COALESCE(is_primary, 0)
FROM media_files
WHERE media_id IN (` + strings.Join(ids, ",") + `)
ORDER BY is_primary DESC, COALESCE(missing_since, '') = '', id ASC`
	rows, err := dbConn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seen := make(map[int]struct{}, len(items))
	for rows.Next() {
		var row mediaFileRow
		var isPrimary int
		if err := rows.Scan(
			&row.MediaID,
			&row.Path,
			&row.FileSizeBytes,
			&row.FileModTime,
			&row.FileHash,
			&row.FileHashKind,
			&row.Duration,
			&row.MissingSince,
			&row.LastSeenAt,
			&isPrimary,
		); err != nil {
			return nil, err
		}
		if _, ok := seen[row.MediaID]; ok {
			continue
		}
		seen[row.MediaID] = struct{}{}
		idx, ok := index[row.MediaID]
		if !ok {
			continue
		}
		items[idx].Path = row.Path
		if row.Duration > 0 {
			items[idx].Duration = row.Duration
		}
		items[idx].FileSizeBytes = row.FileSizeBytes
		items[idx].FileModTime = row.FileModTime
		items[idx].FileHash = row.FileHash
		items[idx].FileHashKind = row.FileHashKind
		items[idx].MissingSince = row.MissingSince
		items[idx].Missing = row.MissingSince != ""
	}
	for i := range items {
		decorateMediaItemURLs(&items[i])
	}
	return items, rows.Err()
}

func upsertMediaFileForMediaID(ctx context.Context, dbConn *sql.DB, mediaID int, item MediaItem, primary bool) error {
	if mediaID <= 0 || strings.TrimSpace(item.Path) == "" {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if primary {
		if _, err := dbConn.ExecContext(ctx, `UPDATE media_files SET is_primary = 0, updated_at = ? WHERE media_id = ?`, now, mediaID); err != nil {
			return err
		}
	}
	_, err := dbConn.ExecContext(ctx, `INSERT INTO media_files (
media_id, path, file_size_bytes, file_mod_time, file_hash, file_hash_kind, duration, missing_since, last_seen_at, is_primary, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(path) DO UPDATE SET
media_id = excluded.media_id,
file_size_bytes = excluded.file_size_bytes,
file_mod_time = excluded.file_mod_time,
file_hash = excluded.file_hash,
file_hash_kind = excluded.file_hash_kind,
duration = excluded.duration,
missing_since = excluded.missing_since,
last_seen_at = excluded.last_seen_at,
is_primary = excluded.is_primary,
updated_at = excluded.updated_at`,
		mediaID,
		item.Path,
		item.FileSizeBytes,
		nullStr(item.FileModTime),
		nullStr(item.FileHash),
		nullStr(item.FileHashKind),
		item.Duration,
		nullStr(item.MissingSince),
		now,
		boolToInt(primary),
		now,
		now,
	)
	return err
}

func lookupPrimaryMediaFile(dbConn *sql.DB, mediaID int) (mediaFileRow, error) {
	var row mediaFileRow
	err := dbConn.QueryRow(
		`SELECT media_id, path, COALESCE(file_size_bytes, 0), COALESCE(file_mod_time, ''), COALESCE(file_hash, ''),
		        COALESCE(file_hash_kind, ''), COALESCE(duration, 0), COALESCE(missing_since, ''), COALESCE(last_seen_at, ''), COALESCE(is_primary, 0),
		        intro_start_sec, intro_end_sec
		   FROM media_files
		  WHERE media_id = ?
		  ORDER BY is_primary DESC, COALESCE(missing_since, '') = '', id ASC
		  LIMIT 1`,
		mediaID,
	).Scan(
		&row.MediaID,
		&row.Path,
		&row.FileSizeBytes,
		&row.FileModTime,
		&row.FileHash,
		&row.FileHashKind,
		&row.Duration,
		&row.MissingSince,
		&row.LastSeenAt,
		&row.IsPrimary,
		&row.IntroStartSec,
		&row.IntroEndSec,
	)
	if err != nil {
		if isMissingMediaFilesSchemaError(err) {
			return lookupPrimaryMediaFileLegacy(dbConn, mediaID)
		}
		return row, err
	}
	return row, nil
}

func lookupPrimaryMediaFileLegacy(dbConn *sql.DB, mediaID int) (mediaFileRow, error) {
	var row mediaFileRow
	err := dbConn.QueryRow(
		`SELECT media_id, path, COALESCE(file_size_bytes, 0), COALESCE(file_mod_time, ''), COALESCE(file_hash, ''),
		        COALESCE(file_hash_kind, ''), COALESCE(duration, 0), COALESCE(missing_since, ''), COALESCE(last_seen_at, ''), COALESCE(is_primary, 0)
		   FROM media_files
		  WHERE media_id = ?
		  ORDER BY is_primary DESC, COALESCE(missing_since, '') = '', id ASC
		  LIMIT 1`,
		mediaID,
	).Scan(
		&row.MediaID,
		&row.Path,
		&row.FileSizeBytes,
		&row.FileModTime,
		&row.FileHash,
		&row.FileHashKind,
		&row.Duration,
		&row.MissingSince,
		&row.LastSeenAt,
		&row.IsPrimary,
	)
	return row, err
}
