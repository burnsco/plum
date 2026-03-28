package db

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

type MovieArtworkTarget struct {
	GlobalID     int
	RefID        int
	LibraryID    int
	Title        string
	TMDBID       int
	IMDbID       string
	PosterPath   string
	PosterLocked bool
}

type ShowArtworkTarget struct {
	ID           int
	LibraryID    int
	Kind         string
	Title        string
	ShowKey      string
	TMDBID       int
	TVDBID       string
	IMDbID       string
	PosterPath   string
	PosterLocked bool
}

func GetMovieArtworkTarget(dbConn *sql.DB, libraryID int, mediaID int) (*MovieArtworkTarget, error) {
	var target MovieArtworkTarget
	var posterLocked int
	err := dbConn.QueryRow(
		`SELECT mg.id, movies.id, movies.library_id, movies.title, COALESCE(movies.tmdb_id, 0), COALESCE(movies.imdb_id, ''),
		        COALESCE(movies.poster_path, ''), COALESCE(movies.poster_locked, 0)
		   FROM media_global mg
		   JOIN movies ON mg.kind = 'movie' AND mg.ref_id = movies.id
		  WHERE movies.library_id = ? AND mg.id = ?
		  LIMIT 1`,
		libraryID,
		mediaID,
	).Scan(
		&target.GlobalID,
		&target.RefID,
		&target.LibraryID,
		&target.Title,
		&target.TMDBID,
		&target.IMDbID,
		&target.PosterPath,
		&posterLocked,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	target.PosterLocked = posterLocked != 0
	return &target, nil
}

func SetMoviePosterSelection(dbConn *sql.DB, refID int, sourceURL string, locked bool) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := dbConn.Exec(
		`UPDATE movies SET poster_path = ?, poster_locked = ?, updated_at = ? WHERE id = ?`,
		nullStr(strings.TrimSpace(sourceURL)),
		locked,
		now,
		refID,
	)
	return err
}

func GetShowArtworkTarget(dbConn *sql.DB, libraryID int, showKey string) (*ShowArtworkTarget, error) {
	var libraryType string
	if err := dbConn.QueryRow(`SELECT type FROM libraries WHERE id = ?`, libraryID).Scan(&libraryType); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if libraryType != LibraryTypeTV && libraryType != LibraryTypeAnime {
		return nil, nil
	}

	query := `SELECT id, library_id, kind, title, COALESCE(tmdb_id, 0), COALESCE(tvdb_id, ''), COALESCE(imdb_id, ''),
	                 COALESCE(poster_path, ''), COALESCE(poster_locked, 0)
	            FROM shows
	           WHERE library_id = ? AND kind = ?`
	args := []any{libraryID, libraryType}
	if strings.HasPrefix(showKey, "tmdb-") {
		tmdbID := strings.TrimSpace(strings.TrimPrefix(showKey, "tmdb-"))
		if tmdbID == "" {
			return nil, nil
		}
		query += ` AND tmdb_id = ? LIMIT 1`
		args = append(args, tmdbID)
	} else {
		titleKey := strings.TrimSpace(strings.TrimPrefix(showKey, "title-"))
		if titleKey == "" {
			return nil, nil
		}
		query += ` AND title_key = ? LIMIT 1`
		args = append(args, titleKey)
	}

	var target ShowArtworkTarget
	var posterLocked int
	err := dbConn.QueryRow(query, args...).Scan(
		&target.ID,
		&target.LibraryID,
		&target.Kind,
		&target.Title,
		&target.TMDBID,
		&target.TVDBID,
		&target.IMDbID,
		&target.PosterPath,
		&posterLocked,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	target.ShowKey = showKeyFromItem(target.TMDBID, target.Title)
	target.PosterLocked = posterLocked != 0
	return &target, nil
}

func SetShowPosterSelection(dbConn *sql.DB, showID int, sourceURL string, locked bool) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := dbConn.Exec(
		`UPDATE shows SET poster_path = ?, poster_locked = ?, updated_at = ? WHERE id = ?`,
		nullStr(strings.TrimSpace(sourceURL)),
		locked,
		now,
		showID,
	)
	return err
}
