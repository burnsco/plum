package httpapi

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"plum/internal/arr"
	"plum/internal/db"
	"plum/internal/metadata"
)

type LibraryHandler struct {
	DB            *sql.DB
	Meta          metadata.Identifier
	Artwork       metadata.MetadataArtworkProvider
	Movies        metadata.MovieDetailsProvider
	MovieQuery    metadata.MovieProvider
	MovieLookup   metadata.MovieLookupProvider
	Series        metadata.SeriesDetailsProvider
	SeriesQuery   metadata.SeriesSearchProvider
	Discover      metadata.DiscoverProvider
	Arr           *arr.Service
	ScanJobs      *LibraryScanManager
	SearchIndex   *SearchIndexManager
	identifyRunMu sync.RWMutex
	identifyRun   *identifyRunTracker

	// librarySideJobsMu serializes checks and updates across playback refresh jobs
	// so at most one side job runs per library at a time.
	librarySideJobsMu     sync.Mutex
	playbackRefreshStatus map[int]*playbackRefreshProgress
}

type identifyRunTracker struct {
	mu      sync.RWMutex
	byLibID map[int]map[string]string
}

func newIdentifyRunTracker() *identifyRunTracker {
	return &identifyRunTracker{
		byLibID: make(map[int]map[string]string),
	}
}

func identifyRowKey(kind, path string) string {
	return kind + ":" + path
}

func (t *identifyRunTracker) startLibrary(libraryID int, rows []db.IdentificationRow) {
	if t == nil {
		return
	}
	states := make(map[string]string, len(rows))
	for _, row := range rows {
		states[identifyRowKey(row.Kind, row.Path)] = "queued"
	}
	t.mu.Lock()
	t.byLibID[libraryID] = states
	t.mu.Unlock()
}

func (t *identifyRunTracker) setState(libraryID int, kind, path, state string) {
	if t == nil {
		return
	}
	key := identifyRowKey(kind, path)
	t.mu.Lock()
	defer t.mu.Unlock()
	states, ok := t.byLibID[libraryID]
	if !ok {
		return
	}
	if state == "" {
		delete(states, key)
		return
	}
	states[key] = state
}

func (t *identifyRunTracker) failRows(libraryID int, rows []db.IdentificationRow) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	states, ok := t.byLibID[libraryID]
	if !ok {
		return
	}
	for _, row := range rows {
		states[identifyRowKey(row.Kind, row.Path)] = "failed"
	}
}

func (t *identifyRunTracker) finishLibrary(libraryID int) {
	if t == nil {
		return
	}
	t.mu.Lock()
	states := t.byLibID[libraryID]
	if len(states) == 0 {
		delete(t.byLibID, libraryID)
		t.mu.Unlock()
		return
	}
	failedOnly := make(map[string]string)
	for key, value := range states {
		if value == "failed" {
			failedOnly[key] = value
		}
	}
	if len(failedOnly) == 0 {
		delete(t.byLibID, libraryID)
	} else {
		t.byLibID[libraryID] = failedOnly
	}
	t.mu.Unlock()
}

