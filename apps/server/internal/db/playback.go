package db

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	completedProgressPercent = 95.0
	completedRemainingSecs   = 120.0
	recentlyAddedLimit       = 24

	// Home dashboard "recently added" merges newest rows across movie/tv/anime; per-kind caps bound DB work.
	dashboardRecentPerKindCap = 250
	dashboardRecentMergeCap   = 500
)

type ContinueWatchingEntry struct {
	Kind             string    `json:"kind"`
	Media            MediaItem `json:"media"`
	ShowKey          string    `json:"show_key,omitempty"`
	ShowTitle        string    `json:"show_title,omitempty"`
	EpisodeLabel     string    `json:"episode_label,omitempty"`
	RemainingSeconds float64   `json:"remaining_seconds"`
	activityAt       string
}

type RecentlyAddedEntry struct {
	Kind         string    `json:"kind"`
	Media        MediaItem `json:"media"`
	ShowKey      string    `json:"show_key,omitempty"`
	ShowTitle    string    `json:"show_title,omitempty"`
	EpisodeLabel string    `json:"episode_label,omitempty"`
}

// HomeDashboard recently-added rails (below continue watching), fixed order on clients:
// TV episodes → TV shows → movies → anime episodes → anime shows.
type HomeDashboard struct {
	ContinueWatching           []ContinueWatchingEntry `json:"continueWatching"`
	RecentlyAddedTvEpisodes    []RecentlyAddedEntry    `json:"recentlyAddedTvEpisodes"`
	RecentlyAddedTvShows       []RecentlyAddedEntry    `json:"recentlyAddedTvShows"`
	RecentlyAddedMovies        []RecentlyAddedEntry    `json:"recentlyAddedMovies"`
	RecentlyAddedAnimeEpisodes []RecentlyAddedEntry    `json:"recentlyAddedAnimeEpisodes"`
	RecentlyAddedAnimeShows    []RecentlyAddedEntry    `json:"recentlyAddedAnimeShows"`
}

type playbackProgressRow struct {
	PositionSeconds float64
	DurationSeconds float64
	ProgressPercent float64
	Completed       bool
	LastWatchedAt   string
}

func GetMediaByLibraryIDForUser(db *sql.DB, libraryID int, userID int) ([]MediaItem, error) {
	items, err := GetMediaByLibraryID(db, libraryID)
	if err != nil {
		return nil, err
	}
	return attachPlaybackProgressBatch(db, userID, items)
}

func GetMediaPageByLibraryIDForUser(db *sql.DB, libraryID int, userID int, offset int, limit int) (LibraryMediaPage, error) {
	page, err := GetMediaPageByLibraryID(db, libraryID, offset, limit)
	if err != nil {
		return LibraryMediaPage{}, err
	}
	items, err := attachPlaybackProgressBatch(db, userID, page.Items)
	if err != nil {
		return LibraryMediaPage{}, err
	}
	page.Items = items
	return page, nil
}

func GetHomeDashboardForUser(db *sql.DB, userID int) (HomeDashboard, error) {
	movieItems, err := loadDashboardMoviesInProgress(db, userID)
	if err != nil {
		return HomeDashboard{}, err
	}
	movieItems, err = attachMediaFilesBatch(db, movieItems)
	if err != nil {
		return HomeDashboard{}, err
	}
	movieItems, err = attachDuplicateState(db, movieItems)
	if err != nil {
		return HomeDashboard{}, err
	}
	movieItems, err = attachPlaybackProgressBatch(db, userID, movieItems)
	if err != nil {
		return HomeDashboard{}, err
	}

	showEntries, err := loadDashboardContinueWatchingShows(db, userID)
	if err != nil {
		return HomeDashboard{}, err
	}

	continueWatching := make([]ContinueWatchingEntry, 0, len(movieItems)+len(showEntries))
	for _, item := range movieItems {
		continueWatching = append(continueWatching, ContinueWatchingEntry{
			Kind:             "movie",
			Media:            item,
			RemainingSeconds: item.RemainingSeconds,
			activityAt:       item.LastWatchedAt,
		})
	}
	continueWatching = append(continueWatching, showEntries...)
	sort.Slice(continueWatching, func(i, j int) bool {
		return continueWatching[i].activityAt > continueWatching[j].activityAt
	})

	recentCandidates, err := loadRecentDashboardMediaCandidates(db, userID)
	if err != nil {
		return HomeDashboard{}, err
	}
	recentCandidates, err = attachMediaFilesBatch(db, recentCandidates)
	if err != nil {
		return HomeDashboard{}, err
	}
	recentCandidates, err = attachDuplicateState(db, recentCandidates)
	if err != nil {
		return HomeDashboard{}, err
	}
	recentCandidates, err = attachPlaybackProgressBatch(db, userID, recentCandidates)
	if err != nil {
		return HomeDashboard{}, err
	}
	tvItems, animeItems, movieItems := partitionRecentDashboardCandidatesByType(recentCandidates)
	dash := HomeDashboard{
		ContinueWatching:           continueWatching,
		RecentlyAddedTvEpisodes:    buildRecentlyAddedEpisodeEntries(tvItems, recentlyAddedLimit),
		RecentlyAddedTvShows:       buildRecentlyAddedShowsForKind(tvItems, LibraryTypeTV),
		RecentlyAddedMovies:        buildRecentlyAddedMovieEntries(movieItems),
		RecentlyAddedAnimeEpisodes: buildRecentlyAddedEpisodeEntries(animeItems, recentlyAddedLimit),
		RecentlyAddedAnimeShows:    buildRecentlyAddedShowsForKind(animeItems, LibraryTypeAnime),
	}
	if err := attachHomeDashboardSubtitles(db, &dash); err != nil {
		return HomeDashboard{}, err
	}
	enrichHomeDashboardShowTitles(db, &dash)
	redactEpisodeRatingsInHomeDashboard(&dash)
	return dash, nil
}

