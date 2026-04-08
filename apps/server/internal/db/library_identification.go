package db

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

// IdentificationRow is a library media row eligible for metadata identification or repair.
type IdentificationRow struct {
	RefID       int
	Kind        string
	Title       string
	Path        string
	Season      int
	Episode     int
	MatchStatus string
	TMDBID      int
	TVDBID      string
}

type EpisodeIdentifyRow struct {
	IdentificationRow
	TMDBID int
	TVDBID string
}

type EpisodeIdentifyGroup struct {
	Key  string
	Kind string
	Rows []EpisodeIdentifyRow
}

// ListIdentifiableByLibrary returns non-music media rows that still need identification
// or metadata repair (for example, missing TMDB IDs or poster art).
//
// Movies: missing imdb_id alone does not keep a TMDB-matched row in the queue. Otherwise
// libraries accrue endless "refresh" work that starves new files and duplicates TMDB calls.
func ListIdentifiableByLibrary(db *sql.DB, libraryID int) ([]IdentificationRow, error) {
	var typ string
	if err := db.QueryRow(`SELECT type FROM libraries WHERE id = ?`, libraryID).Scan(&typ); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	table := mediaTableForKind(typ)
	if table == "music_tracks" {
		return nil, nil
	}
	policy := GetMetadataRefreshPolicy(db)
	refreshBefore := time.Now().UTC().Add(-time.Duration(policy.ScanRefreshMinAgeHours) * time.Hour).Format(time.RFC3339)
	var q string
	var args []interface{}
	if table == "tv_episodes" || table == "anime_episodes" {
		q = `SELECT m.id, m.title, m.path, COALESCE(m.season, 0), COALESCE(m.episode, 0), COALESCE(m.match_status, ''), COALESCE(m.tmdb_id, 0), COALESCE(m.tvdb_id, '') FROM ` + table + ` m
WHERE m.library_id = ?
  AND COALESCE(m.missing_since, '') = ''
  AND COALESCE(m.metadata_confirmed, 0) = 0
  AND (
    COALESCE(m.match_status, '') != ? OR
    COALESCE(m.tmdb_id, 0) = 0 OR
    COALESCE(m.poster_path, '') = '' OR
    COALESCE(m.imdb_id, '') = '' OR
    COALESCE(m.last_metadata_refresh_at, '') = '' OR
    COALESCE(m.last_metadata_refresh_at, '') <= ?
  )`
		args = []interface{}{libraryID, MatchStatusIdentified, refreshBefore}
	} else {
		q = `SELECT m.id, m.title, m.path, COALESCE(m.match_status, ''), COALESCE(m.tmdb_id, 0), COALESCE(m.tvdb_id, '') FROM ` + table + ` m
WHERE m.library_id = ?
  AND COALESCE(m.missing_since, '') = ''
  AND (
    COALESCE(m.match_status, '') != ? OR
    COALESCE(m.tmdb_id, 0) = 0 OR
    COALESCE(m.poster_path, '') = ''
  )`
		args = []interface{}{libraryID, MatchStatusIdentified}
	}
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IdentificationRow
	for rows.Next() {
		var row IdentificationRow
		row.Kind = typ
		if table == "tv_episodes" || table == "anime_episodes" {
			err = rows.Scan(
				&row.RefID,
				&row.Title,
				&row.Path,
				&row.Season,
				&row.Episode,
				&row.MatchStatus,
				&row.TMDBID,
				&row.TVDBID,
			)
		} else {
			err = rows.Scan(&row.RefID, &row.Title, &row.Path, &row.MatchStatus, &row.TMDBID, &row.TVDBID)
		}
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// UnidentifiedLibrarySummary is a non-music library that has at least one row still needing
// provider identification (aligns with HTTP identify "tracked" rows, not refresh-only repairs).
type UnidentifiedLibrarySummary struct {
	LibraryID int    `json:"library_id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Count     int    `json:"count"`
}

func identificationRowNeedsProviderAttention(matchStatus string, tmdbID int, tvdbID string) bool {
	if matchStatus != MatchStatusIdentified {
		return true
	}
	return !(tmdbID > 0 || strings.TrimSpace(tvdbID) != "")
}

// CountTrackedUnidentifiedByLibrary counts rows that still need a provider match or are not
// marked identified. Music libraries always return 0.
func CountTrackedUnidentifiedByLibrary(db *sql.DB, libraryID int) (int, error) {
	var typ string
	if err := db.QueryRow(`SELECT type FROM libraries WHERE id = ?`, libraryID).Scan(&typ); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	table := mediaTableForKind(typ)
	if table == "music_tracks" {
		return 0, nil
	}
	if table == "tv_episodes" || table == "anime_episodes" {
		rows, err := ListEpisodeIdentifyRowsByLibrary(db, libraryID)
		if err != nil {
			return 0, err
		}
		n := 0
		for _, row := range rows {
			if identificationRowNeedsProviderAttention(row.MatchStatus, row.TMDBID, row.TVDBID) {
				n++
			}
		}
		return n, nil
	}
	rows, err := ListIdentifiableByLibrary(db, libraryID)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, row := range rows {
		if identificationRowNeedsProviderAttention(row.MatchStatus, row.TMDBID, row.TVDBID) {
			n++
		}
	}
	return n, nil
}

// ListUnidentifiedLibrarySummariesForUser returns non-music libraries with count > 0.
func ListUnidentifiedLibrarySummariesForUser(db *sql.DB, userID int) ([]UnidentifiedLibrarySummary, error) {
	rows, err := db.Query(
		`SELECT id, name, type FROM libraries WHERE user_id = ? AND type != ? ORDER BY name COLLATE NOCASE, id`,
		userID, LibraryTypeMusic,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []UnidentifiedLibrarySummary
	for rows.Next() {
		var id int
		var name, typ string
		if err := rows.Scan(&id, &name, &typ); err != nil {
			return nil, err
		}
		n, err := CountTrackedUnidentifiedByLibrary(db, id)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			continue
		}
		out = append(out, UnidentifiedLibrarySummary{
			LibraryID: id,
			Name:      name,
			Type:      typ,
			Count:     n,
		})
	}
	return out, rows.Err()
}

func ListEpisodeIdentifyRowsByLibrary(db *sql.DB, libraryID int) ([]EpisodeIdentifyRow, error) {
	var typ string
	if err := db.QueryRow(`SELECT type FROM libraries WHERE id = ?`, libraryID).Scan(&typ); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	table := mediaTableForKind(typ)
	if table != "tv_episodes" && table != "anime_episodes" {
		return nil, nil
	}
	policy := GetMetadataRefreshPolicy(db)
	refreshBefore := time.Now().UTC().Add(-time.Duration(policy.ScanRefreshMinAgeHours) * time.Hour).Format(time.RFC3339)
	rows, err := db.Query(`SELECT m.id, m.title, m.path, COALESCE(m.season, 0), COALESCE(m.episode, 0), COALESCE(m.match_status, ''), COALESCE(m.tmdb_id, 0), COALESCE(m.tvdb_id, '')
FROM `+table+` m
WHERE m.library_id = ?
  AND COALESCE(m.missing_since, '') = ''
  AND COALESCE(m.metadata_confirmed, 0) = 0
  AND (
    COALESCE(m.match_status, '') != ? OR
    COALESCE(m.tmdb_id, 0) = 0 OR
    COALESCE(m.poster_path, '') = '' OR
    COALESCE(m.imdb_id, '') = '' OR
    COALESCE(m.last_metadata_refresh_at, '') = '' OR
    COALESCE(m.last_metadata_refresh_at, '') <= ?
  )`, libraryID, MatchStatusIdentified, refreshBefore)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []EpisodeIdentifyRow
	for rows.Next() {
		var row EpisodeIdentifyRow
		row.Kind = typ
		if err := rows.Scan(
			&row.RefID,
			&row.Title,
			&row.Path,
			&row.Season,
			&row.Episode,
			&row.MatchStatus,
			&row.TMDBID,
			&row.TVDBID,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