func (t *identifyRunTracker) stateForLibrary(libraryID int) map[string]string {
	if t == nil {
		return nil
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	states := t.byLibID[libraryID]
	if len(states) == 0 {
		return nil
	}
	out := make(map[string]string, len(states))
	for key, value := range states {
		out[key] = value
	}
	return out
}

func (t *identifyRunTracker) clearLibrary(libraryID int) {
	if t == nil {
		return
	}
	t.mu.Lock()
	delete(t.byLibID, libraryID)
	t.mu.Unlock()
}

func (h *LibraryHandler) getIdentifyRun() *identifyRunTracker {
	h.identifyRunMu.RLock()
	defer h.identifyRunMu.RUnlock()
	return h.identifyRun
}

func (h *LibraryHandler) ensureIdentifyRun() *identifyRunTracker {
	h.identifyRunMu.Lock()
	defer h.identifyRunMu.Unlock()
	if h.identifyRun == nil {
		h.identifyRun = newIdentifyRunTracker()
	}
	return h.identifyRun
}

type createLibraryRequest struct {
	Name                string `json:"name"`
	Type                string `json:"type"`
	Path                string `json:"path"`
	WatcherEnabled      *bool  `json:"watcher_enabled,omitempty"`
	WatcherMode         string `json:"watcher_mode,omitempty"`
	ScanIntervalMinutes *int   `json:"scan_interval_minutes,omitempty"`
}

type updateLibraryPlaybackPreferencesRequest struct {
	PreferredAudioLanguage    string `json:"preferred_audio_language"`
	PreferredSubtitleLanguage string `json:"preferred_subtitle_language"`
	SubtitlesEnabledByDefault bool   `json:"subtitles_enabled_by_default"`
	IntroSkipMode             string `json:"intro_skip_mode,omitempty"`
	WatcherEnabled            *bool  `json:"watcher_enabled,omitempty"`
	WatcherMode               string `json:"watcher_mode,omitempty"`
	ScanIntervalMinutes       *int   `json:"scan_interval_minutes,omitempty"`
}

type libraryResponse struct {
	ID                        int    `json:"id"`
	Name                      string `json:"name"`
	Type                      string `json:"type"`
	Path                      string `json:"path"`
	UserID                    int    `json:"user_id"`
	PreferredAudioLanguage    string `json:"preferred_audio_language,omitempty"`
	PreferredSubtitleLanguage string `json:"preferred_subtitle_language,omitempty"`
	SubtitlesEnabledByDefault bool   `json:"subtitles_enabled_by_default"`
	WatcherEnabled            bool   `json:"watcher_enabled"`
	WatcherMode               string `json:"watcher_mode,omitempty"`
	ScanIntervalMinutes       int    `json:"scan_interval_minutes"`
}

type libraryBrowseItemResponse struct {
	ID                   int      `json:"id"`
	LibraryID            int      `json:"library_id,omitempty"`
	Title                string   `json:"title"`
	Path                 string   `json:"path"`
	Duration             int      `json:"duration"`
	Type                 string   `json:"type"`
	MatchStatus          string   `json:"match_status,omitempty"`
	IdentifyState        string   `json:"identify_state,omitempty"`
	TMDBID               int      `json:"tmdb_id,omitempty"`
	TVDBID               string   `json:"tvdb_id,omitempty"`
	Overview             string   `json:"overview,omitempty"`
	PosterPath           string   `json:"poster_path,omitempty"`
	BackdropPath         string   `json:"backdrop_path,omitempty"`
	PosterURL            string   `json:"poster_url,omitempty"`
	BackdropURL          string   `json:"backdrop_url,omitempty"`
	ShowPosterPath       string   `json:"show_poster_path,omitempty"`
	ShowPosterURL        string   `json:"show_poster_url,omitempty"`
	ShowTitle            string   `json:"show_title,omitempty"`
	ReleaseDate          string   `json:"release_date,omitempty"`
	ShowVoteAverage      float64  `json:"show_vote_average,omitempty"`
	ShowIMDbRating       float64  `json:"show_imdb_rating,omitempty"`
	VoteAverage          float64  `json:"vote_average,omitempty"`
	IMDbID               string   `json:"imdb_id,omitempty"`
	IMDbRating           float64  `json:"imdb_rating,omitempty"`
	Artist               string   `json:"artist,omitempty"`
	Album                string   `json:"album,omitempty"`
	AlbumArtist          string   `json:"album_artist,omitempty"`
	DiscNumber           int      `json:"disc_number,omitempty"`
	TrackNumber          int      `json:"track_number,omitempty"`
	ReleaseYear          int      `json:"release_year,omitempty"`
	ProgressSeconds      float64  `json:"progress_seconds,omitempty"`
	ProgressPercent      float64  `json:"progress_percent,omitempty"`
	RemainingSeconds     float64  `json:"remaining_seconds,omitempty"`
	Completed            bool     `json:"completed,omitempty"`
	LastWatchedAt        string   `json:"last_watched_at,omitempty"`
	Season               int      `json:"season,omitempty"`
	Episode              int      `json:"episode,omitempty"`
	MetadataReviewNeeded bool     `json:"metadata_review_needed,omitempty"`
	MetadataConfirmed    bool     `json:"metadata_confirmed,omitempty"`
	ThumbnailPath        string   `json:"thumbnail_path,omitempty"`
	ThumbnailURL         string   `json:"thumbnail_url,omitempty"`
	Missing              bool     `json:"missing,omitempty"`
	MissingSince         string   `json:"missing_since,omitempty"`
	IntroStartSeconds    *float64 `json:"intro_start_seconds,omitempty"`
	IntroEndSeconds      *float64 `json:"intro_end_seconds,omitempty"`
}

type libraryMediaPageResponse struct {
	Items      []libraryBrowseItemResponse `json:"items"`
	NextOffset *int                        `json:"next_offset,omitempty"`
	HasMore    bool                        `json:"has_more"`
	Total      int                         `json:"total,omitempty"`
}

func buildLibraryBrowseItemResponse(item db.MediaItem) libraryBrowseItemResponse {
	resp := libraryBrowseItemResponse{
		ID:                   item.ID,
		LibraryID:            item.LibraryID,
		Title:                item.Title,
		Path:                 item.Path,
		Duration:             item.Duration,
		Type:                 item.Type,
		MatchStatus:          item.MatchStatus,
		IdentifyState:        item.IdentifyState,
		TMDBID:               item.TMDBID,
		TVDBID:               item.TVDBID,
		Overview:             item.Overview,
		PosterPath:           item.PosterPath,
		BackdropPath:         item.BackdropPath,
		PosterURL:            item.PosterURL,
		BackdropURL:          item.BackdropURL,
		ShowPosterPath:       item.ShowPosterPath,
		ShowPosterURL:        item.ShowPosterURL,
		ShowTitle:            item.ShowTitle,
		ReleaseDate:          item.ReleaseDate,
		ShowVoteAverage:      item.ShowVoteAverage,
		ShowIMDbRating:       item.ShowIMDbRating,
		VoteAverage:          item.VoteAverage,
		IMDbID:               item.IMDbID,
		IMDbRating:           item.IMDbRating,
		Artist:               item.Artist,
		Album:                item.Album,
		AlbumArtist:          item.AlbumArtist,
		DiscNumber:           item.DiscNumber,
		TrackNumber:          item.TrackNumber,
		ReleaseYear:          item.ReleaseYear,
		ProgressSeconds:      item.ProgressSeconds,
		ProgressPercent:      item.ProgressPercent,
		RemainingSeconds:     item.RemainingSeconds,
		Completed:            item.Completed,
		LastWatchedAt:        item.LastWatchedAt,
		Season:               item.Season,
		Episode:              item.Episode,
		MetadataReviewNeeded: item.MetadataReviewNeeded,
		MetadataConfirmed:    item.MetadataConfirmed,
		ThumbnailPath:        item.ThumbnailPath,
		ThumbnailURL:         item.ThumbnailURL,
		Missing:              item.Missing,
		MissingSince:         item.MissingSince,
		IntroStartSeconds:    item.IntroStartSeconds,
		IntroEndSeconds:      item.IntroEndSeconds,
	}
	if item.Type == db.LibraryTypeTV || item.Type == db.LibraryTypeAnime {
		// Ratings apply to the series/movie identity, not individual episodes.
		resp.VoteAverage = 0
		resp.IMDbRating = 0
		resp.IMDbID = ""
	}
	return resp
}

func defaultLibraryPlaybackPreferences(libraryType string) (preferredAudio string, preferredSubtitle string, subtitlesEnabled bool) {
	switch libraryType {
	case db.LibraryTypeAnime:
		return "ja", "en", true
	case db.LibraryTypeMovie, db.LibraryTypeTV:
		return "en", "en", true
	default:
		return "", "", false
	}
}

func defaultLibraryAutomation() (watcherEnabled bool, watcherMode string, scanIntervalMinutes int) {
	return true, db.LibraryWatcherModeAuto, 0
}

func normalizeLibraryAutomationWithDefaults(
	defaultEnabled bool,
	defaultMode string,
	defaultInterval int,
	watcherEnabled *bool,
	watcherMode string,
	scanIntervalMinutes *int,
) (bool, string, int) {
	enabled := defaultEnabled
	if watcherEnabled != nil {
		enabled = *watcherEnabled
	}
	mode := strings.TrimSpace(strings.ToLower(watcherMode))
	if mode == "" {
		mode = defaultMode
	}
	if mode != db.LibraryWatcherModeAuto && mode != db.LibraryWatcherModePoll {
		mode = defaultMode
	}
	interval := defaultInterval
	if scanIntervalMinutes != nil && *scanIntervalMinutes > 0 {
		interval = *scanIntervalMinutes
	}
	return enabled, mode, interval
}

func normalizeLibraryAutomation(
	watcherEnabled *bool,
	watcherMode string,
	scanIntervalMinutes *int,
) (bool, string, int) {
	defaultEnabled, defaultMode, defaultInterval := defaultLibraryAutomation()
	return normalizeLibraryAutomationWithDefaults(defaultEnabled, defaultMode, defaultInterval, watcherEnabled, watcherMode, scanIntervalMinutes)
}

func buildLibraryResponse(
	id int,
	name string,
	libraryType string,
	path string,
	userID int,
	preferredAudio sql.NullString,
	preferredSubtitle sql.NullString,
	subtitlesEnabled sql.NullBool,
	watcherEnabled sql.NullBool,
	watcherMode sql.NullString,
	scanIntervalMinutes sql.NullInt64,
) libraryResponse {
	defaultAudio, defaultSubtitle, defaultSubtitlesEnabled := defaultLibraryPlaybackPreferences(libraryType)
	defaultWatcherEnabled, defaultWatcherMode, defaultScanIntervalMinutes := defaultLibraryAutomation()
	return libraryResponse{
		ID:                        id,
		Name:                      name,
		Type:                      libraryType,
		Path:                      path,
		UserID:                    userID,
		PreferredAudioLanguage:    strings.TrimSpace(coalesceNullableString(preferredAudio, defaultAudio)),
		PreferredSubtitleLanguage: strings.TrimSpace(coalesceNullableString(preferredSubtitle, defaultSubtitle)),
		SubtitlesEnabledByDefault: coalesceNullableBool(subtitlesEnabled, defaultSubtitlesEnabled),
		WatcherEnabled:            coalesceNullableBool(watcherEnabled, defaultWatcherEnabled),
		WatcherMode:               strings.TrimSpace(coalesceNullableString(watcherMode, defaultWatcherMode)),
		ScanIntervalMinutes:       coalesceNullableInt(scanIntervalMinutes, defaultScanIntervalMinutes),
	}
}

func coalesceNullableString(value sql.NullString, fallback string) string {
	if value.Valid {
		return value.String
	}
	return fallback
}

func coalesceNullableBool(value sql.NullBool, fallback bool) bool {
	if value.Valid {
		return value.Bool
	}
	return fallback
}

func coalesceNullableInt(value sql.NullInt64, fallback int) int {
	if value.Valid {
		return int(value.Int64)
	}
	return fallback
}

func isMissingColumnError(err error, column string) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "no such column: "+strings.ToLower(column)) ||
		strings.Contains(text, "has no column named "+strings.ToLower(column))
}