func redactEpisodeRatingsForDashboardMedia(m *MediaItem) {
	if m.Type == LibraryTypeTV || m.Type == LibraryTypeAnime {
		m.VoteAverage = 0
		m.IMDbRating = 0
		m.IMDbID = ""
	}
}

func redactEpisodeRatingsInHomeDashboard(dash *HomeDashboard) {
	for i := range dash.ContinueWatching {
		redactEpisodeRatingsForDashboardMedia(&dash.ContinueWatching[i].Media)
	}
	for i := range dash.RecentlyAddedTvEpisodes {
		redactEpisodeRatingsForDashboardMedia(&dash.RecentlyAddedTvEpisodes[i].Media)
	}
	for i := range dash.RecentlyAddedTvShows {
		redactEpisodeRatingsForDashboardMedia(&dash.RecentlyAddedTvShows[i].Media)
	}
	for i := range dash.RecentlyAddedAnimeEpisodes {
		redactEpisodeRatingsForDashboardMedia(&dash.RecentlyAddedAnimeEpisodes[i].Media)
	}
	for i := range dash.RecentlyAddedAnimeShows {
		redactEpisodeRatingsForDashboardMedia(&dash.RecentlyAddedAnimeShows[i].Media)
	}
}

// loadDashboardMoviesInProgress returns movie rows the user is partially watching (same rules as buildContinueWatching).
func loadDashboardMoviesInProgress(db *sql.DB, userID int) ([]MediaItem, error) {
	if userID <= 0 {
		return nil, nil
	}
	const q = `SELECT g.id, m.library_id, m.title, m.path, m.duration, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.missing_since, ''), m.match_status, m.tmdb_id, m.tvdb_id, m.overview, m.poster_path, m.backdrop_path, m.release_date, m.vote_average, m.imdb_id, m.imdb_rating
FROM playback_progress pp
JOIN media_global g ON g.id = pp.media_id AND g.kind = 'movie'
JOIN movies m ON g.ref_id = m.id
JOIN libraries l ON l.id = m.library_id AND l.user_id = ?
WHERE pp.user_id = ? AND pp.completed = 0 AND pp.progress_percent > 0`
	rows, err := db.Query(q, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMovieMediaItems(rows)
}

func scanMovieMediaItems(rows *sql.Rows) ([]MediaItem, error) {
	items := make([]MediaItem, 0)
	for rows.Next() {
		var m MediaItem
		m.Type = LibraryTypeMovie
		var overview, posterPath, backdropPath, releaseDate, matchStatus, imdbID sql.NullString
		var voteAvg, imdbRating sql.NullFloat64
		var tmdbID sql.NullInt64
		var tvdbID sql.NullString
		err := rows.Scan(&m.ID, &m.LibraryID, &m.Title, &m.Path, &m.Duration, &m.FileSizeBytes, &m.FileModTime, &m.FileHash, &m.FileHashKind, &m.MissingSince, &matchStatus, &tmdbID, &tvdbID, &overview, &posterPath, &backdropPath, &releaseDate, &voteAvg, &imdbID, &imdbRating)
		if err != nil {
			return nil, err
		}
		m.TMDBID = int(tmdbID.Int64)
		if tvdbID.Valid {
			m.TVDBID = tvdbID.String
		}
		if overview.Valid {
			m.Overview = overview.String
		}
		if posterPath.Valid {
			m.PosterPath = posterPath.String
		}
		if backdropPath.Valid {
			m.BackdropPath = backdropPath.String
		}
		if releaseDate.Valid {
			m.ReleaseDate = releaseDate.String
		}
		if voteAvg.Valid {
			m.VoteAverage = voteAvg.Float64
		}
		if imdbID.Valid {
			m.IMDbID = imdbID.String
		}
		if imdbRating.Valid {
			m.IMDbRating = imdbRating.Float64
		}
		if matchStatus.Valid {
			m.MatchStatus = matchStatus.String
		}
		m.Missing = m.MissingSince != ""
		items = append(items, m)
	}
	return items, rows.Err()
}

type dashboardShowKey struct {
	libraryID int
	kind      string
	showKey   string
}

func loadDashboardContinueWatchingShows(db *sql.DB, userID int) ([]ContinueWatchingEntry, error) {
	if userID <= 0 {
		return nil, nil
	}
	seen := make(map[dashboardShowKey]struct{})
	var keys []dashboardShowKey

	const tvQ = `SELECT m.library_id, COALESCE(m.tmdb_id, 0), m.title
FROM playback_progress pp
JOIN media_global g ON g.id = pp.media_id AND g.kind = 'tv'
JOIN tv_episodes m ON g.ref_id = m.id
JOIN libraries l ON l.id = m.library_id AND l.user_id = ?
WHERE pp.user_id = ?`
	if err := collectDistinctShowKeys(db, tvQ, userID, userID, LibraryTypeTV, seen, &keys); err != nil {
		return nil, err
	}
	const animeQ = `SELECT m.library_id, COALESCE(m.tmdb_id, 0), m.title
FROM playback_progress pp
JOIN media_global g ON g.id = pp.media_id AND g.kind = 'anime'
JOIN anime_episodes m ON g.ref_id = m.id
JOIN libraries l ON l.id = m.library_id AND l.user_id = ?
WHERE pp.user_id = ?`
	if err := collectDistinctShowKeys(db, animeQ, userID, userID, LibraryTypeAnime, seen, &keys); err != nil {
		return nil, err
	}

	entries := make([]ContinueWatchingEntry, 0, len(keys))
	for _, sk := range keys {
		episodes, err := loadDashboardEpisodesForShow(db, sk.libraryID, sk.kind, sk.showKey)
		if err != nil {
			return nil, err
		}
		if len(episodes) == 0 {
			continue
		}
		episodes, err = attachMediaFilesBatch(db, episodes)
		if err != nil {
			return nil, err
		}
		episodes, err = attachDuplicateState(db, episodes)
		if err != nil {
			return nil, err
		}
		episodes, err = attachPlaybackProgressBatch(db, userID, episodes)
		if err != nil {
			return nil, err
		}
		entry, ok := continueWatchingEntryForShow(sk.showKey, episodes)
		if !ok {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func collectDistinctShowKeys(db *sql.DB, q string, libUserArg, ppUserArg int, kind string, seen map[dashboardShowKey]struct{}, keys *[]dashboardShowKey) error {
	rows, err := db.Query(q, libUserArg, ppUserArg)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var libraryID, tmdbID int
		var title string
		if err := rows.Scan(&libraryID, &tmdbID, &title); err != nil {
			return err
		}
		dk := dashboardShowKey{libraryID: libraryID, kind: kind, showKey: showKeyFromItem(tmdbID, title)}
		if _, ok := seen[dk]; ok {
			continue
		}
		seen[dk] = struct{}{}
		*keys = append(*keys, dk)
	}
	return rows.Err()
}

// loadDashboardEpisodesForShow loads all episodes belonging to the same UI show key within a library.
func loadDashboardEpisodesForShow(db *sql.DB, libraryID int, kind, showKey string) ([]MediaItem, error) {
	if err := ensureLibraryShowsAndSeasons(db, libraryID, kind); err != nil {
		return nil, err
	}
	showID, _, _, _, _, _, _, _, _, _, _, err := getShowCanonicalMetadata(db, libraryID, kind, showKey)
	if err != nil {
		return nil, err
	}
	if showID > 0 {
		return queryMediaByShowID(db, libraryID, kind, showID)
	}
	refs, err := ListShowEpisodeRefs(db, libraryID, showKey)
	if err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return nil, nil
	}
	ids := make([]int, len(refs))
	for i := range refs {
		ids[i] = refs[i].GlobalID
	}
	return batchLoadEpisodeMediaItems(db, libraryID, kind, ids)
}

func batchLoadEpisodeMediaItems(db *sql.DB, libraryID int, kind string, globalIDs []int) ([]MediaItem, error) {
	if len(globalIDs) == 0 {
		return nil, nil
	}
	table := mediaTableForKind(kind)
	if table != "tv_episodes" && table != "anime_episodes" {
		return nil, nil
	}
	placeholders := make([]string, len(globalIDs))
	args := make([]interface{}, 0, len(globalIDs)+3)
	args = append(args, kind, libraryID)
	for i, id := range globalIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	q := `SELECT g.id, m.library_id, m.title, m.path, m.duration, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.missing_since, ''), m.match_status, m.tmdb_id, m.tvdb_id, m.overview, m.poster_path, m.backdrop_path, m.release_date, m.vote_average, m.imdb_id, m.imdb_rating, COALESCE(m.season, 0), COALESCE(m.episode, 0), COALESCE(m.metadata_review_needed, 0), COALESCE(m.metadata_confirmed, 0), m.thumbnail_path, COALESCE(s.poster_path, ''), COALESCE(s.vote_average, 0), COALESCE(s.imdb_rating, 0)
FROM ` + table + ` m
JOIN media_global g ON g.kind = ? AND g.ref_id = m.id
LEFT JOIN shows s ON s.id = m.show_id
WHERE m.library_id = ? AND g.id IN (` + strings.Join(placeholders, ",") + `)
ORDER BY COALESCE(m.season, 0), COALESCE(m.episode, 0), COALESCE(m.title, ''), g.id`
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items, err := scanEpisodeMediaItems(rows, kind)
	if err != nil {
		return nil, err
	}
	if err := hydrateEpisodeShowPosters(db, libraryID, kind, items); err != nil {
		return nil, err
	}
	return items, nil
}

func scanEpisodeMediaItems(rows *sql.Rows, kind string) ([]MediaItem, error) {
	items := make([]MediaItem, 0)
	for rows.Next() {
		var m MediaItem
		m.Type = kind
		var overview, posterPath, backdropPath, releaseDate, thumbnailPath, matchStatus, imdbID sql.NullString
		var showPosterPath sql.NullString
		var voteAvg, showVoteAvg, showImdbAvg, imdbRating sql.NullFloat64
		var tmdbID sql.NullInt64
		var tvdbID sql.NullString
		var metadataReviewNeeded sql.NullBool
		var metadataConfirmed sql.NullBool
		err := rows.Scan(&m.ID, &m.LibraryID, &m.Title, &m.Path, &m.Duration, &m.FileSizeBytes, &m.FileModTime, &m.FileHash, &m.FileHashKind, &m.MissingSince, &matchStatus, &tmdbID, &tvdbID, &overview, &posterPath, &backdropPath, &releaseDate, &voteAvg, &imdbID, &imdbRating, &m.Season, &m.Episode, &metadataReviewNeeded, &metadataConfirmed, &thumbnailPath, &showPosterPath, &showVoteAvg, &showImdbAvg)
		if err != nil {
			return nil, err
		}
		m.TMDBID = int(tmdbID.Int64)
		if tvdbID.Valid {
			m.TVDBID = tvdbID.String
		}
		if overview.Valid {
			m.Overview = overview.String
		}
		if posterPath.Valid {
			m.PosterPath = posterPath.String
		}
		if backdropPath.Valid {
			m.BackdropPath = backdropPath.String
		}
		if releaseDate.Valid {
			m.ReleaseDate = releaseDate.String
		}
		if voteAvg.Valid {
			m.VoteAverage = voteAvg.Float64
		}
		if imdbID.Valid {
			m.IMDbID = imdbID.String
		}
		if imdbRating.Valid {
			m.IMDbRating = imdbRating.Float64
		}
		if metadataReviewNeeded.Valid {
			m.MetadataReviewNeeded = metadataReviewNeeded.Bool
		}
		if metadataConfirmed.Valid {
			m.MetadataConfirmed = metadataConfirmed.Bool
		}
		if thumbnailPath.Valid {
			m.ThumbnailPath = thumbnailPath.String
		}
		if showPosterPath.Valid {
			m.ShowPosterPath = showPosterPath.String
		}
		if showVoteAvg.Valid {
			m.ShowVoteAverage = showVoteAvg.Float64
		}
		if showImdbAvg.Valid {
			m.ShowIMDbRating = showImdbAvg.Float64
		}
		if matchStatus.Valid {
			m.MatchStatus = matchStatus.String
		}
		m.Missing = m.MissingSince != ""
		items = append(items, m)
	}
	return items, rows.Err()
}

type globalKindRow struct {
	id   int
	kind string
}

func loadRecentDashboardMediaCandidates(db *sql.DB, userID int) ([]MediaItem, error) {
	if userID <= 0 {
		return nil, nil
	}
	limit := strconv.Itoa(dashboardRecentPerKindCap)
	var rows []globalKindRow

	add := func(rowsOut *[]globalKindRow, q string, args ...interface{}) error {
		r, err := db.Query(q, args...)
		if err != nil {
			return err
		}
		defer r.Close()
		for r.Next() {
			var id int
			var kind string
			if err := r.Scan(&id, &kind); err != nil {
				return err
			}
			*rowsOut = append(*rowsOut, globalKindRow{id: id, kind: kind})
		}
		return r.Err()
	}
	movieQ := `SELECT g.id, 'movie' FROM movies m
JOIN media_global g ON g.kind = 'movie' AND g.ref_id = m.id
JOIN libraries l ON l.id = m.library_id AND l.user_id = ?
WHERE COALESCE(m.missing_since, '') = ''
ORDER BY g.id DESC LIMIT ` + limit
	if err := add(&rows, movieQ, userID); err != nil {
		return nil, err
	}
	tvQ := `SELECT g.id, 'tv' FROM tv_episodes m
JOIN media_global g ON g.kind = 'tv' AND g.ref_id = m.id
JOIN libraries l ON l.id = m.library_id AND l.user_id = ?
WHERE COALESCE(m.missing_since, '') = ''
ORDER BY g.id DESC LIMIT ` + limit
	if err := add(&rows, tvQ, userID); err != nil {
		return nil, err
	}
	animeQ := `SELECT g.id, 'anime' FROM anime_episodes m
JOIN media_global g ON g.kind = 'anime' AND g.ref_id = m.id
JOIN libraries l ON l.id = m.library_id AND l.user_id = ?
WHERE COALESCE(m.missing_since, '') = ''
ORDER BY g.id DESC LIMIT ` + limit
	if err := add(&rows, animeQ, userID); err != nil {
		return nil, err
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].id > rows[j].id })
	if len(rows) > dashboardRecentMergeCap {
		rows = rows[:dashboardRecentMergeCap]
	}
	if len(rows) == 0 {
		return nil, nil
	}

	movieIDs := make([]int, 0)
	tvIDs := make([]int, 0)
	animeIDs := make([]int, 0)
	for _, r := range rows {
		switch r.kind {
		case LibraryTypeMovie:
			movieIDs = append(movieIDs, r.id)
		case LibraryTypeTV:
			tvIDs = append(tvIDs, r.id)
		case LibraryTypeAnime:
			animeIDs = append(animeIDs, r.id)
		}
	}

	out := make([]MediaItem, 0, len(rows))
	if len(movieIDs) > 0 {
		it, err := batchLoadMoviesByGlobalIDsForUser(db, userID, movieIDs)
		if err != nil {
			return nil, err
		}
		out = append(out, it...)
	}
	if len(tvIDs) > 0 {
		it, err := batchLoadEpisodeMediaByKindAndGlobalIDs(db, userID, LibraryTypeTV, tvIDs)
		if err != nil {
			return nil, err
		}
		out = append(out, it...)
	}
	if len(animeIDs) > 0 {
		it, err := batchLoadEpisodeMediaByKindAndGlobalIDs(db, userID, LibraryTypeAnime, animeIDs)
		if err != nil {
			return nil, err
		}
		out = append(out, it...)
	}
	return out, nil
}

