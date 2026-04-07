package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"
)

// MediaFileIntroLocked is true when the primary media_files row has intro_locked set (user-defined bounds).
func MediaFileIntroLocked(ctx context.Context, dbConn *sql.DB, mediaID int) (bool, error) {
	if mediaID <= 0 {
		return false, nil
	}
	var n int
	err := dbConn.QueryRowContext(ctx,
		`SELECT COALESCE(intro_locked, 0) FROM media_files WHERE media_id = ? AND is_primary = 1`,
		mediaID,
	).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		if isMissingMediaFilesSchemaError(err) {
			return false, nil
		}
		return false, err
	}
	return n != 0, nil
}

// UpdateMediaFileIntroFromProbe persists intro range (chapter markers and/or silence detection) on the primary media_files row.
// Call after a successful metadata probe; clears intro columns when no intro window was found.
// Always sets intro_probed_at so "never probed" can be distinguished from "probed, no intro".
// Skips updates when intro_locked is set so manual bounds are preserved.
func UpdateMediaFileIntroFromProbe(ctx context.Context, dbConn *sql.DB, mediaID int, path string, probed VideoProbeResult) error {
	_ = path // retained for API stability; row is targeted by media_id + primary selection
	if mediaID <= 0 {
		return nil
	}
	locked, err := MediaFileIntroLocked(ctx, dbConn, mediaID)
	if err != nil {
		return err
	}
	if locked {
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
	IntroLocked     int
	CreditsStartSec sql.NullFloat64
	CreditsEndSec   sql.NullFloat64
}

// ApplyPrimaryMediaIntroCreditsToItem copies intro/credits fields from the primary media_files row onto item.
func ApplyPrimaryMediaIntroCreditsToItem(dbConn *sql.DB, item *MediaItem) error {
	if item == nil || item.ID <= 0 {
		return nil
	}
	row, err := lookupPrimaryMediaFile(dbConn, item.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	applyMediaFileRowIntroCredits(item, row)
	return nil
}

func applyMediaFileRowIntroCredits(item *MediaItem, row mediaFileRow) {
	item.IntroLocked = row.IntroLocked != 0
	if row.IntroStartSec.Valid {
		v := row.IntroStartSec.Float64
		item.IntroStartSeconds = &v
	} else {
		item.IntroStartSeconds = nil
	}
	if row.IntroEndSec.Valid {
		v := row.IntroEndSec.Float64
		item.IntroEndSeconds = &v
	} else {
		item.IntroEndSeconds = nil
	}
	if row.CreditsStartSec.Valid {
		v := row.CreditsStartSec.Float64
		item.CreditsStartSeconds = &v
	} else {
		item.CreditsStartSeconds = nil
	}
	if row.CreditsEndSec.Valid {
		v := row.CreditsEndSec.Float64
		item.CreditsEndSeconds = &v
	} else {
		item.CreditsEndSeconds = nil
	}
}

// PatchMediaPlaybackSegments updates intro/credits bounds on the primary media_files row.
// When clear_intro is true, intro columns and lock are cleared and intro_probed_at is cleared.
// When intro_start_seconds or intro_end_seconds are sent, intro_locked defaults to true unless intro_locked is explicitly false.
func PatchMediaPlaybackSegments(ctx context.Context, dbConn *sql.DB, mediaID int, introStart, introEnd *float64, introLocked *bool, clearIntro bool, creditsStart, creditsEnd *float64, clearCredits bool) error {
	if mediaID <= 0 {
		return fmt.Errorf("invalid media id")
	}
	now := time.Now().UTC().Format(time.RFC3339)

	if clearIntro {
		res, err := dbConn.ExecContext(ctx,
			`UPDATE media_files SET intro_start_sec = NULL, intro_end_sec = NULL, intro_probed_at = NULL, intro_locked = 0, updated_at = ? WHERE media_id = ? AND is_primary = 1`,
			now, mediaID,
		)
		if err != nil {
			return err
		}
		n, raErr := res.RowsAffected()
		if raErr != nil {
			return raErr
		}
		if n == 0 {
			_, err = dbConn.ExecContext(ctx, `
UPDATE media_files SET intro_start_sec = NULL, intro_end_sec = NULL, intro_probed_at = NULL, intro_locked = 0, updated_at = ?
WHERE id = (SELECT id FROM media_files WHERE media_id = ? ORDER BY is_primary DESC, COALESCE(missing_since, '') = '', id ASC LIMIT 1)`,
				now, mediaID,
			)
			if err != nil {
				return err
			}
		}
	} else if introStart != nil || introEnd != nil || introLocked != nil {
		row, err := lookupPrimaryMediaFile(dbConn, mediaID)
		if err != nil {
			return err
		}
		var s, e sql.NullFloat64
		if introStart != nil {
			s = sql.NullFloat64{Float64: *introStart, Valid: true}
		} else if row.IntroStartSec.Valid {
			s = row.IntroStartSec
		}
		if introEnd != nil {
			e = sql.NullFloat64{Float64: *introEnd, Valid: true}
		} else if row.IntroEndSec.Valid {
			e = row.IntroEndSec
		}
		if s.Valid && e.Valid && !(e.Float64 > s.Float64) {
			return fmt.Errorf("intro_end_seconds must be greater than intro_start_seconds")
		}
		locked := row.IntroLocked
		if introLocked != nil {
			if *introLocked {
				locked = 1
			} else {
				locked = 0
			}
		} else if introStart != nil || introEnd != nil {
			locked = 1
		}
		var startArg, endArg interface{}
		if s.Valid {
			startArg = s.Float64
		}
		if e.Valid {
			endArg = e.Float64
		}
		res, err := dbConn.ExecContext(ctx,
			`UPDATE media_files SET intro_start_sec = ?, intro_end_sec = ?, intro_locked = ?, intro_probed_at = ?, updated_at = ? WHERE media_id = ? AND is_primary = 1`,
			startArg, endArg, locked, now, now, mediaID,
		)
		if err != nil {
			return err
		}
		n, raErr := res.RowsAffected()
		if raErr != nil {
			return raErr
		}
		if n == 0 {
			_, err = dbConn.ExecContext(ctx, `
UPDATE media_files SET intro_start_sec = ?, intro_end_sec = ?, intro_locked = ?, intro_probed_at = ?, updated_at = ?
WHERE id = (SELECT id FROM media_files WHERE media_id = ? ORDER BY is_primary DESC, COALESCE(missing_since, '') = '', id ASC LIMIT 1)`,
				startArg, endArg, locked, now, now, mediaID,
			)
			if err != nil {
				return err
			}
		}
	}

	if clearCredits {
		res, err := dbConn.ExecContext(ctx,
			`UPDATE media_files SET credits_start_sec = NULL, credits_end_sec = NULL, updated_at = ? WHERE media_id = ? AND is_primary = 1`,
			now, mediaID,
		)
		if err != nil {
			return err
		}
		n, raErr := res.RowsAffected()
		if raErr != nil {
			return raErr
		}
		if n == 0 {
			_, err = dbConn.ExecContext(ctx, `
UPDATE media_files SET credits_start_sec = NULL, credits_end_sec = NULL, updated_at = ?
WHERE id = (SELECT id FROM media_files WHERE media_id = ? ORDER BY is_primary DESC, COALESCE(missing_since, '') = '', id ASC LIMIT 1)`,
				now, mediaID,
			)
			return err
		}
		return nil
	}
	if creditsStart != nil || creditsEnd != nil {
		row, err := lookupPrimaryMediaFile(dbConn, mediaID)
		if err != nil {
			return err
		}
		var cs, ce sql.NullFloat64
		if creditsStart != nil {
			cs = sql.NullFloat64{Float64: *creditsStart, Valid: true}
		} else if row.CreditsStartSec.Valid {
			cs = row.CreditsStartSec
		}
		if creditsEnd != nil {
			ce = sql.NullFloat64{Float64: *creditsEnd, Valid: true}
		} else if row.CreditsEndSec.Valid {
			ce = row.CreditsEndSec
		}
		if cs.Valid && ce.Valid && !(ce.Float64 > cs.Float64) {
			return fmt.Errorf("credits_end_seconds must be greater than credits_start_seconds")
		}
		var csa, cea interface{}
		if cs.Valid {
			csa = cs.Float64
		}
		if ce.Valid {
			cea = ce.Float64
		}
		res, err := dbConn.ExecContext(ctx,
			`UPDATE media_files SET credits_start_sec = ?, credits_end_sec = ?, updated_at = ? WHERE media_id = ? AND is_primary = 1`,
			csa, cea, now, mediaID,
		)
		if err != nil {
			return err
		}
		n, raErr := res.RowsAffected()
		if raErr != nil {
			return raErr
		}
		if n == 0 {
			_, err = dbConn.ExecContext(ctx, `
UPDATE media_files SET credits_start_sec = ?, credits_end_sec = ?, updated_at = ?
WHERE id = (SELECT id FROM media_files WHERE media_id = ? ORDER BY is_primary DESC, COALESCE(missing_since, '') = '', id ASC LIMIT 1)`,
				csa, cea, now, mediaID,
			)
			if err != nil {
				return err
			}
		}
		return nil
	}
	return nil
}

// UpdateMediaFileIntroFromChromaprint writes probe-derived intro bounds unless intro_locked is set.
func UpdateMediaFileIntroFromChromaprint(ctx context.Context, dbConn *sql.DB, mediaID int, startSec, endSec float64) error {
	if mediaID <= 0 || !(endSec > startSec) {
		return nil
	}
	locked, err := MediaFileIntroLocked(ctx, dbConn, mediaID)
	if err != nil || locked {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := dbConn.ExecContext(ctx,
		`UPDATE media_files SET intro_start_sec = ?, intro_end_sec = ?, intro_probed_at = ?, updated_at = ? WHERE media_id = ? AND is_primary = 1`,
		startSec, endSec, now, now, mediaID,
	)
	if err != nil {
		return err
	}
	n, raErr := res.RowsAffected()
	if raErr != nil {
		return raErr
	}
	if n > 0 {
		return nil
	}
	_, err = dbConn.ExecContext(ctx, `
UPDATE media_files SET intro_start_sec = ?, intro_end_sec = ?, intro_probed_at = ?, updated_at = ?
WHERE id = (SELECT id FROM media_files WHERE media_id = ? ORDER BY is_primary DESC, COALESCE(missing_since, '') = '', id ASC LIMIT 1)`,
		startSec, endSec, now, now, mediaID,
	)
	return err
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
intro_start_sec, intro_end_sec, COALESCE(intro_locked, 0), credits_start_sec, credits_end_sec
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
			&row.IntroLocked,
			&row.CreditsStartSec,
			&row.CreditsEndSec,
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
		items[idx].IntroLocked = row.IntroLocked != 0
		if row.CreditsStartSec.Valid {
			v := row.CreditsStartSec.Float64
			items[idx].CreditsStartSeconds = &v
		}
		if row.CreditsEndSec.Valid {
			v := row.CreditsEndSec.Float64
			items[idx].CreditsEndSeconds = &v
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
		        intro_start_sec, intro_end_sec, COALESCE(intro_locked, 0), credits_start_sec, credits_end_sec
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
		&row.IntroLocked,
		&row.CreditsStartSec,
		&row.CreditsEndSec,
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
