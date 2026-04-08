package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// UpdateMediaMetadata updates a single category row with identified metadata (title, overview, poster, tmdb_id, etc.).
func UpdateMediaMetadata(ctx context.Context, db *sql.DB, table string, refID int, title string, overview, posterPath, backdropPath, releaseDate string, voteAvg float64, imdbID string, imdbRating float64, tmdbID int, tvdbID string, season, episode int) error {
	return UpdateMediaMetadataWithState(ctx, db, table, refID, title, overview, posterPath, backdropPath, releaseDate, voteAvg, imdbID, imdbRating, tmdbID, tvdbID, season, episode, false, false)
}

// UpdateMediaMetadataWithReview updates a single category row with identified metadata and review state.
func UpdateMediaMetadataWithReview(ctx context.Context, db *sql.DB, table string, refID int, title string, overview, posterPath, backdropPath, releaseDate string, voteAvg float64, imdbID string, imdbRating float64, tmdbID int, tvdbID string, season, episode int, metadataReviewNeeded bool) error {
	return UpdateMediaMetadataWithState(ctx, db, table, refID, title, overview, posterPath, backdropPath, releaseDate, voteAvg, imdbID, imdbRating, tmdbID, tvdbID, season, episode, metadataReviewNeeded, false)
}

// UpdateMediaMetadataWithState updates a single category row with identified metadata and episodic metadata state.
func UpdateMediaMetadataWithState(ctx context.Context, db *sql.DB, table string, refID int, title string, overview, posterPath, backdropPath, releaseDate string, voteAvg float64, imdbID string, imdbRating float64, tmdbID int, tvdbID string, season, episode int, metadataReviewNeeded bool, metadataConfirmed bool) error {
	return UpdateMediaMetadataWithCanonicalState(ctx, db, table, refID, title, overview, posterPath, backdropPath, releaseDate, voteAvg, imdbID, imdbRating, tmdbID, tvdbID, season, episode, CanonicalMetadata{
		Title:        title,
		Overview:     overview,
		PosterPath:   posterPath,
		BackdropPath: backdropPath,
		ReleaseDate:  releaseDate,
		VoteAverage:  voteAvg,
		IMDbID:       imdbID,
		IMDbRating:   imdbRating,
	}, metadataReviewNeeded, metadataConfirmed, true)
}