func batchLoadMoviesByGlobalIDsForUser(db *sql.DB, userID int, globalIDs []int) ([]MediaItem, error) {
	placeholders := make([]string, len(globalIDs))
	args := make([]interface{}, 0, len(globalIDs)+1)
	args = append(args, userID)
	for i, id := range globalIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	q := `SELECT g.id, m.library_id, m.title, m.path, m.duration, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.missing_since, ''), m.match_status, m.tmdb_id, m.tvdb_id, m.overview, m.poster_path, m.backdrop_path, m.release_date, m.vote_average, m.imdb_id, m.imdb_rating
FROM movies m
JOIN media_global g ON g.kind = 'movie' AND g.ref_id = m.id
JOIN libraries l ON l.id = m.library_id AND l.user_id = ?
WHERE g.id IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMovieMediaItems(rows)
}

func batchLoadEpisodeMediaByKindAndGlobalIDs(db *sql.DB, userID int, kind string, globalIDs []int) ([]MediaItem, error) {
	table := mediaTableForKind(kind)
	if table != "tv_episodes" && table != "anime_episodes" {
		return nil, nil
	}
	placeholders := make([]string, len(globalIDs))
	args := make([]interface{}, 0, len(globalIDs)+2)
	args = append(args, kind, userID)
	for i, id := range globalIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	q := `SELECT g.id, m.library_id, m.title, m.path, m.duration, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.missing_since, ''), m.match_status, m.tmdb_id, m.tvdb_id, m.overview, m.poster_path, m.backdrop_path, m.release_date, m.vote_average, m.imdb_id, m.imdb_rating, COALESCE(m.season, 0), COALESCE(m.episode, 0), COALESCE(m.metadata_review_needed, 0), COALESCE(m.metadata_confirmed, 0), m.thumbnail_path, COALESCE(s.poster_path, ''), COALESCE(s.vote_average, 0), COALESCE(s.imdb_rating, 0)