func isSQLiteBusyError(err error) bool {
	return db.IsSQLiteBusy(err)
}

func discoverHTTPStatus(err error) (int, string) {
	if err == nil {
		return http.StatusOK, ""
	}
	if errors.Is(err, metadata.ErrTMDBNotConfigured) {
		return http.StatusServiceUnavailable, err.Error()
	}
	return http.StatusInternalServerError, "discover failed: " + err.Error()
}

func (h *LibraryHandler) CreateLibrary(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var payload createLibraryRequest
	if !decodeRequestJSON(w, r, &payload) {
		return
	}
	if payload.Name == "" || payload.Path == "" || payload.Type == "" {
		http.Error(w, "name, path and type are required", http.StatusBadRequest)
		return
	}
	switch payload.Type {
	case db.LibraryTypeTV, db.LibraryTypeMovie, db.LibraryTypeMusic, db.LibraryTypeAnime:
		// allowed
	default:
		http.Error(w, "type must be tv, movie, music, or anime", http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()
	var libID int
	defaultAudio, defaultSubtitle, subtitlesEnabled := defaultLibraryPlaybackPreferences(payload.Type)
	watcherEnabled, watcherMode, scanIntervalMinutes := normalizeLibraryAutomation(
		payload.WatcherEnabled,
		payload.WatcherMode,
		payload.ScanIntervalMinutes,
	)
	err := retryCreateLibraryInsert(
		h.DB,
		u.ID,
		payload,
		defaultAudio,
		defaultSubtitle,
		subtitlesEnabled,
		watcherEnabled,
		watcherMode,
		scanIntervalMinutes,
		now,
		&libID,
	)
	if err != nil {
		slog.Error("create library", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if h.ScanJobs != nil {
		h.ScanJobs.ConfigureLibraryAutomation(libID, payload.Path, payload.Type, watcherEnabled, watcherMode, scanIntervalMinutes)
	}

	writeJSON(w, http.StatusOK, libraryResponse{
		ID:                        libID,
		Name:                      payload.Name,
		Type:                      payload.Type,
		Path:                      payload.Path,
		UserID:                    u.ID,
		PreferredAudioLanguage:    defaultAudio,
		PreferredSubtitleLanguage: defaultSubtitle,
		SubtitlesEnabledByDefault: subtitlesEnabled,
		WatcherEnabled:            watcherEnabled,
		WatcherMode:               watcherMode,
		ScanIntervalMinutes:       scanIntervalMinutes,
	})
}

func retryCreateLibraryInsert(
	dbConn *sql.DB,
	userID int,
	payload createLibraryRequest,
	defaultAudio string,
	defaultSubtitle string,
	subtitlesEnabled bool,
	watcherEnabled bool,
	watcherMode string,
	scanIntervalMinutes int,
	now time.Time,
	libID *int,
) error {
	var err error
	for attempt := 0; attempt < 4; attempt++ {
		err = dbConn.QueryRow(
			`INSERT INTO libraries (
				user_id, name, type, path, preferred_audio_language, preferred_subtitle_language,
				subtitles_enabled_by_default, watcher_enabled, watcher_mode, scan_interval_minutes, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			userID, payload.Name, payload.Type, payload.Path, defaultAudio, defaultSubtitle, subtitlesEnabled, watcherEnabled, watcherMode, scanIntervalMinutes, now,
		).Scan(libID)
		if isMissingColumnError(err, "preferred_audio_language") {
			err = dbConn.QueryRow(
				`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
				userID, payload.Name, payload.Type, payload.Path, now,
			).Scan(libID)
		} else if isMissingColumnError(err, "watcher_enabled") {
			err = dbConn.QueryRow(
				`INSERT INTO libraries (
					user_id, name, type, path, preferred_audio_language, preferred_subtitle_language,
					subtitles_enabled_by_default, created_at
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
				userID, payload.Name, payload.Type, payload.Path, defaultAudio, defaultSubtitle, subtitlesEnabled, now,
			).Scan(libID)
		}
		if err == nil || !isSQLiteBusyError(err) {
			return err
		}
		time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
	}
	return err
}

func (h *LibraryHandler) ListLibraries(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	rows, err := h.DB.Query(
		`SELECT id, name, type, path, user_id, preferred_audio_language, preferred_subtitle_language,
		        subtitles_enabled_by_default, watcher_enabled, watcher_mode, scan_interval_minutes
		   FROM libraries WHERE user_id = ? ORDER BY name COLLATE NOCASE, id`,
		u.ID,
	)
	legacyColumns := false
	legacyAutomationColumns := false
	if err != nil && isMissingColumnError(err, "preferred_audio_language") {
		legacyColumns = true
		rows, err = h.DB.Query(
			`SELECT id, name, type, path, user_id FROM libraries WHERE user_id = ? ORDER BY name COLLATE NOCASE, id`,
			u.ID,
		)
	} else if err != nil && isMissingColumnError(err, "watcher_enabled") {
		legacyAutomationColumns = true
		rows, err = h.DB.Query(
			`SELECT id, name, type, path, user_id, preferred_audio_language, preferred_subtitle_language, subtitles_enabled_by_default
			   FROM libraries WHERE user_id = ? ORDER BY name COLLATE NOCASE, id`,
			u.ID,
		)
	}
	if err != nil {
		slog.Error("list libraries", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var libs []libraryResponse
	for rows.Next() {
		var (
			id                int
			name              string
			libraryType       string
			path              string
			userID            int
			preferredAudio    sql.NullString
			preferredSubtitle sql.NullString
			subtitlesEnabled  sql.NullBool
			watcherEnabled    sql.NullBool
			watcherMode       sql.NullString
			scanInterval      sql.NullInt64
		)
		if legacyColumns {
			err = rows.Scan(&id, &name, &libraryType, &path, &userID)
		} else if legacyAutomationColumns {
			err = rows.Scan(
				&id,
				&name,
				&libraryType,
				&path,
				&userID,
				&preferredAudio,
				&preferredSubtitle,
				&subtitlesEnabled,
			)
		} else {
			err = rows.Scan(
				&id,
				&name,
				&libraryType,
				&path,
				&userID,
				&preferredAudio,
				&preferredSubtitle,
				&subtitlesEnabled,
				&watcherEnabled,
				&watcherMode,
				&scanInterval,
			)
		}
		if err != nil {
			slog.Error("scan libraries row", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		libs = append(libs, buildLibraryResponse(
			id,
			name,
			libraryType,
			path,
			userID,
			preferredAudio,
			preferredSubtitle,
			subtitlesEnabled,
			watcherEnabled,
			watcherMode,
			scanInterval,
		))
	}

	writeJSON(w, http.StatusOK, libs)
}

func (h *LibraryHandler) ListUnidentifiedLibrarySummaries(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	summaries, err := db.ListUnidentifiedLibrarySummariesForUser(h.DB, u.ID)
	if err != nil {
		slog.Error("list unidentified library summaries", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"libraries": summaries})
}

func (h *LibraryHandler) UpdateLibraryPlaybackPreferences(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := chi.URLParam(r, "id")
	var payload updateLibraryPlaybackPreferencesRequest
	if !decodeRequestJSON(w, r, &payload) {
		return
	}

	payload.PreferredAudioLanguage = strings.TrimSpace(strings.ToLower(payload.PreferredAudioLanguage))
	payload.PreferredSubtitleLanguage = strings.TrimSpace(strings.ToLower(payload.PreferredSubtitleLanguage))

	var (
		libraryID             int
		ownerID               int
		name                  string
		libraryType           string
		path                  string
		currentWatcherEnabled sql.NullBool
		currentWatcherMode    sql.NullString
		currentScanInterval   sql.NullInt64
	)
	err := h.DB.QueryRow(
		`SELECT id, user_id, name, type, path, watcher_enabled, watcher_mode, scan_interval_minutes FROM libraries WHERE id = ?`,
		idStr,
	).Scan(&libraryID, &ownerID, &name, &libraryType, &path, &currentWatcherEnabled, &currentWatcherMode, &currentScanInterval)
	legacyAutomationColumns := false
	if err != nil && isMissingColumnError(err, "watcher_enabled") {
		legacyAutomationColumns = true
		err = h.DB.QueryRow(
			`SELECT id, user_id, name, type, path FROM libraries WHERE id = ?`,
			idStr,
		).Scan(&libraryID, &ownerID, &name, &libraryType, &path)
	}
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if ownerID != u.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if legacyAutomationColumns {
		currentWatcherEnabled = sql.NullBool{}
		currentWatcherMode = sql.NullString{}
		currentScanInterval = sql.NullInt64{}
	}
	defaultWatcherEnabled, defaultWatcherMode, defaultScanInterval := defaultLibraryAutomation()
	watcherEnabled, watcherMode, scanIntervalMinutes := normalizeLibraryAutomationWithDefaults(
		coalesceNullableBool(currentWatcherEnabled, defaultWatcherEnabled),
		coalesceNullableString(currentWatcherMode, defaultWatcherMode),
		coalesceNullableInt(currentScanInterval, defaultScanInterval),
		payload.WatcherEnabled,
		payload.WatcherMode,
		payload.ScanIntervalMinutes,
	)

	if _, err := h.DB.Exec(
		`UPDATE libraries
		    SET preferred_audio_language = ?, preferred_subtitle_language = ?, subtitles_enabled_by_default = ?,
		        watcher_enabled = ?, watcher_mode = ?, scan_interval_minutes = ?
		  WHERE id = ?`,
		payload.PreferredAudioLanguage,
		payload.PreferredSubtitleLanguage,
		payload.SubtitlesEnabledByDefault,
		watcherEnabled,
		watcherMode,
		scanIntervalMinutes,
		libraryID,
	); err != nil {
		if isMissingColumnError(err, "watcher_enabled") {
			if _, err := h.DB.Exec(
				`UPDATE libraries SET preferred_audio_language = ?, preferred_subtitle_language = ?, subtitles_enabled_by_default = ? WHERE id = ?`,
				payload.PreferredAudioLanguage,
				payload.PreferredSubtitleLanguage,
				payload.SubtitlesEnabledByDefault,
				libraryID,
			); err == nil {
				goto encodeLibraryResponse
			}
		}
		if !isMissingColumnError(err, "preferred_audio_language") {
			slog.Error("update library playback preferences", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	if h.ScanJobs != nil {
		h.ScanJobs.ConfigureLibraryAutomation(libraryID, path, libraryType, watcherEnabled, watcherMode, scanIntervalMinutes)
	}

encodeLibraryResponse:
	writeJSON(w, http.StatusOK, libraryResponse{
		ID:                        libraryID,
		Name:                      name,
		Type:                      libraryType,
		Path:                      path,
		UserID:                    ownerID,
		PreferredAudioLanguage:    payload.PreferredAudioLanguage,
		PreferredSubtitleLanguage: payload.PreferredSubtitleLanguage,
		SubtitlesEnabledByDefault: payload.SubtitlesEnabledByDefault,
		WatcherEnabled:            watcherEnabled,
		WatcherMode:               watcherMode,
		ScanIntervalMinutes:       scanIntervalMinutes,
	})
}

func (h *LibraryHandler) ScanLibrary(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	libraryID, ownerID, path, typ, ok := h.authorizeLibraryRequest(w, r, u.ID)
	if !ok {
		return
	}
	if ownerID != u.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	identify := r.URL.Query().Get("identify") != "false"
	subpaths, err := requestedScanSubpaths(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var id metadata.Identifier
	if identify {
		id = h.Meta
	}

	var musicIdentifier metadata.MusicIdentifier
	if detected, ok := id.(metadata.MusicIdentifier); ok {
		musicIdentifier = detected
	}
	added, err := db.HandleScanLibraryWithOptions(r.Context(), h.DB, path, typ, libraryID, db.ScanOptions{
		Identifier:             id,
		MusicIdentifier:        musicIdentifier,
		ProbeMedia:             true,
		ProbeEmbeddedSubtitles: true,
		ScanSidecarSubtitles:   true,
		Subpaths:               subpaths,
	})
	if err != nil {
		http.Error(w, "scan error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, added)
}

func (h *LibraryHandler) StartLibraryScan(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.ScanJobs == nil {
		http.Error(w, "scan queue unavailable", http.StatusServiceUnavailable)
		return
	}

	libraryID, ownerID, path, typ, ok := h.authorizeLibraryRequest(w, r, u.ID)
	if !ok {
		return
	}
	if ownerID != u.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	subpaths, err := requestedScanSubpaths(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	status := h.ScanJobs.start(libraryID, path, typ, r.URL.Query().Get("identify") != "false", subpaths)
	writeJSON(w, http.StatusOK, status)
}

func requestedScanSubpaths(r *http.Request) ([]string, error) {
	subpath := strings.TrimSpace(r.URL.Query().Get("subpath"))
	if subpath == "" {
		return nil, nil
	}
	return db.NormalizeScanSubpaths([]string{subpath})
}

func (h *LibraryHandler) GetLibraryScanStatus(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.ScanJobs == nil {
		http.Error(w, "scan queue unavailable", http.StatusServiceUnavailable)
		return
	}

	libraryID, _, _, _, ok := h.authorizeLibraryRequest(w, r, u.ID)
	if !ok {
		return
	}

	writeJSON(w, http.StatusOK, h.ScanJobs.status(libraryID))
}

func (h *LibraryHandler) authorizeLibraryRequest(
	w http.ResponseWriter,
	r *http.Request,
	userID int,
) (libraryID int, ownerID int, path string, typ string, ok bool) {
	idStr := chi.URLParam(r, "id")
	err := h.DB.QueryRow(
		`SELECT id, user_id, path, type FROM libraries WHERE id = ?`,
		idStr,
	).Scan(&libraryID, &ownerID, &path, &typ)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return 0, 0, "", "", false
	}
	allowed, accessErr := db.UserHasLibraryAccess(h.DB, userID, libraryID)
	if accessErr != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return 0, 0, "", "", false
	}
	if !allowed {
		http.Error(w, "forbidden", http.StatusForbidden)
		return 0, 0, "", "", false
	}
	return libraryID, ownerID, path, typ, true
}