// UpdateMediaMetadataWithCanonicalState updates a single category row with separate canonical show/season metadata.
// When updateShowVoteAverage is false, the shows.vote_average column is left unchanged (e.g. episode-only identify flows
// that do not have provider show-level scores).
// ctx is used for transactions and queries so shutdown or request cancellation can abort in-flight writes.
func UpdateMediaMetadataWithCanonicalState(ctx context.Context, db *sql.DB, table string, refID int, title string, overview, posterPath, backdropPath, releaseDate string, voteAvg float64, imdbID string, imdbRating float64, tmdbID int, tvdbID string, season, episode int, canonical CanonicalMetadata, metadataReviewNeeded bool, metadataConfirmed bool, updateShowVoteAverage bool) error {
	if strings.TrimSpace(canonical.Title) == "" {
		canonical.Title = title
	}
	if strings.TrimSpace(canonical.Overview) == "" {
		canonical.Overview = overview
	}
	if strings.TrimSpace(canonical.PosterPath) == "" {
		canonical.PosterPath = posterPath
	}
	if strings.TrimSpace(canonical.BackdropPath) == "" {
		canonical.BackdropPath = backdropPath
	}
	if strings.TrimSpace(canonical.ReleaseDate) == "" {
		canonical.ReleaseDate = releaseDate
	}
	if strings.TrimSpace(canonical.IMDbID) == "" {
		canonical.IMDbID = imdbID
	}
	contentHash := metadataHash(
		title,
		overview,
		posterPath,
		backdropPath,
		releaseDate,
		fmt.Sprintf("%.3f", voteAvg),
		imdbID,
		fmt.Sprintf("%.3f", imdbRating),
		strconv.Itoa(tmdbID),
		tvdbID,
		strconv.Itoa(season),
		strconv.Itoa(episode),
	)
	now := time.Now().UTC().Format(time.RFC3339)
	if table == "tv_episodes" || table == "anime_episodes" {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		var libraryID int
		if err := tx.QueryRowContext(ctx, `SELECT library_id FROM `+table+` WHERE id = ?`, refID).Scan(&libraryID); err != nil {
			_ = tx.Rollback()
			return err
		}
		showID, seasonID, err := upsertShowAndSeasonTx(ctx, tx, libraryID, table, tmdbID, tvdbID, canonical, season, updateShowVoteAverage)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE `+table+` SET
title = ?,
match_status = ?,
tmdb_id = ?,
tvdb_id = ?,
overview = ?,
poster_path = ?,
backdrop_path = ?,
release_date = ?,
vote_average = ?,
imdb_id = ?,
imdb_rating = ?,
season = ?,
episode = ?,
metadata_review_needed = ?,
metadata_confirmed = ?,
show_id = ?,
season_id = ?,
metadata_version = CASE WHEN COALESCE(metadata_content_hash, '') != ? THEN COALESCE(metadata_version, 1) + 1 ELSE COALESCE(metadata_version, 1) END,
metadata_content_hash = ?,
last_metadata_refresh_at = ?
WHERE id = ?`,
			title,
			MatchStatusIdentified,
			tmdbID,
			nullStr(tvdbID),
			nullStr(overview),
			nullStr(posterPath),
			nullStr(backdropPath),
			nullStr(releaseDate),
			nullFloat64(voteAvg),
			nullStr(imdbID),
			nullFloat64(imdbRating),
			season,
			episode,
			metadataReviewNeeded,
			metadataConfirmed,
			nullInt(showID),
			nullInt(seasonID),
			contentHash,
			contentHash,
			now,
			refID,
		)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := syncTitleMetadataTx(ctx, tx, "show", showID, canonical); err != nil {
			_ = tx.Rollback()
			return err
		}
		return tx.Commit()
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	var (
		existingPosterPath string
		posterLocked       int
	)
	if err := tx.QueryRowContext(
		ctx,
		`SELECT COALESCE(poster_path, ''), COALESCE(poster_locked, 0) FROM `+table+` WHERE id = ?`,
		refID,
	).Scan(&existingPosterPath, &posterLocked); err != nil {
		_ = tx.Rollback()
		return err
	}
	if posterLocked != 0 {
		posterPath = existingPosterPath
	}
	if _, err := tx.ExecContext(ctx, `UPDATE `+table+` SET title = ?, match_status = ?, tmdb_id = ?, tvdb_id = ?, overview = ?, poster_path = ?, backdrop_path = ?, release_date = ?, vote_average = ?, imdb_id = ?, imdb_rating = ? WHERE id = ?`,
		title, MatchStatusIdentified, tmdbID, nullStr(tvdbID), nullStr(overview), nullStr(posterPath), nullStr(backdropPath), nullStr(releaseDate), nullFloat64(voteAvg), nullStr(imdbID), nullFloat64(imdbRating), refID); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := syncTitleMetadataTx(ctx, tx, "movie", refID, canonical); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// UpdateShowMetadataState sets episodic metadata flags for a batch of rows.
func UpdateShowMetadataState(db *sql.DB, table string, refIDs []int, metadataReviewNeeded bool, metadataConfirmed bool) (int, error) {
	if (table != "tv_episodes" && table != "anime_episodes") || len(refIDs) == 0 {
		return 0, nil
	}
	placeholders := make([]string, len(refIDs))
	args := make([]interface{}, 0, len(refIDs)+2)
	args = append(args, metadataReviewNeeded, metadataConfirmed)
	for i, refID := range refIDs {
		placeholders[i] = "?"
		args = append(args, refID)
	}
	result, err := db.Exec(`UPDATE `+table+` SET metadata_review_needed = ?, metadata_confirmed = ? WHERE id IN (`+strings.Join(placeholders, ",")+`)`, args...)
	if err != nil {
		return 0, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(rowsAffected), nil
}

// ShowEpisodeRef identifies one episode row for refresh/identify (global id, category ref_id, kind, season, episode, tmdb_id).
type ShowEpisodeRef struct {
	GlobalID int
	RefID    int
	Kind     string
	Season   int
	Episode  int
	TMDBID   int
}

func normalizeShowKeyTitle(title string) string {
	title = showNameFromTitle(title)
	title = strings.ToLower(title)
	title = showKeyNonAlnumRegexp.ReplaceAllString(title, "")
	return title
}

func showNameFromTitle(title string) string {
	if match := showNameFromTitleRegexp.FindStringSubmatch(title); len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	if i := strings.Index(title, " - "); i > 0 {
		return strings.TrimSpace(title[:i])
	}
	return strings.TrimSpace(title)
}

// showKeyFromItem returns the same key the frontend uses: "tmdb-{id}" when tmdb_id set, else "title-{normalizedTitle}".
func showKeyFromItem(tmdbID int, title string) string {
	if tmdbID > 0 {
		return fmt.Sprintf("tmdb-%d", tmdbID)
	}
	return "title-" + normalizeShowKeyTitle(title)
}

// ListShowEpisodeRefs returns all episode refs (globalID, refID, kind, season, episode) for the given library and showKey.
// Only TV and anime libraries are supported; returns nil when library type is not tv/anime.
func ListShowEpisodeRefs(db *sql.DB, libraryID int, showKey string) ([]ShowEpisodeRef, error) {
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
	q := `SELECT g.id, m.id, COALESCE(m.season, 0), COALESCE(m.episode, 0), COALESCE(m.tmdb_id, 0), m.title
FROM ` + table + ` m
JOIN media_global g ON g.kind = ? AND g.ref_id = m.id
WHERE m.library_id = ?
ORDER BY g.id`
	rows, err := db.Query(q, typ, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ShowEpisodeRef
	for rows.Next() {
		var globalID, refID, season, episode, tmdbID int
		var title string
		if err := rows.Scan(&globalID, &refID, &season, &episode, &tmdbID, &title); err != nil {
			return nil, err
		}
		key := showKeyFromItem(tmdbID, title)
		if key != showKey {
			continue
		}
		out = append(out, ShowEpisodeRef{GlobalID: globalID, RefID: refID, Kind: typ, Season: season, Episode: episode, TMDBID: tmdbID})
	}
	return out, rows.Err()
}