FROM ` + table + ` m
JOIN media_global g ON g.kind = ? AND g.ref_id = m.id
JOIN libraries l ON l.id = m.library_id AND l.user_id = ?
LEFT JOIN shows s ON s.id = m.show_id
WHERE g.id IN (` + strings.Join(placeholders, ",") + `)
ORDER BY g.id`
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]MediaItem, 0, len(globalIDs))
	for rows.Next() {
		var m MediaItem
		m.Type = kind
		var libID int
		var overview, posterPath, backdropPath, releaseDate, thumbnailPath, matchStatus, imdbID sql.NullString
		var showPosterPath sql.NullString
		var voteAvg, showVoteAvg, showImdbAvg, imdbRating sql.NullFloat64
		var tmdbID sql.NullInt64
		var tvdbID sql.NullString
		var metadataReviewNeeded sql.NullBool
		var metadataConfirmed sql.NullBool
		err := rows.Scan(&m.ID, &libID, &m.Title, &m.Path, &m.Duration, &m.FileSizeBytes, &m.FileModTime, &m.FileHash, &m.FileHashKind, &m.MissingSince, &matchStatus, &tmdbID, &tvdbID, &overview, &posterPath, &backdropPath, &releaseDate, &voteAvg, &imdbID, &imdbRating, &m.Season, &m.Episode, &metadataReviewNeeded, &metadataConfirmed, &thumbnailPath, &showPosterPath, &showVoteAvg, &showImdbAvg)
		if err != nil {
			return nil, err
		}
		m.LibraryID = libID
		m.TMDBID = int(tmdbID.Int64)
		if tvdbID.Valid {
			m.TVDBID = tvdbID.String
		}
		if overview.Valid {
			m.Overview = overview.String
		}
		if posterPath.Valid {
			m.PosterPath = posterPath.String
		}
		if backdropPath.Valid {
			m.BackdropPath = backdropPath.String
		}
		if releaseDate.Valid {
			m.ReleaseDate = releaseDate.String
		}
		if voteAvg.Valid {
			m.VoteAverage = voteAvg.Float64
		}
		if imdbID.Valid {
			m.IMDbID = imdbID.String
		}
		if imdbRating.Valid {
			m.IMDbRating = imdbRating.Float64
		}
		if metadataReviewNeeded.Valid {
			m.MetadataReviewNeeded = metadataReviewNeeded.Bool
		}
		if metadataConfirmed.Valid {
			m.MetadataConfirmed = metadataConfirmed.Bool
		}
		if thumbnailPath.Valid {
			m.ThumbnailPath = thumbnailPath.String
		}
		if showPosterPath.Valid {
			m.ShowPosterPath = showPosterPath.String
		}
		if showVoteAvg.Valid {
			m.ShowVoteAverage = showVoteAvg.Float64
		}
		if showImdbAvg.Valid {
			m.ShowIMDbRating = showImdbAvg.Float64
		}
		if matchStatus.Valid {
			m.MatchStatus = matchStatus.String
		}
		m.Missing = m.MissingSince != ""
		items = append(items, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	byLibrary := make(map[int][]MediaItem)
	for _, it := range items {
		lib := it.LibraryID
		byLibrary[lib] = append(byLibrary[lib], it)
	}
	out := make([]MediaItem, 0, len(items))
	for libID, chunk := range byLibrary {
		k := chunk[0].Type
		if err := hydrateEpisodeShowPosters(db, libID, k, chunk); err != nil {
			return nil, err
		}
		out = append(out, chunk...)
	}
	return out, nil
}

// enrichHomeDashboardShowTitles replaces show_title on episodic dashboard entries with the
// canonical series title from shows when available (same source as the library show view).
func enrichHomeDashboardShowTitles(db *sql.DB, dash *HomeDashboard) {
	patchCW := func(entries []ContinueWatchingEntry) {
		for i := range entries {
			e := &entries[i]
			if e.Kind != "show" && e.Kind != "episode" {
				continue
			}
			if t := canonicalShowTitleFromShowsTable(db, e.ShowKey, e.Media.LibraryID, e.Media.Type); t != "" {
				e.ShowTitle = t
			}
		}
	}
	patchCW(dash.ContinueWatching)

	patchRA := func(entries []RecentlyAddedEntry) {
		for i := range entries {
			e := &entries[i]
			if e.Kind != "show" && e.Kind != "episode" {
				continue
			}
			if t := canonicalShowTitleFromShowsTable(db, e.ShowKey, e.Media.LibraryID, e.Media.Type); t != "" {
				e.ShowTitle = t
			}
		}
	}
	patchRA(dash.RecentlyAddedTvEpisodes)
	patchRA(dash.RecentlyAddedTvShows)
	patchRA(dash.RecentlyAddedAnimeEpisodes)
	patchRA(dash.RecentlyAddedAnimeShows)
}

func canonicalShowTitleFromShowsTable(db *sql.DB, showKey string, libraryID int, mediaType string) string {
	if showKey == "" || libraryID <= 0 {
		return ""
	}
	if mediaType != LibraryTypeTV && mediaType != LibraryTypeAnime {
		return ""
	}
	title, err := lookupShowTitleByShowKey(db, libraryID, mediaType, showKey)
	if err != nil || title == "" {
		return ""
	}
	return title
}

func attachHomeDashboardSubtitles(db *sql.DB, dash *HomeDashboard) error {
	uniqueMedia := make(map[int]MediaItem)
	for i := range dash.ContinueWatching {
		uniqueMedia[dash.ContinueWatching[i].Media.ID] = dash.ContinueWatching[i].Media
	}
	mergeRecently := func(entries []RecentlyAddedEntry) {
		for i := range entries {
			uniqueMedia[entries[i].Media.ID] = entries[i].Media
		}
	}
	mergeRecently(dash.RecentlyAddedTvEpisodes)
	mergeRecently(dash.RecentlyAddedTvShows)
	mergeRecently(dash.RecentlyAddedMovies)
	mergeRecently(dash.RecentlyAddedAnimeEpisodes)
	mergeRecently(dash.RecentlyAddedAnimeShows)

	items := make([]MediaItem, 0, len(uniqueMedia))
	for _, item := range uniqueMedia {
		items = append(items, item)
	}
	items, err := attachSubtitlesBatch(db, items)
	if err != nil {
		return err
	}
	mediaByID := make(map[int]MediaItem, len(items))
	for _, item := range items {
		mediaByID[item.ID] = item
	}
	for i := range dash.ContinueWatching {
		if item, ok := mediaByID[dash.ContinueWatching[i].Media.ID]; ok {
			dash.ContinueWatching[i].Media = item
		}
	}
	patchRecently := func(entries []RecentlyAddedEntry) {
		for i := range entries {
			if item, ok := mediaByID[entries[i].Media.ID]; ok {
				entries[i].Media = item
			}
		}
	}
	patchRecently(dash.RecentlyAddedTvEpisodes)
	patchRecently(dash.RecentlyAddedTvShows)
	patchRecently(dash.RecentlyAddedMovies)
	patchRecently(dash.RecentlyAddedAnimeEpisodes)
	patchRecently(dash.RecentlyAddedAnimeShows)
	return nil
}

func partitionRecentDashboardCandidatesByType(items []MediaItem) (tv, anime, movies []MediaItem) {
	for _, item := range items {
		switch item.Type {
		case LibraryTypeTV:
			tv = append(tv, item)
		case LibraryTypeAnime:
			anime = append(anime, item)
		case LibraryTypeMovie:
			movies = append(movies, item)
		}
	}
	return tv, anime, movies
}

func buildContinueWatching(items []MediaItem) []ContinueWatchingEntry {
	movies := make([]ContinueWatchingEntry, 0)
	showItems := make(map[string][]MediaItem)
	for _, item := range items {
		if item.Type == LibraryTypeMusic {
			continue
		}
		if item.Type == LibraryTypeMovie {
			if item.Completed || item.ProgressPercent <= 0 {
				continue
			}
			entry := ContinueWatchingEntry{
				Kind:             "movie",
				Media:            item,
				RemainingSeconds: item.RemainingSeconds,
				activityAt:       item.LastWatchedAt,
			}
			movies = append(movies, entry)
			continue
		}
		if item.Type != LibraryTypeTV && item.Type != LibraryTypeAnime {
			continue
		}
		key := showKeyFromItem(item.TMDBID, item.Title)
		showItems[key] = append(showItems[key], item)
	}

	entries := make([]ContinueWatchingEntry, 0, len(movies)+len(showItems))
	entries = append(entries, movies...)
	for showKey, episodes := range showItems {
		if entry, ok := continueWatchingEntryForShow(showKey, episodes); ok {
			entries = append(entries, entry)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].activityAt > entries[j].activityAt
	})

	return entries
}

func buildRecentlyAddedEpisodeEntries(items []MediaItem, limit int) []RecentlyAddedEntry {
	if limit <= 0 {
		return nil
	}
	episodes := append([]MediaItem(nil), items...)
	sort.Slice(episodes, func(i, j int) bool {
		if episodes[i].ID != episodes[j].ID {
			return episodes[i].ID > episodes[j].ID
		}
		return episodes[i].Title < episodes[j].Title
	})
	out := make([]RecentlyAddedEntry, 0, min(len(episodes), limit))
	for _, ep := range episodes {
		if len(out) >= limit {
			break
		}
		out = append(out, recentlyAddedEpisodeEntry(ep))
	}
	return out
}

func recentlyAddedEpisodeEntry(ep MediaItem) RecentlyAddedEntry {
	key := showKeyFromItem(ep.TMDBID, ep.Title)
	return RecentlyAddedEntry{
		Kind:         "episode",
		Media:        ep,
		ShowKey:      key,
		ShowTitle:    showTitleFromEpisodeTitle(ep.Title),
		EpisodeLabel: episodeLabel(ep),
	}
}

func buildRecentlyAddedShowsForKind(items []MediaItem, kind string) []RecentlyAddedEntry {
	showItems := make(map[string][]MediaItem)
	for _, item := range items {
		if item.Type != kind {
			continue
		}
		key := showKeyFromItem(item.TMDBID, item.Title)
		showItems[key] = append(showItems[key], item)
	}
	entries := make([]RecentlyAddedEntry, 0, len(showItems))
	for showKey, eps := range showItems {
		entry, ok := recentlyAddedEntryForShow(showKey, eps)
		if !ok {
			continue
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Media.ID != entries[j].Media.ID {
			return entries[i].Media.ID > entries[j].Media.ID
		}
		return entries[i].Media.Title < entries[j].Media.Title
	})
	if len(entries) > recentlyAddedLimit {
		return entries[:recentlyAddedLimit]
	}
	return entries
}

func buildRecentlyAddedMovieEntries(items []MediaItem) []RecentlyAddedEntry {
	movies := make([]MediaItem, 0)
	for _, item := range items {
		if item.Type == LibraryTypeMovie {
			movies = append(movies, item)
		}
	}
	sort.Slice(movies, func(i, j int) bool {
		if movies[i].ID != movies[j].ID {
			return movies[i].ID > movies[j].ID
		}
		return movies[i].Title < movies[j].Title
	})
	out := make([]RecentlyAddedEntry, 0, min(len(movies), recentlyAddedLimit))
	for _, m := range movies {
		if len(out) >= recentlyAddedLimit {
			break
		}
		out = append(out, RecentlyAddedEntry{Kind: "movie", Media: m})
	}
	return out
}

func continueWatchingEntryForShow(showKey string, episodes []MediaItem) (ContinueWatchingEntry, bool) {
	if len(episodes) == 0 {
		return ContinueWatchingEntry{}, false
	}
	sort.Slice(episodes, func(i, j int) bool {
		if episodes[i].Season != episodes[j].Season {
			return episodes[i].Season < episodes[j].Season
		}
		if episodes[i].Episode != episodes[j].Episode {
			return episodes[i].Episode < episodes[j].Episode
		}
		return episodes[i].Title < episodes[j].Title
	})

	var partial *MediaItem
	for i := range episodes {
		if episodes[i].Completed || episodes[i].ProgressPercent <= 0 {
			continue
		}
		if partial == nil || partial.LastWatchedAt < episodes[i].LastWatchedAt {
			partial = &episodes[i]
		}
	}
	if partial != nil {
		return buildShowContinueWatchingEntry(showKey, *partial, partial.LastWatchedAt), true
	}

	var latestCompletedIndex = -1
	var latestCompletedAt string
	for i := range episodes {
		if !episodes[i].Completed || episodes[i].LastWatchedAt == "" {
			continue
		}
		if latestCompletedIndex < 0 || latestCompletedAt < episodes[i].LastWatchedAt {
			latestCompletedIndex = i
			latestCompletedAt = episodes[i].LastWatchedAt
		}
	}
	if latestCompletedIndex < 0 {
		return ContinueWatchingEntry{}, false
	}
	for i := latestCompletedIndex + 1; i < len(episodes); i++ {
		if episodes[i].Completed {
			continue
		}
		return buildShowContinueWatchingEntry(showKey, episodes[i], latestCompletedAt), true
	}
	return ContinueWatchingEntry{}, false
}

func buildShowContinueWatchingEntry(showKey string, item MediaItem, activityAt string) ContinueWatchingEntry {
	return ContinueWatchingEntry{
		Kind:             "show",
		Media:            item,
		ShowKey:          showKey,
		ShowTitle:        showTitleFromEpisodeTitle(item.Title),
		EpisodeLabel:     episodeLabel(item),
		RemainingSeconds: item.RemainingSeconds,
		activityAt:       activityAt,
	}
}

func recentlyAddedEntryForShow(showKey string, episodes []MediaItem) (RecentlyAddedEntry, bool) {
	if len(episodes) == 0 {
		return RecentlyAddedEntry{}, false
	}
	newest := episodes[0]
	for i := 1; i < len(episodes); i++ {
		if episodes[i].ID > newest.ID {
			newest = episodes[i]
		}
	}
	return RecentlyAddedEntry{
		Kind:         "show",
		Media:        newest,
		ShowKey:      showKey,
		ShowTitle:    showTitleFromEpisodeTitle(newest.Title),
		EpisodeLabel: episodeLabel(newest),
	}, true
}

func UpsertPlaybackProgress(db *sql.DB, userID, mediaID int, positionSeconds, durationSeconds float64, completed bool) error {
	if userID <= 0 || mediaID <= 0 {
		return fmt.Errorf("user and media ids are required")
	}
	if positionSeconds < 0 {
		positionSeconds = 0
	}
	if durationSeconds < 0 {
		durationSeconds = 0
	}
	progressPercent := 0.0
	if durationSeconds > 0 {
		progressPercent = (positionSeconds / durationSeconds) * 100
		if progressPercent < 0 {
			progressPercent = 0
		}
		if progressPercent > 100 {
			progressPercent = 100
		}
	}
	remainingSeconds := durationSeconds - positionSeconds
	if remainingSeconds < 0 {
		remainingSeconds = 0
	}
	if !completed && (progressPercent >= completedProgressPercent || (durationSeconds > 0 && remainingSeconds <= completedRemainingSecs)) {
		completed = true
	}
	if completed {
		positionSeconds = 0
		progressPercent = 100
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(`
INSERT INTO playback_progress (
  user_id, media_id, position_seconds, duration_seconds, progress_percent, completed, last_watched_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(user_id, media_id) DO UPDATE SET
  position_seconds = excluded.position_seconds,
  duration_seconds = excluded.duration_seconds,
  progress_percent = excluded.progress_percent,
  completed = excluded.completed,
  last_watched_at = excluded.last_watched_at,
  updated_at = excluded.updated_at
`,
		userID,
		mediaID,
		positionSeconds,
		durationSeconds,
		progressPercent,
		completed,
		now,
		now,
		now,
	)
	return err
}

func attachPlaybackProgressBatch(db *sql.DB, userID int, items []MediaItem) ([]MediaItem, error) {
	if userID <= 0 || len(items) == 0 {
		return items, nil
	}
	ids := make([]int, len(items))
	for i := range items {
		ids[i] = items[i].ID
	}
	progressByID, err := getPlaybackProgressByMediaIDs(db, userID, ids)
	if err != nil {
		return nil, err
	}
	for i := range items {
		progress, ok := progressByID[items[i].ID]
		if !ok {
			continue
		}
		items[i].ProgressSeconds = progress.PositionSeconds
		items[i].ProgressPercent = progress.ProgressPercent
		items[i].Completed = progress.Completed
		items[i].LastWatchedAt = progress.LastWatchedAt
		if duration := progress.DurationSeconds; duration > 0 {
			remaining := duration - progress.PositionSeconds
			if remaining < 0 {
				remaining = 0
			}
			items[i].RemainingSeconds = remaining
		}
	}
	return items, nil
}

// AttachPlaybackProgressToLibraryMovieDetails fills the playback fields on a movie details response.
func AttachPlaybackProgressToLibraryMovieDetails(db *sql.DB, userID int, mediaID int, details *LibraryMovieDetails) error {
	if details == nil || userID <= 0 || mediaID <= 0 {
		return nil
	}
	progressByID, err := getPlaybackProgressByMediaIDs(db, userID, []int{mediaID})
	if err != nil {
		return err
	}
	progress, ok := progressByID[mediaID]
	if !ok {
		details.ProgressSeconds = nil
		details.ProgressPercent = nil
		details.Completed = nil
		return nil
	}
	details.ProgressSeconds = &progress.PositionSeconds
	details.ProgressPercent = &progress.ProgressPercent
	details.Completed = &progress.Completed
	return nil
}

func getPlaybackProgressByMediaIDs(db *sql.DB, userID int, mediaIDs []int) (map[int]playbackProgressRow, error) {
	if len(mediaIDs) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(mediaIDs))
	args := make([]any, 0, len(mediaIDs)+1)
	args = append(args, userID)
	for i, mediaID := range mediaIDs {
		placeholders[i] = "?"
		args = append(args, mediaID)
	}
	rows, err := db.Query(
		`SELECT media_id, position_seconds, duration_seconds, progress_percent, completed, COALESCE(last_watched_at, '') FROM playback_progress WHERE user_id = ? AND media_id IN (`+strings.Join(placeholders, ",")+`)`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[int]playbackProgressRow, len(mediaIDs))
	for rows.Next() {
		var mediaID int
		var progress playbackProgressRow
		if err := rows.Scan(&mediaID, &progress.PositionSeconds, &progress.DurationSeconds, &progress.ProgressPercent, &progress.Completed, &progress.LastWatchedAt); err != nil {
			return nil, err
		}
		out[mediaID] = progress
	}
	return out, rows.Err()
}

func showTitleFromEpisodeTitle(title string) string {
	if i := strings.Index(strings.ToLower(title), " - s"); i > 0 {
		return strings.TrimSpace(title[:i])
	}
	if i := strings.Index(title, " - "); i > 0 {
		return strings.TrimSpace(title[:i])
	}
	return strings.TrimSpace(title)
}

func episodeLabel(item MediaItem) string {
	season := item.Season
	episode := item.Episode
	if season <= 0 && episode <= 0 {
		return ""
	}
	return fmt.Sprintf("S%02dE%02d", season, episode)
}

// ErrShowNotFound is returned when the library is not TV/anime or the show key does not resolve to a show row.
var ErrShowNotFound = errors.New("show not found")

// GetLibraryShowEpisodesForUser returns all episode media rows for a show key, with progress and files attached.
func GetLibraryShowEpisodesForUser(db *sql.DB, libraryID, userID int, showKey string) ([]MediaItem, error) {
	var libraryType string
	err := db.QueryRow(`SELECT type FROM libraries WHERE id = ?`, libraryID).Scan(&libraryType)
	if err != nil {
		return nil, err
	}
	if libraryType != LibraryTypeTV && libraryType != LibraryTypeAnime {
		return nil, ErrShowNotFound
	}
	if err := ensureLibraryShowsAndSeasons(db, libraryID, libraryType); err != nil {
		return nil, err
	}
	showID, _, _, _, _, _, _, _, _, _, _, err := getShowCanonicalMetadata(db, libraryID, libraryType, showKey)
	if err != nil {
		return nil, err
	}
	if showID == 0 {
		return nil, ErrShowNotFound
	}
	items, err := queryMediaByShowID(db, libraryID, libraryType, showID)
	if err != nil {
		return nil, err
	}
	items, err = attachMediaFilesBatch(db, items)
	if err != nil {
		return nil, err
	}
	items, err = attachPlaybackProgressBatch(db, userID, items)
	if err != nil {
		return nil, err
	}
	return attachDuplicateState(db, items)
}
