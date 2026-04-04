package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"plum/internal/arr"
	"plum/internal/db"
	"plum/internal/metadata"
)

type LibraryHandler struct {
	DB          *sql.DB
	Meta        metadata.Identifier
	Artwork     metadata.MetadataArtworkProvider
	Movies      metadata.MovieDetailsProvider
	MovieQuery  metadata.MovieProvider
	MovieLookup metadata.MovieLookupProvider
	Series      metadata.SeriesDetailsProvider
	SeriesQuery metadata.SeriesSearchProvider
	Discover    metadata.DiscoverProvider
	Arr         *arr.Service
	ScanJobs    *LibraryScanManager
	SearchIndex *SearchIndexManager
	identifyRun *identifyRunTracker

	playbackRefreshMu      sync.Mutex
	playbackRefreshRunning map[int]struct{}
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
	IntroSkipMode             string `json:"intro_skip_mode,omitempty"`
}

type libraryBrowseItemResponse struct {
	ID                   int     `json:"id"`
	LibraryID            int     `json:"library_id,omitempty"`
	Title                string  `json:"title"`
	Path                 string  `json:"path"`
	Duration             int     `json:"duration"`
	Type                 string  `json:"type"`
	MatchStatus          string  `json:"match_status,omitempty"`
	IdentifyState        string  `json:"identify_state,omitempty"`
	TMDBID               int     `json:"tmdb_id,omitempty"`
	TVDBID               string  `json:"tvdb_id,omitempty"`
	Overview             string  `json:"overview,omitempty"`
	PosterPath           string  `json:"poster_path,omitempty"`
	BackdropPath         string  `json:"backdrop_path,omitempty"`
	PosterURL            string  `json:"poster_url,omitempty"`
	BackdropURL          string  `json:"backdrop_url,omitempty"`
	ShowPosterPath       string  `json:"show_poster_path,omitempty"`
	ShowPosterURL        string  `json:"show_poster_url,omitempty"`
	ReleaseDate          string  `json:"release_date,omitempty"`
	ShowVoteAverage      float64 `json:"show_vote_average,omitempty"`
	ShowIMDbRating       float64 `json:"show_imdb_rating,omitempty"`
	VoteAverage          float64 `json:"vote_average,omitempty"`
	IMDbID               string  `json:"imdb_id,omitempty"`
	IMDbRating           float64 `json:"imdb_rating,omitempty"`
	Artist               string  `json:"artist,omitempty"`
	Album                string  `json:"album,omitempty"`
	AlbumArtist          string  `json:"album_artist,omitempty"`
	DiscNumber           int     `json:"disc_number,omitempty"`
	TrackNumber          int     `json:"track_number,omitempty"`
	ReleaseYear          int     `json:"release_year,omitempty"`
	ProgressSeconds      float64 `json:"progress_seconds,omitempty"`
	ProgressPercent      float64 `json:"progress_percent,omitempty"`
	RemainingSeconds     float64 `json:"remaining_seconds,omitempty"`
	Completed            bool    `json:"completed,omitempty"`
	LastWatchedAt        string  `json:"last_watched_at,omitempty"`
	Season               int     `json:"season,omitempty"`
	Episode              int     `json:"episode,omitempty"`
	MetadataReviewNeeded bool    `json:"metadata_review_needed,omitempty"`
	MetadataConfirmed    bool    `json:"metadata_confirmed,omitempty"`
	ThumbnailPath        string  `json:"thumbnail_path,omitempty"`
	ThumbnailURL         string  `json:"thumbnail_url,omitempty"`
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
	introSkip sql.NullString,
) libraryResponse {
	defaultAudio, defaultSubtitle, defaultSubtitlesEnabled := defaultLibraryPlaybackPreferences(libraryType)
	defaultWatcherEnabled, defaultWatcherMode, defaultScanIntervalMinutes := defaultLibraryAutomation()
	introMode := db.IntroSkipModeManual
	if introSkip.Valid && strings.TrimSpace(introSkip.String) != "" {
		introMode = db.NormalizeIntroSkipMode(introSkip.String)
	}
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
		IntroSkipMode:             introMode,
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
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "database is locked") || strings.Contains(text, "sqlite_busy")
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
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
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
		log.Printf("create library: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if h.ScanJobs != nil {
		h.ScanJobs.ConfigureLibraryAutomation(libID, payload.Path, payload.Type, watcherEnabled, watcherMode, scanIntervalMinutes)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(libraryResponse{
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
		IntroSkipMode:             db.IntroSkipModeManual,
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
				subtitles_enabled_by_default, watcher_enabled, watcher_mode, scan_interval_minutes, intro_skip_mode, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			userID, payload.Name, payload.Type, payload.Path, defaultAudio, defaultSubtitle, subtitlesEnabled, watcherEnabled, watcherMode, scanIntervalMinutes, db.IntroSkipModeManual, now,
		).Scan(libID)
		if isMissingColumnError(err, "intro_skip_mode") {
			err = dbConn.QueryRow(
				`INSERT INTO libraries (
					user_id, name, type, path, preferred_audio_language, preferred_subtitle_language,
					subtitles_enabled_by_default, watcher_enabled, watcher_mode, scan_interval_minutes, created_at
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
				userID, payload.Name, payload.Type, payload.Path, defaultAudio, defaultSubtitle, subtitlesEnabled, watcherEnabled, watcherMode, scanIntervalMinutes, now,
			).Scan(libID)
		} else if isMissingColumnError(err, "preferred_audio_language") {
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
		        subtitles_enabled_by_default, watcher_enabled, watcher_mode, scan_interval_minutes, intro_skip_mode
		   FROM libraries WHERE user_id = ? ORDER BY id`,
		u.ID,
	)
	legacyColumns := false
	legacyAutomationColumns := false
	if err != nil && isMissingColumnError(err, "intro_skip_mode") {
		rows, err = h.DB.Query(
			`SELECT id, name, type, path, user_id, preferred_audio_language, preferred_subtitle_language,
			        subtitles_enabled_by_default, watcher_enabled, watcher_mode, scan_interval_minutes
			   FROM libraries WHERE user_id = ? ORDER BY id`,
			u.ID,
		)
	}
	if err != nil && isMissingColumnError(err, "preferred_audio_language") {
		legacyColumns = true
		rows, err = h.DB.Query(
			`SELECT id, name, type, path, user_id FROM libraries WHERE user_id = ? ORDER BY id`,
			u.ID,
		)
	} else if err != nil && isMissingColumnError(err, "watcher_enabled") {
		legacyAutomationColumns = true
		rows, err = h.DB.Query(
			`SELECT id, name, type, path, user_id, preferred_audio_language, preferred_subtitle_language, subtitles_enabled_by_default
			   FROM libraries WHERE user_id = ? ORDER BY id`,
			u.ID,
		)
	}
	if err != nil {
		log.Printf("list libraries: %v", err)
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
			introSkip         sql.NullString
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
				&introSkip,
			)
		}
		if err != nil {
			log.Printf("scan libraries row: %v", err)
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
			introSkip,
		))
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(libs)
}

func (h *LibraryHandler) UpdateLibraryPlaybackPreferences(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := chi.URLParam(r, "id")
	var payload updateLibraryPlaybackPreferencesRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
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
		currentIntro          sql.NullString
	)
	err := h.DB.QueryRow(
		`SELECT id, user_id, name, type, path, watcher_enabled, watcher_mode, scan_interval_minutes, intro_skip_mode FROM libraries WHERE id = ?`,
		idStr,
	).Scan(&libraryID, &ownerID, &name, &libraryType, &path, &currentWatcherEnabled, &currentWatcherMode, &currentScanInterval, &currentIntro)
	if err != nil && isMissingColumnError(err, "intro_skip_mode") {
		err = h.DB.QueryRow(
			`SELECT id, user_id, name, type, path, watcher_enabled, watcher_mode, scan_interval_minutes FROM libraries WHERE id = ?`,
			idStr,
		).Scan(&libraryID, &ownerID, &name, &libraryType, &path, &currentWatcherEnabled, &currentWatcherMode, &currentScanInterval)
		currentIntro = sql.NullString{}
	}
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

	introMode := db.NormalizeIntroSkipMode(payload.IntroSkipMode)
	if strings.TrimSpace(payload.IntroSkipMode) == "" && currentIntro.Valid {
		introMode = db.NormalizeIntroSkipMode(currentIntro.String)
	}

	if _, err := h.DB.Exec(
		`UPDATE libraries
		    SET preferred_audio_language = ?, preferred_subtitle_language = ?, subtitles_enabled_by_default = ?,
		        watcher_enabled = ?, watcher_mode = ?, scan_interval_minutes = ?, intro_skip_mode = ?
		  WHERE id = ?`,
		payload.PreferredAudioLanguage,
		payload.PreferredSubtitleLanguage,
		payload.SubtitlesEnabledByDefault,
		watcherEnabled,
		watcherMode,
		scanIntervalMinutes,
		introMode,
		libraryID,
	); err != nil {
		if isMissingColumnError(err, "intro_skip_mode") {
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
			); err == nil {
				goto encodeLibraryResponse
			}
		}
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
			log.Printf("update library playback preferences: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	if h.ScanJobs != nil {
		h.ScanJobs.ConfigureLibraryAutomation(libraryID, path, libraryType, watcherEnabled, watcherMode, scanIntervalMinutes)
	}

encodeLibraryResponse:
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(libraryResponse{
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
		IntroSkipMode:             introMode,
	})
}

type scanResult = db.ScanResult

type identifyResult struct {
	Identified int `json:"identified"`
	Failed     int `json:"failed"`
}

var (
	identifyInitialTimeout   = 8 * time.Second
	identifyRetryTimeout     = 45 * time.Second
	identifyMovieWorkers     = 2
	identifyMovieRateLimit   = 250 * time.Millisecond
	identifyMovieRateBurst   = 2
	identifyEpisodeWorkers   = 4
	identifyEpisodeRateLimit = 100 * time.Millisecond
	identifyEpisodeRateBurst = 4
)

type identifyJob struct {
	row     db.IdentificationRow
	attempt int
}

type identifyJobStatus string

const (
	identifyJobSucceeded identifyJobStatus = "succeeded"
	identifyJobRetry     identifyJobStatus = "retry"
	identifyJobFailed    identifyJobStatus = "failed"
)

type identifyJobResult struct {
	status           identifyJobStatus
	job              identifyJob
	fallbackEligible bool
}

type identifyRunConfig struct {
	workers      int
	rateInterval time.Duration
	rateBurst    int
}

type episodeIdentifyGroup struct {
	key             string
	kind            string
	groupQuery      string
	fallbackQueries []string
	explicitTMDBID  int
	explicitTVDBID  string
	attempt         int
	representative  db.EpisodeIdentifyRow
	rows            []db.EpisodeIdentifyRow
}

type episodeGroupJob struct {
	group   episodeIdentifyGroup
	attempt int
}

type episodeGroupResult struct {
	group      episodeIdentifyGroup
	identified int
	retry      bool
	failed     []identifyJobResult
}

type episodeSearchCache struct {
	mu    sync.Mutex
	calls map[string]*episodeSearchCall
}

type episodeSearchCall struct {
	done    chan struct{}
	results []metadata.MatchResult
	err     error
}

type episodeSeriesDetailsCache struct {
	mu    sync.Mutex
	calls map[int]*episodeSeriesDetailsCall
}

type episodeSeriesDetailsCall struct {
	done    chan struct{}
	details *metadata.SeriesDetails
	err     error
}

type episodeLookupCache struct {
	mu    sync.Mutex
	calls map[string]*episodeLookupCall
}

type episodeLookupCall struct {
	done  chan struct{}
	match *metadata.MatchResult
	err   error
}

type episodicIdentifyCache struct {
	handler      *LibraryHandler
	searchCache  *episodeSearchCache
	detailsCache *episodeSeriesDetailsCache
	episodeCache *episodeLookupCache
}

type movieIdentifyCall struct {
	done chan struct{}
	res  *metadata.MatchResult
	err  error
}

type movieIdentifyCache struct {
	mu    sync.Mutex
	calls map[string]*movieIdentifyCall
}

func newMovieIdentifyCache() *movieIdentifyCache {
	return &movieIdentifyCache{calls: make(map[string]*movieIdentifyCall)}
}

func (c *movieIdentifyCache) lookupOrRun(key string, fn func() (*metadata.MatchResult, error)) (*metadata.MatchResult, error) {
	if c == nil || key == "" {
		return fn()
	}

	c.mu.Lock()
	if call, ok := c.calls[key]; ok {
		c.mu.Unlock()
		<-call.done
		return call.res, call.err
	}
	call := &movieIdentifyCall{done: make(chan struct{})}
	c.calls[key] = call
	c.mu.Unlock()

	call.res, call.err = fn()
	close(call.done)

	c.mu.Lock()
	if call.res == nil || call.err != nil {
		delete(c.calls, key)
	}
	c.mu.Unlock()
	return call.res, call.err
}

func newEpisodicIdentifyCache(handler *LibraryHandler) *episodicIdentifyCache {
	return &episodicIdentifyCache{
		handler: handler,
		searchCache: &episodeSearchCache{
			calls: make(map[string]*episodeSearchCall),
		},
		detailsCache: &episodeSeriesDetailsCache{
			calls: make(map[int]*episodeSeriesDetailsCall),
		},
		episodeCache: &episodeLookupCache{
			calls: make(map[string]*episodeLookupCall),
		},
	}
}

func (c *episodeSearchCache) lookupOrRun(key string, fn func() ([]metadata.MatchResult, error)) ([]metadata.MatchResult, error) {
	if c == nil || key == "" {
		return fn()
	}
	c.mu.Lock()
	if call, ok := c.calls[key]; ok {
		c.mu.Unlock()
		<-call.done
		return append([]metadata.MatchResult(nil), call.results...), call.err
	}
	call := &episodeSearchCall{done: make(chan struct{})}
	c.calls[key] = call
	c.mu.Unlock()

	call.results, call.err = fn()
	close(call.done)

	c.mu.Lock()
	if call.err != nil || len(call.results) == 0 {
		delete(c.calls, key)
	}
	c.mu.Unlock()

	return append([]metadata.MatchResult(nil), call.results...), call.err
}

func (c *episodeSeriesDetailsCache) lookupOrRun(key int, fn func() (*metadata.SeriesDetails, error)) (*metadata.SeriesDetails, error) {
	if c == nil || key <= 0 {
		return fn()
	}
	c.mu.Lock()
	if call, ok := c.calls[key]; ok {
		c.mu.Unlock()
		<-call.done
		return call.details, call.err
	}
	call := &episodeSeriesDetailsCall{done: make(chan struct{})}
	c.calls[key] = call
	c.mu.Unlock()

	call.details, call.err = fn()
	close(call.done)

	c.mu.Lock()
	if call.err != nil || call.details == nil {
		delete(c.calls, key)
	}
	c.mu.Unlock()

	return call.details, call.err
}

func (c *episodeLookupCache) lookupOrRun(key string, fn func() (*metadata.MatchResult, error)) (*metadata.MatchResult, error) {
	if c == nil || key == "" {
		return fn()
	}
	c.mu.Lock()
	if call, ok := c.calls[key]; ok {
		c.mu.Unlock()
		<-call.done
		return call.match, call.err
	}
	call := &episodeLookupCall{done: make(chan struct{})}
	c.calls[key] = call
	c.mu.Unlock()

	call.match, call.err = fn()
	close(call.done)

	c.mu.Lock()
	if call.err != nil || call.match == nil {
		delete(c.calls, key)
	}
	c.mu.Unlock()

	return call.match, call.err
}

func (c *episodicIdentifyCache) SearchTV(ctx context.Context, query string) ([]metadata.MatchResult, error) {
	if c == nil || c.handler == nil || c.handler.SeriesQuery == nil {
		return nil, nil
	}
	return c.searchCache.lookupOrRun(strings.ToLower(strings.TrimSpace(query)), func() ([]metadata.MatchResult, error) {
		return c.handler.SeriesQuery.SearchTV(ctx, query)
	})
}

func (c *episodicIdentifyCache) GetSeriesDetails(ctx context.Context, tmdbID int) (*metadata.SeriesDetails, error) {
	if c == nil || c.handler == nil || c.handler.Series == nil {
		return nil, nil
	}
	return c.detailsCache.lookupOrRun(tmdbID, func() (*metadata.SeriesDetails, error) {
		return c.handler.Series.GetSeriesDetails(ctx, tmdbID)
	})
}

func (c *episodicIdentifyCache) GetEpisode(ctx context.Context, provider, seriesID string, season, episode int) (*metadata.MatchResult, error) {
	if c == nil || c.handler == nil || c.handler.SeriesQuery == nil {
		return nil, nil
	}
	key := provider + ":" + seriesID + ":" + strconv.Itoa(season) + ":" + strconv.Itoa(episode)
	return c.episodeCache.lookupOrRun(key, func() (*metadata.MatchResult, error) {
		return c.handler.SeriesQuery.GetEpisode(ctx, provider, seriesID, season, episode)
	})
}

func identifyConfigForKind(kind string) identifyRunConfig {
	if kind == db.LibraryTypeMovie {
		return identifyRunConfig{
			workers:      identifyMovieWorkers,
			rateInterval: identifyMovieRateLimit,
			rateBurst:    identifyMovieRateBurst,
		}
	}
	return identifyRunConfig{
		workers:      identifyEpisodeWorkers,
		rateInterval: identifyEpisodeRateLimit,
		rateBurst:    identifyEpisodeRateBurst,
	}
}

func identificationRowsFromEpisodeRows(rows []db.EpisodeIdentifyRow) []db.IdentificationRow {
	out := make([]db.IdentificationRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.IdentificationRow)
	}
	return out
}

func (h *LibraryHandler) IdentifyLibrary(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	idStr := chi.URLParam(r, "id")
	var libraryID, ownerID int
	err := h.DB.QueryRow(`SELECT id, user_id FROM libraries WHERE id = ?`, idStr).Scan(&libraryID, &ownerID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if ownerID != u.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	result, err := h.identifyLibrary(r.Context(), libraryID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func (h *LibraryHandler) identifyLibrary(ctx context.Context, libraryID int) (identifyResult, error) {
	if h.Meta == nil {
		return identifyResult{Identified: 0, Failed: 0}, nil
	}
	var libraryPath string
	_ = h.DB.QueryRow(`SELECT path FROM libraries WHERE id = ?`, libraryID).Scan(&libraryPath)
	var libraryType string
	if err := h.DB.QueryRow(`SELECT type FROM libraries WHERE id = ?`, libraryID).Scan(&libraryType); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.identifyRun.clearLibrary(libraryID)
			return identifyResult{Identified: 0, Failed: 0}, nil
		}
		return identifyResult{}, err
	}

	if libraryType == db.LibraryTypeTV || libraryType == db.LibraryTypeAnime {
		rows, err := db.ListEpisodeIdentifyRowsByLibrary(h.DB, libraryID)
		if err != nil {
			return identifyResult{}, err
		}
		if len(rows) == 0 {
			h.identifyRun.clearLibrary(libraryID)
			return identifyResult{Identified: 0, Failed: 0}, nil
		}
		trackedRows, refreshOnlyRows := splitEpisodeIdentifyRows(rows)
		if h.identifyRun == nil {
			h.identifyRun = newIdentifyRunTracker()
		}
		if len(trackedRows) > 0 {
			h.identifyRun.startLibrary(libraryID, identificationRowsFromEpisodeRows(trackedRows))
			defer h.identifyRun.finishLibrary(libraryID)
		} else {
			h.identifyRun.clearLibrary(libraryID)
		}
		log.Printf(
			"identify library=%d type=%s tracked_rows=%d refresh_rows=%d",
			libraryID,
			libraryType,
			len(trackedRows),
			len(refreshOnlyRows),
		)
		identified, failed, err := h.identifyEpisodeRowsWithRefresh(ctx, libraryID, libraryPath, libraryType, trackedRows, refreshOnlyRows)
		if err != nil {
			return identifyResult{}, err
		}
		if identified > 0 && h.SearchIndex != nil {
			h.SearchIndex.Queue(libraryID, false)
		}
		return identifyResult{Identified: identified, Failed: failed}, nil
	}

	rows, err := db.ListIdentifiableByLibrary(h.DB, libraryID)
	if err != nil {
		return identifyResult{}, err
	}
	if len(rows) == 0 {
		h.identifyRun.clearLibrary(libraryID)
		return identifyResult{Identified: 0, Failed: 0}, nil
	}
	trackedRows, refreshOnlyRows := splitIdentifyRows(rows)
	if h.identifyRun == nil {
		h.identifyRun = newIdentifyRunTracker()
	}
	if len(trackedRows) > 0 {
		h.identifyRun.startLibrary(libraryID, trackedRows)
		defer h.identifyRun.finishLibrary(libraryID)
	} else {
		h.identifyRun.clearLibrary(libraryID)
	}
	log.Printf(
		"identify library=%d type=%s tracked_rows=%d refresh_rows=%d",
		libraryID,
		libraryType,
		len(trackedRows),
		len(refreshOnlyRows),
	)

	identified, failed := 0, 0
	initialJobs := make([]identifyJob, 0, len(trackedRows))
	for _, row := range trackedRows {
		initialJobs = append(initialJobs, identifyJob{row: row})
	}
	sortIdentifyJobs(initialJobs, libraryPath)
	var movieCache *movieIdentifyCache
	if len(rows) > 0 && rows[0].Kind == db.LibraryTypeMovie {
		movieCache = newMovieIdentifyCache()
	}
	initialIdentified, retryJobs, initialFailed := h.runIdentifyJobs(ctx, libraryID, libraryPath, initialJobs, movieCache)
	retryIdentified, _, retryFailed := h.runIdentifyJobs(ctx, libraryID, libraryPath, retryJobs, movieCache)
	identified += initialIdentified + retryIdentified

	fallbackIdentified, fallbackFailed := h.identifyShowFallbacks(ctx, libraryID, libraryPath, append(initialFailed, retryFailed...), nil, false)
	identified += fallbackIdentified
	failed += fallbackFailed
	if len(refreshOnlyRows) > 0 {
		refreshIdentified, refreshErr := h.refreshMatchedRows(ctx, libraryID, libraryPath, refreshOnlyRows, movieCache)
		if refreshErr != nil {
			log.Printf("identify refresh-only rows failed library=%d type=%s error=%v", libraryID, libraryType, refreshErr)
		}
		identified += refreshIdentified
	}

	if identified > 0 && h.SearchIndex != nil {
		h.SearchIndex.Queue(libraryID, false)
	}
	return identifyResult{Identified: identified, Failed: failed}, nil
}

func hasProviderMatch(row db.IdentificationRow) bool {
	return row.TMDBID > 0 || strings.TrimSpace(row.TVDBID) != ""
}

func rowNeedsTrackedIdentify(row db.IdentificationRow) bool {
	return row.MatchStatus != db.MatchStatusIdentified || !hasProviderMatch(row)
}

func splitIdentifyRows(rows []db.IdentificationRow) (tracked []db.IdentificationRow, refreshOnly []db.IdentificationRow) {
	tracked = make([]db.IdentificationRow, 0, len(rows))
	refreshOnly = make([]db.IdentificationRow, 0, len(rows))
	for _, row := range rows {
		if rowNeedsTrackedIdentify(row) {
			tracked = append(tracked, row)
			continue
		}
		refreshOnly = append(refreshOnly, row)
	}
	return tracked, refreshOnly
}

func splitEpisodeIdentifyRows(rows []db.EpisodeIdentifyRow) (tracked []db.EpisodeIdentifyRow, refreshOnly []db.EpisodeIdentifyRow) {
	tracked = make([]db.EpisodeIdentifyRow, 0, len(rows))
	refreshOnly = make([]db.EpisodeIdentifyRow, 0, len(rows))
	for _, row := range rows {
		if rowNeedsTrackedIdentify(row.IdentificationRow) {
			tracked = append(tracked, row)
			continue
		}
		refreshOnly = append(refreshOnly, row)
	}
	return tracked, refreshOnly
}

func (h *LibraryHandler) refreshMatchedRows(
	ctx context.Context,
	libraryID int,
	libraryPath string,
	rows []db.IdentificationRow,
	movieCache *movieIdentifyCache,
) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	jobs := make([]identifyJob, 0, len(rows))
	for _, row := range rows {
		jobs = append(jobs, identifyJob{row: row})
	}
	sortIdentifyJobs(jobs, libraryPath)
	identified, retryJobs, initialFailed := h.runIdentifyJobs(ctx, libraryID, libraryPath, jobs, movieCache)
	retryIdentified, _, retryFailed := h.runIdentifyJobs(ctx, libraryID, libraryPath, retryJobs, movieCache)
	identified += retryIdentified
	refreshIdentified, _ := h.identifyShowFallbacks(ctx, libraryID, libraryPath, append(initialFailed, retryFailed...), nil, false)
	return identified + refreshIdentified, nil
}

func (h *LibraryHandler) identifyEpisodeRowsWithRefresh(
	ctx context.Context,
	libraryID int,
	libraryPath string,
	libraryType string,
	trackedRows []db.EpisodeIdentifyRow,
	refreshOnlyRows []db.EpisodeIdentifyRow,
) (identified int, failed int, err error) {
	if len(trackedRows) > 0 {
		trackedResult, trackErr := h.identifyEpisodesByGroup(ctx, libraryID, libraryPath, trackedRows)
		if trackErr != nil {
			return 0, 0, trackErr
		}
		identified += trackedResult.Identified
		failed += trackedResult.Failed
	}
	if len(refreshOnlyRows) > 0 {
		refreshResult, refreshErr := h.identifyEpisodesByGroup(ctx, libraryID, libraryPath, refreshOnlyRows)
		if refreshErr != nil {
			log.Printf("identify refresh-only episode rows failed library=%d type=%s error=%v", libraryID, libraryType, refreshErr)
		} else {
			identified += refreshResult.Identified
		}
	}
	return identified, failed, nil
}

func (h *LibraryHandler) runIdentifyJobs(
	ctx context.Context,
	libraryID int,
	libraryPath string,
	jobsToRun []identifyJob,
	movieCache *movieIdentifyCache,
) (identified int, retryJobs []identifyJob, failed []identifyJobResult) {
	if len(jobsToRun) == 0 {
		return 0, nil, nil
	}

	results := make(chan identifyJobResult, len(jobsToRun))
	jobs := make(chan identifyJob)
	config := identifyConfigForKind(jobsToRun[0].row.Kind)
	workerCount := config.workers
	if workerCount > len(jobsToRun) {
		workerCount = len(jobsToRun)
	}
	if workerCount < 1 {
		workerCount = 1
	}
	rateLimiter := newIdentifyRateLimiter(ctx, config.rateInterval, config.rateBurst)

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case job, ok := <-jobs:
					if !ok {
						return
					}
					results <- h.identifyLibraryJob(ctx, libraryID, job, libraryPath, rateLimiter, movieCache)
				}
			}
		}()
	}

	go func() {
		defer close(results)
		wg.Wait()
	}()

enqueueLoop:
	for _, job := range jobsToRun {
		select {
		case <-ctx.Done():
			break enqueueLoop
		case jobs <- job:
		}
	}
	close(jobs)

	for result := range results {
		switch result.status {
		case identifyJobSucceeded:
			identified++
		case identifyJobRetry:
			retryJobs = append(retryJobs, identifyJob{
				row:     result.job.row,
				attempt: result.job.attempt + 1,
			})
		case identifyJobFailed:
			failed = append(failed, result)
		}
	}

	return identified, retryJobs, failed
}

type showFallbackGroup struct {
	queries []string
	rows    []db.IdentificationRow
}

func (h *LibraryHandler) identifyShowFallbacks(
	ctx context.Context,
	libraryID int,
	libraryPath string,
	failedResults []identifyJobResult,
	cache *episodicIdentifyCache,
	queueSearch bool,
) (identified int, failed int) {
	if len(failedResults) == 0 {
		return 0, 0
	}

	groups := make(map[string]*showFallbackGroup)
	for _, result := range failedResults {
		if !result.fallbackEligible {
			h.identifyRun.setState(libraryID, result.job.row.Kind, result.job.row.Path, "failed")
			failed++
			continue
		}
		queries := showFallbackQueries(result.job.row, libraryPath)
		if len(queries) == 0 {
			h.identifyRun.setState(libraryID, result.job.row.Kind, result.job.row.Path, "failed")
			failed++
			continue
		}
		groupKey := strings.ToLower(queries[0])
		group, ok := groups[groupKey]
		if !ok {
			group = &showFallbackGroup{queries: queries}
			groups[groupKey] = group
		}
		group.rows = append(group.rows, result.job.row)
	}

	for _, group := range groups {
		updated, err := h.identifyShowFallbackGroup(ctx, libraryPath, group.queries, group.rows, cache, queueSearch)
		if err != nil || updated != len(group.rows) {
			h.identifyRun.failRows(libraryID, group.rows[updated:])
			identified += updated
			failed += len(group.rows) - updated
			continue
		}
		for _, row := range group.rows {
			h.identifyRun.setState(libraryID, row.Kind, row.Path, "")
		}
		identified += updated
	}

	return identified, failed
}

func episodeGroupKey(row db.EpisodeIdentifyRow, libraryPath string) (string, string, []string) {
	if row.Season <= 0 || row.Episode <= 0 {
		return "", "", nil
	}
	info := identifyMediaInfo(row.IdentificationRow, libraryPath)
	title := strings.TrimSpace(info.Title)
	queries := showFallbackQueries(row.IdentificationRow, libraryPath)
	if row.TMDBID > 0 {
		return "tmdb:" + strconv.Itoa(row.TMDBID), "", nil
	}
	if row.TVDBID != "" {
		return "tvdb:" + row.TVDBID, title, queries
	}
	if title != "" {
		return "title:" + metadata.NormalizeSeriesTitle(title), title, queries
	}
	if len(queries) > 0 {
		return "fallback:" + strings.ToLower(queries[0]), queries[0], queries
	}
	return "", "", nil
}

func buildEpisodeIdentifyGroups(rows []db.EpisodeIdentifyRow, libraryPath string) ([]episodeIdentifyGroup, []identifyJob) {
	groupsByKey := make(map[string]*episodeIdentifyGroup)
	order := make([]string, 0, len(rows))
	residual := make([]identifyJob, 0)
	for _, row := range rows {
		key, query, fallbackQueries := episodeGroupKey(row, libraryPath)
		if key == "" {
			residual = append(residual, identifyJob{row: row.IdentificationRow})
			continue
		}
		group, ok := groupsByKey[key]
		if !ok {
			group = &episodeIdentifyGroup{
				key:             key,
				kind:            row.Kind,
				groupQuery:      strings.TrimSpace(query),
				fallbackQueries: append([]string(nil), fallbackQueries...),
				explicitTMDBID:  row.TMDBID,
				explicitTVDBID:  row.TVDBID,
				representative:  row,
			}
			groupsByKey[key] = group
			order = append(order, key)
		}
		group.rows = append(group.rows, row)
		if row.TMDBID > 0 && group.explicitTMDBID == 0 {
			group.explicitTMDBID = row.TMDBID
		}
		if row.TVDBID != "" && group.explicitTVDBID == "" {
			group.explicitTVDBID = row.TVDBID
		}
		if group.groupQuery == "" && query != "" {
			group.groupQuery = query
		}
		if len(group.fallbackQueries) == 0 && len(fallbackQueries) > 0 {
			group.fallbackQueries = append([]string(nil), fallbackQueries...)
		}
	}

	groups := make([]episodeIdentifyGroup, 0, len(order))
	for _, key := range order {
		group := groupsByKey[key]
		if len(group.rows) < 2 && group.explicitTMDBID == 0 && group.explicitTVDBID == "" {
			residual = append(residual, identifyJob{row: group.rows[0].IdentificationRow})
			continue
		}
		sort.SliceStable(group.rows, func(i, j int) bool {
			if group.rows[i].Season != group.rows[j].Season {
				return group.rows[i].Season < group.rows[j].Season
			}
			if group.rows[i].Episode != group.rows[j].Episode {
				return group.rows[i].Episode < group.rows[j].Episode
			}
			return group.rows[i].Path < group.rows[j].Path
		})
		group.representative = group.rows[0]
		groups = append(groups, *group)
	}
	sortIdentifyJobs(residual, libraryPath)
	return groups, residual
}

func identifyGroupRowsAsQueued(tracker *identifyRunTracker, libraryID int, rows []db.EpisodeIdentifyRow) {
	if tracker == nil {
		return
	}
	for _, row := range rows {
		tracker.setState(libraryID, row.Kind, row.Path, "queued")
	}
}

func identifyGroupRowsAsIdentifying(tracker *identifyRunTracker, libraryID int, rows []db.EpisodeIdentifyRow) {
	if tracker == nil {
		return
	}
	for _, row := range rows {
		tracker.setState(libraryID, row.Kind, row.Path, "identifying")
	}
}

func identifyGroupRowsClear(tracker *identifyRunTracker, libraryID int, rows []db.EpisodeIdentifyRow) {
	if tracker == nil {
		return
	}
	for _, row := range rows {
		tracker.setState(libraryID, row.Kind, row.Path, "")
	}
}

func identifyGroupRowsFail(tracker *identifyRunTracker, libraryID int, rows []db.EpisodeIdentifyRow) {
	if tracker == nil {
		return
	}
	for _, row := range rows {
		tracker.setState(libraryID, row.Kind, row.Path, "failed")
	}
}

func episodeIdentifyFailedResults(group episodeIdentifyGroup) []identifyJobResult {
	out := make([]identifyJobResult, 0, len(group.rows))
	for _, row := range group.rows {
		out = append(out, identifyJobResult{
			status:           identifyJobFailed,
			job:              identifyJob{row: row.IdentificationRow},
			fallbackEligible: true,
		})
	}
	return out
}

func episodeIdentifyFallbackResultsFromJobs(results []identifyJobResult) []identifyJobResult {
	out := make([]identifyJobResult, 0, len(results))
	for _, result := range results {
		if result.status != identifyJobFailed {
			continue
		}
		out = append(out, result)
	}
	return out
}

func (h *LibraryHandler) identifyEpisodesByGroup(
	ctx context.Context,
	libraryID int,
	libraryPath string,
	rows []db.EpisodeIdentifyRow,
) (identifyResult, error) {
	groups, residualJobs := buildEpisodeIdentifyGroups(rows, libraryPath)
	cache := newEpisodicIdentifyCache(h)

	identified := 0
	failedResults := make([]identifyJobResult, 0)

	groupIdentified, retryGroups, groupFailed := h.runEpisodeIdentifyGroups(ctx, libraryID, libraryPath, groups, cache)
	identified += groupIdentified
	failedResults = append(failedResults, groupFailed...)

	retryIdentified, unresolvedGroups, retryFailed := h.runEpisodeIdentifyGroups(ctx, libraryID, libraryPath, retryGroups, cache)
	identified += retryIdentified
	failedResults = append(failedResults, retryFailed...)
	for _, group := range unresolvedGroups {
		failedResults = append(failedResults, episodeIdentifyFailedResults(group)...)
	}

	residualIdentified, residualRetryJobs, residualInitialFailed := h.runIdentifyJobs(ctx, libraryID, libraryPath, residualJobs, nil)
	identified += residualIdentified
	residualRetryIdentified, _, residualRetryFailed := h.runIdentifyJobs(ctx, libraryID, libraryPath, residualRetryJobs, nil)
	identified += residualRetryIdentified
	failedResults = append(failedResults, residualInitialFailed...)
	failedResults = append(failedResults, residualRetryFailed...)

	fallbackIdentified, fallbackFailed := h.identifyShowFallbacks(ctx, libraryID, libraryPath, failedResults, cache, false)
	identified += fallbackIdentified
	return identifyResult{Identified: identified, Failed: fallbackFailed}, nil
}

func (h *LibraryHandler) runEpisodeIdentifyGroups(
	ctx context.Context,
	libraryID int,
	libraryPath string,
	groups []episodeIdentifyGroup,
	cache *episodicIdentifyCache,
) (identified int, retryGroups []episodeIdentifyGroup, failed []identifyJobResult) {
	if len(groups) == 0 {
		return 0, nil, nil
	}

	results := make(chan episodeGroupResult, len(groups))
	groupJobs := make(chan episodeGroupJob)
	config := identifyConfigForKind(groups[0].kind)
	workerCount := config.workers
	if workerCount > len(groups) {
		workerCount = len(groups)
	}
	if workerCount < 1 {
		workerCount = 1
	}
	rateLimiter := newIdentifyRateLimiter(ctx, config.rateInterval, config.rateBurst)

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case job, ok := <-groupJobs:
					if !ok {
						return
					}
					groupIdentified, retry, groupFailed := h.identifyEpisodeGroup(ctx, libraryID, libraryPath, job, cache, rateLimiter)
					results <- episodeGroupResult{
						group:      job.group,
						identified: groupIdentified,
						retry:      retry,
						failed:     groupFailed,
					}
				}
			}
		}()
	}

	go func() {
		defer close(results)
		wg.Wait()
	}()

enqueueLoop:
	for _, group := range groups {
		select {
		case <-ctx.Done():
			break enqueueLoop
		case groupJobs <- episodeGroupJob{group: group, attempt: group.attempt}:
		}
	}
	close(groupJobs)

	for result := range results {
		identified += result.identified
		if result.retry {
			next := result.group
			next.attempt++
			retryGroups = append(retryGroups, next)
		}
		failed = append(failed, result.failed...)
	}

	return identified, retryGroups, failed
}

type tmdbSeriesSelection struct {
	tmdbID               int
	metadataReviewNeeded bool
}

func fallbackIdentifyInfo(row db.IdentificationRow, libraryPath string) metadata.MediaInfo {
	info := identifyMediaInfo(row, libraryPath)
	if info.Season == 0 {
		info.Season = row.Season
	}
	if info.Episode == 0 {
		info.Episode = row.Episode
	}
	if info.Title == "" {
		info.Title = row.Title
	}
	return info
}

func scoredTMDBSeriesMatch(
	candidates []metadata.MatchResult,
	info metadata.MediaInfo,
) (best *metadata.MatchResult, topScore int, secondScore int, hasSecond bool) {
	type scored struct {
		match *metadata.MatchResult
		score int
	}
	scores := make([]scored, 0, len(candidates))
	for i := range candidates {
		candidate := &candidates[i]
		if candidate.Provider != "tmdb" {
			continue
		}
		if tmdbID, err := strconv.Atoi(candidate.ExternalID); err != nil || tmdbID <= 0 {
			continue
		}
		scores = append(scores, scored{
			match: candidate,
			score: metadata.ScoreTV(candidate, info),
		})
	}
	if len(scores) == 0 {
		return nil, 0, 0, false
	}
	sort.SliceStable(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})
	best = scores[0].match
	topScore = scores[0].score
	if len(scores) > 1 {
		secondScore = scores[1].score
		hasSecond = true
	}
	return best, topScore, secondScore, hasSecond
}

func (h *LibraryHandler) selectTMDBSeriesFallback(
	ctx context.Context,
	libraryPath string,
	representative db.IdentificationRow,
	queries []string,
	cache *episodicIdentifyCache,
) (tmdbSeriesSelection, error) {
	info := fallbackIdentifyInfo(representative, libraryPath)
	seenQueries := make(map[string]struct{}, len(queries))
	bestTentative := tmdbSeriesSelection{}
	bestTentativeScore := 0
	hasTentative := false

	for _, query := range queries {
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}
		key := strings.ToLower(query)
		if _, ok := seenQueries[key]; ok {
			continue
		}
		seenQueries[key] = struct{}{}

		var (
			results []metadata.MatchResult
			err     error
		)
		if cache != nil {
			results, err = cache.SearchTV(ctx, query)
		} else {
			results, err = h.SeriesQuery.SearchTV(ctx, query)
		}
		if err != nil {
			return tmdbSeriesSelection{}, err
		}

		best, topScore, secondScore, hasSecond := scoredTMDBSeriesMatch(results, info)
		if best == nil {
			continue
		}
		tmdbID, err := strconv.Atoi(best.ExternalID)
		if err != nil || tmdbID <= 0 {
			continue
		}
		if topScore >= metadata.ScoreAutoMatch &&
			(!hasSecond || (topScore-secondScore) >= metadata.ScoreMargin) {
			return tmdbSeriesSelection{tmdbID: tmdbID, metadataReviewNeeded: false}, nil
		}
		if !hasTentative || topScore > bestTentativeScore {
			bestTentative = tmdbSeriesSelection{
				tmdbID:               tmdbID,
				metadataReviewNeeded: true,
			}
			bestTentativeScore = topScore
			hasTentative = true
		}
	}

	if hasTentative {
		return bestTentative, nil
	}
	return tmdbSeriesSelection{}, nil
}

func (h *LibraryHandler) resolveTMDBSeriesSelectionForGroup(
	ctx context.Context,
	libraryPath string,
	group episodeIdentifyGroup,
	cache *episodicIdentifyCache,
) (tmdbSeriesSelection, error) {
	if group.explicitTMDBID > 0 {
		return tmdbSeriesSelection{tmdbID: group.explicitTMDBID}, nil
	}
	queries := make([]string, 0, len(group.fallbackQueries)+1)
	if group.groupQuery != "" {
		queries = append(queries, group.groupQuery)
	}
	for _, query := range group.fallbackQueries {
		if query == "" {
			continue
		}
		seen := false
		for _, existing := range queries {
			if strings.EqualFold(existing, query) {
				seen = true
				break
			}
		}
		if !seen {
			queries = append(queries, query)
		}
	}
	return h.selectTMDBSeriesFallback(ctx, libraryPath, group.representative.IdentificationRow, queries, cache)
}

func (h *LibraryHandler) identifyEpisodeGroup(
	ctx context.Context,
	libraryID int,
	libraryPath string,
	job episodeGroupJob,
	cache *episodicIdentifyCache,
	rateLimiter <-chan struct{},
) (identified int, retry bool, failed []identifyJobResult) {
	identifyGroupRowsAsIdentifying(h.identifyRun, libraryID, job.group.rows)
	if h.ScanJobs != nil {
		h.ScanJobs.RecordIdentifyActivity(libraryID, job.group.representative.Path)
	}
	select {
	case <-ctx.Done():
		return 0, false, episodeIdentifyFailedResults(job.group)
	case <-rateLimiter:
	}

	itemCtx, cancel := context.WithTimeout(ctx, identifyTimeoutForAttempt(job.attempt))
	defer cancel()

	selection, err := h.resolveTMDBSeriesSelectionForGroup(itemCtx, libraryPath, job.group, cache)
	if err != nil || selection.tmdbID <= 0 {
		if err == nil && job.attempt == 0 {
			identifyGroupRowsAsQueued(h.identifyRun, libraryID, job.group.rows)
			return 0, true, nil
		}
		identifyGroupRowsFail(h.identifyRun, libraryID, job.group.rows)
		return 0, false, episodeIdentifyFailedResults(job.group)
	}

	refs := make([]db.ShowEpisodeRef, 0, len(job.group.rows))
	for _, row := range job.group.rows {
		refs = append(refs, db.ShowEpisodeRef{
			RefID:   row.RefID,
			Kind:    row.Kind,
			Season:  row.Season,
			Episode: row.Episode,
		})
	}
	updatedRefIDs, err := h.applySeriesToRefs(
		itemCtx,
		selection.tmdbID,
		refs,
		selection.metadataReviewNeeded,
		false,
		cache,
		false,
	)
	if err != nil || len(updatedRefIDs) == 0 {
		if err == nil && job.attempt == 0 {
			identifyGroupRowsAsQueued(h.identifyRun, libraryID, job.group.rows)
			return 0, true, nil
		}
		identifyGroupRowsFail(h.identifyRun, libraryID, job.group.rows)
		return 0, false, episodeIdentifyFailedResults(job.group)
	}

	updatedSet := make(map[int]struct{}, len(updatedRefIDs))
	for _, refID := range updatedRefIDs {
		updatedSet[refID] = struct{}{}
	}
	updatedRows := make([]db.EpisodeIdentifyRow, 0, len(updatedSet))
	unresolved := make([]db.EpisodeIdentifyRow, 0)
	for _, row := range job.group.rows {
		if _, ok := updatedSet[row.RefID]; ok {
			updatedRows = append(updatedRows, row)
			continue
		}
		unresolved = append(unresolved, row)
	}
	identifyGroupRowsClear(h.identifyRun, libraryID, updatedRows)
	if len(unresolved) > 0 {
		identifyGroupRowsFail(h.identifyRun, libraryID, unresolved)
		failed = append(failed, episodeIdentifyFailedResults(episodeIdentifyGroup{
			key:             job.group.key,
			kind:            job.group.kind,
			groupQuery:      job.group.groupQuery,
			fallbackQueries: job.group.fallbackQueries,
			rows:            unresolved,
		})...)
	}
	return len(updatedRows), false, failed
}

func newIdentifyRateLimiter(ctx context.Context, interval time.Duration, burst int) <-chan struct{} {
	if burst < 1 {
		burst = 1
	}
	ch := make(chan struct{}, burst)
	for i := 0; i < burst; i++ {
		ch <- struct{}{}
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				select {
				case ch <- struct{}{}:
				default:
				}
			}
		}
	}()
	return ch
}

func identifyTimeoutForAttempt(attempt int) time.Duration {
	if attempt <= 0 {
		return identifyInitialTimeout
	}
	return identifyRetryTimeout
}

func movieIdentifyKey(info metadata.MediaInfo) string {
	title := metadata.NormalizeTitle(info.Title)
	if title == "" {
		return ""
	}
	if info.Year > 0 {
		return title + ":" + strconv.Itoa(info.Year)
	}
	return title
}

func (h *LibraryHandler) identifyMovieResult(
	ctx context.Context,
	info metadata.MediaInfo,
) (*metadata.MatchResult, error) {
	if withError, ok := h.Meta.(metadata.MovieIdentifierWithError); ok {
		return withError.IdentifyMovieResult(ctx, info)
	}
	return h.Meta.IdentifyMovie(ctx, info), nil
}

func logRetryableMovieIdentifyFailure(libraryID int, title string, err error) {
	var providerErr *metadata.ProviderError
	if errors.As(err, &providerErr) {
		log.Printf(
			"identify movie retryable failure library=%d provider=%s status=%d title=%q error=%v",
			libraryID,
			providerErr.Provider,
			providerErr.StatusCode,
			title,
			err,
		)
		return
	}
	log.Printf("identify movie retryable failure library=%d title=%q error=%v", libraryID, title, err)
}

func updateMetadataWithRetry(
	dbConn *sql.DB,
	table string,
	refID int,
	title string,
	overview string,
	posterPath string,
	backdropPath string,
	releaseDate string,
	voteAvg float64,
	imdbID string,
	imdbRating float64,
	tmdbID int,
	tvdbID string,
	season int,
	episode int,
	canonical db.CanonicalMetadata,
	metadataReviewNeeded bool,
	metadataConfirmed bool,
	updateShowVoteAverage bool,
) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		lastErr = db.UpdateMediaMetadataWithCanonicalState(
			dbConn,
			table,
			refID,
			title,
			overview,
			posterPath,
			backdropPath,
			releaseDate,
			voteAvg,
			imdbID,
			imdbRating,
			tmdbID,
			tvdbID,
			season,
			episode,
			canonical,
			metadataReviewNeeded,
			metadataConfirmed,
			updateShowVoteAverage,
		)
		if lastErr == nil || !isSQLiteBusyError(lastErr) {
			return lastErr
		}
		time.Sleep(time.Duration(attempt+1) * 25 * time.Millisecond)
	}
	return lastErr
}

type existingEpisodeMetadata struct {
	Title        string
	Overview     string
	PosterPath   string
	BackdropPath string
	ReleaseDate  string
	VoteAverage  float64
	IMDbID       string
	IMDbRating   float64
}

func loadExistingEpisodeMetadata(dbConn *sql.DB, table string, refID int) (*existingEpisodeMetadata, error) {
	if table != "tv_episodes" && table != "anime_episodes" {
		return nil, nil
	}
	row := &existingEpisodeMetadata{}
	if err := dbConn.QueryRow(
		`SELECT title, COALESCE(overview, ''), COALESCE(poster_path, ''), COALESCE(backdrop_path, ''), COALESCE(release_date, ''), COALESCE(vote_average, 0), COALESCE(imdb_id, ''), COALESCE(imdb_rating, 0) FROM `+table+` WHERE id = ?`,
		refID,
	).Scan(
		&row.Title,
		&row.Overview,
		&row.PosterPath,
		&row.BackdropPath,
		&row.ReleaseDate,
		&row.VoteAverage,
		&row.IMDbID,
		&row.IMDbRating,
	); err != nil {
		return nil, err
	}
	return row, nil
}

func (h *LibraryHandler) identifyLibraryJob(
	ctx context.Context,
	libraryID int,
	job identifyJob,
	libraryPath string,
	rateLimiter <-chan struct{},
	movieCache *movieIdentifyCache,
) identifyJobResult {
	h.identifyRun.setState(libraryID, job.row.Kind, job.row.Path, "identifying")
	if h.ScanJobs != nil {
		h.ScanJobs.RecordIdentifyActivity(libraryID, job.row.Path)
	}
	select {
	case <-ctx.Done():
		return identifyJobResult{job: job}
	case <-rateLimiter:
	}

	row := job.row
	info := identifyMediaInfo(row, libraryPath)
	if info.Season == 0 {
		info.Season = row.Season
	}
	if info.Episode == 0 {
		info.Episode = row.Episode
	}
	if info.Title == "" {
		info.Title = row.Title
	}

	itemCtx, cancel := context.WithTimeout(ctx, identifyTimeoutForAttempt(job.attempt))
	defer cancel()

	var (
		res      *metadata.MatchResult
		movieErr error
	)
	switch row.Kind {
	case db.LibraryTypeTV:
		res = h.Meta.IdentifyTV(itemCtx, info)
	case db.LibraryTypeAnime:
		res = h.Meta.IdentifyAnime(itemCtx, info)
	case db.LibraryTypeMovie:
		if movieCache != nil {
			res, movieErr = movieCache.lookupOrRun(movieIdentifyKey(info), func() (*metadata.MatchResult, error) {
				return h.identifyMovieResult(itemCtx, info)
			})
		} else {
			res, movieErr = h.identifyMovieResult(itemCtx, info)
		}
	default:
		return identifyJobResult{status: identifyJobFailed, job: job}
	}
	if row.Kind == db.LibraryTypeMovie && movieErr != nil {
		if metadata.IsRetryableProviderError(movieErr) && job.attempt == 0 {
			logRetryableMovieIdentifyFailure(libraryID, row.Title, movieErr)
			h.identifyRun.setState(libraryID, row.Kind, row.Path, "queued")
			return identifyJobResult{status: identifyJobRetry, job: job}
		}
		if metadata.IsRetryableProviderError(movieErr) {
			logRetryableMovieIdentifyFailure(libraryID, row.Title, movieErr)
		}
		return identifyJobResult{status: identifyJobFailed, job: job}
	}
	if res == nil {
		if row.Kind == db.LibraryTypeMovie {
			return identifyJobResult{status: identifyJobFailed, job: job}
		}
		if job.attempt == 0 {
			h.identifyRun.setState(libraryID, row.Kind, row.Path, "queued")
			return identifyJobResult{status: identifyJobRetry, job: job}
		}
		return identifyJobResult{
			status:           identifyJobFailed,
			job:              job,
			fallbackEligible: (row.Kind == db.LibraryTypeAnime || row.Kind == db.LibraryTypeTV) && itemCtx.Err() == nil,
		}
	}

	tmdbID, tvdbID := 0, ""
	switch res.Provider {
	case "tmdb":
		if id, err := strconv.Atoi(res.ExternalID); err == nil {
			tmdbID = id
		}
	case "tvdb":
		tvdbID = res.ExternalID
	}
	tbl := db.MediaTableForKind(row.Kind)
	cast := make([]db.CastCredit, 0, len(res.Cast))
	for _, member := range res.Cast {
		cast = append(cast, db.CastCredit{
			Name:        member.Name,
			Character:   member.Character,
			Order:       member.Order,
			ProfilePath: member.ProfilePath,
			Provider:    member.Provider,
			ProviderID:  member.ProviderID,
		})
	}
	seasonNumber := row.Season
	if seasonNumber == 0 {
		seasonNumber = info.Season
	}
	episodeNumber := row.Episode
	if episodeNumber == 0 {
		episodeNumber = info.Episode
	}
	posterPath := res.PosterURL
	settings := loadMetadataArtworkSettings(h.DB)
	canonical := db.CanonicalMetadata{
		Title:        res.Title,
		Overview:     res.Overview,
		BackdropPath: res.BackdropURL,
		ReleaseDate:  res.ReleaseDate,
		VoteAverage:  res.VoteAverage,
		IMDbID:       res.IMDbID,
		IMDbRating:   res.IMDbRating,
		Genres:       res.Genres,
		Cast:         cast,
		Runtime:      res.Runtime,
	}
	switch row.Kind {
	case db.LibraryTypeMovie:
		posterPath = automaticMoviePosterSource(
			ctx,
			h.Artwork,
			settings,
			tmdbID,
			res.IMDbID,
			res.PosterURL,
			res.Provider,
		)
	case db.LibraryTypeTV, db.LibraryTypeAnime:
		showTitle := showTitleFromEpisodeTitle(res.Title)
		canonical.PosterPath = automaticShowPosterSource(
			ctx,
			h.Artwork,
			settings,
			showTitle,
			tmdbID,
			tvdbID,
			res.PosterURL,
			res.Provider,
		)
		canonical.SeasonPosterPath = automaticSeasonPosterSource(
			ctx,
			h.Artwork,
			settings,
			showTitle,
			tmdbID,
			tvdbID,
			seasonNumber,
			canonical.PosterPath,
			"",
		)
		if canonical.SeasonPosterPath == "" {
			canonical.SeasonPosterPath = canonical.PosterPath
		}
		posterPath = automaticEpisodePosterSource(
			ctx,
			h.Artwork,
			settings,
			showTitle,
			tmdbID,
			tvdbID,
			res.IMDbID,
			seasonNumber,
			episodeNumber,
			res.PosterURL,
			res.Provider,
		)
	}
	if err := updateMetadataWithRetry(h.DB, tbl, row.RefID, res.Title, res.Overview, posterPath, res.BackdropURL, res.ReleaseDate, res.VoteAverage, res.IMDbID, res.IMDbRating, tmdbID, tvdbID, seasonNumber, episodeNumber, canonical, false, false, true); err != nil {
		return identifyJobResult{status: identifyJobFailed, job: job}
	}
	h.identifyRun.setState(libraryID, row.Kind, row.Path, "")
	return identifyJobResult{status: identifyJobSucceeded, job: job}
}

func (h *LibraryHandler) identifyShowFallbackGroup(
	ctx context.Context,
	libraryPath string,
	queries []string,
	rows []db.IdentificationRow,
	cache *episodicIdentifyCache,
	queueSearch bool,
) (int, error) {
	if h.SeriesQuery == nil || len(rows) == 0 {
		return 0, nil
	}
	selection, err := h.selectTMDBSeriesFallback(ctx, libraryPath, rows[0], queries, cache)
	if err != nil {
		return 0, err
	}
	if selection.tmdbID <= 0 {
		return 0, nil
	}
	refs := make([]db.ShowEpisodeRef, 0, len(rows))
	for _, row := range rows {
		refs = append(refs, db.ShowEpisodeRef{
			RefID:   row.RefID,
			Kind:    row.Kind,
			Season:  row.Season,
			Episode: row.Episode,
		})
	}
	updatedRefIDs, err := h.applySeriesToRefs(
		ctx,
		selection.tmdbID,
		refs,
		selection.metadataReviewNeeded,
		false,
		cache,
		queueSearch,
	)
	return len(updatedRefIDs), err
}

func showFallbackQueries(row db.IdentificationRow, libraryPath string) []string {
	info := identifyMediaInfo(row, libraryPath)
	candidates := []string{
		showTitleFromEpisodeTitle(row.Title),
		strings.TrimSpace(info.Title),
	}
	seen := make(map[string]struct{}, len(candidates))
	queries := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		key := strings.ToLower(candidate)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		queries = append(queries, candidate)
	}
	return queries
}

func showTitleFromEpisodeTitle(title string) string {
	title = strings.TrimSpace(title)
	if i := strings.Index(strings.ToLower(title), " - s"); i > 0 {
		return strings.TrimSpace(title[:i])
	}
	if i := strings.Index(title, " - "); i > 0 {
		return strings.TrimSpace(title[:i])
	}
	return title
}

func (h *LibraryHandler) applySeriesToRefs(
	ctx context.Context,
	seriesTMDBID int,
	refs []db.ShowEpisodeRef,
	metadataReviewNeeded bool,
	metadataConfirmed bool,
	cache *episodicIdentifyCache,
	queueSearch bool,
) ([]int, error) {
	if h.SeriesQuery == nil || seriesTMDBID <= 0 || len(refs) == 0 {
		return nil, nil
	}
	table := db.MediaTableForKind(refs[0].Kind)
	seriesID := strconv.Itoa(seriesTMDBID)
	var canonical db.CanonicalMetadata
	seriesTVDBID := ""
	if h.Series != nil {
		var details *metadata.SeriesDetails
		var err error
		if cache != nil {
			details, err = cache.GetSeriesDetails(ctx, seriesTMDBID)
		} else {
			details, err = h.Series.GetSeriesDetails(ctx, seriesTMDBID)
		}
		if err == nil && details != nil {
			cast := make([]db.CastCredit, 0, len(details.Cast))
			for _, member := range details.Cast {
				cast = append(cast, db.CastCredit{
					Name:        member.Name,
					Character:   member.Character,
					Order:       member.Order,
					ProfilePath: member.ProfilePath,
					Provider:    member.Provider,
					ProviderID:  member.ProviderID,
				})
			}
			canonical = db.CanonicalMetadata{
				Title:        details.Name,
				Overview:     details.Overview,
				PosterPath:   details.PosterPath,
				BackdropPath: details.BackdropPath,
				ReleaseDate:  details.FirstAirDate,
				VoteAverage:  details.VoteAverage,
				IMDbID:       details.IMDbID,
				IMDbRating:   details.IMDbRating,
				Genres:       details.Genres,
				Cast:         cast,
				Runtime:      details.Runtime,
			}
			seriesTVDBID = details.TVDBID
		}
	}
	settings := loadMetadataArtworkSettings(h.DB)
	showTitle := strings.TrimSpace(canonical.Title)
	canonical.PosterPath = automaticShowPosterSource(
		ctx,
		h.Artwork,
		settings,
		showTitle,
		seriesTMDBID,
		seriesTVDBID,
		canonical.PosterPath,
		"tmdb",
	)
	updatedRefIDs := make([]int, 0, len(refs))
	for _, ref := range refs {
		var (
			ep  *metadata.MatchResult
			err error
		)
		if cache != nil {
			ep, err = cache.GetEpisode(ctx, "tmdb", seriesID, ref.Season, ref.Episode)
		} else {
			ep, err = h.SeriesQuery.GetEpisode(ctx, "tmdb", seriesID, ref.Season, ref.Episode)
		}
		if err != nil || ep == nil {
			if !metadataConfirmed {
				continue
			}
			if len(strings.TrimSpace(canonical.Title)) == 0 {
				continue
			}
			existing, loadErr := loadExistingEpisodeMetadata(h.DB, table, ref.RefID)
			if loadErr != nil || existing == nil {
				continue
			}
			fallbackCanonical := canonical
			fallbackCanonical.SeasonPosterPath = automaticSeasonPosterSource(
				ctx,
				h.Artwork,
				settings,
				showTitle,
				seriesTMDBID,
				seriesTVDBID,
				ref.Season,
				fallbackCanonical.PosterPath,
				"tmdb",
			)
			if fallbackCanonical.SeasonPosterPath == "" {
				fallbackCanonical.SeasonPosterPath = fallbackCanonical.PosterPath
			}
			if err := updateMetadataWithRetry(
				h.DB,
				table,
				ref.RefID,
				existing.Title,
				existing.Overview,
				existing.PosterPath,
				existing.BackdropPath,
				existing.ReleaseDate,
				existing.VoteAverage,
				existing.IMDbID,
				existing.IMDbRating,
				seriesTMDBID,
				seriesTVDBID,
				ref.Season,
				ref.Episode,
				fallbackCanonical,
				metadataReviewNeeded,
				metadataConfirmed,
				true,
			); err != nil {
				continue
			}
			updatedRefIDs = append(updatedRefIDs, ref.RefID)
			if cache == nil {
				time.Sleep(identifyEpisodeRateLimit)
			}
			continue
		}
		if showTitle == "" {
			showTitle = showTitleFromEpisodeTitle(ep.Title)
		}
		tvdbID := ""
		if ep.Provider == "tvdb" {
			tvdbID = ep.ExternalID
		}
		if tvdbID == "" {
			tvdbID = seriesTVDBID
		}
		episodeCanonical := canonical
		episodeCanonical.SeasonPosterPath = automaticSeasonPosterSource(
			ctx,
			h.Artwork,
			settings,
			showTitle,
			seriesTMDBID,
			tvdbID,
			ref.Season,
			episodeCanonical.PosterPath,
			"tmdb",
		)
		if episodeCanonical.SeasonPosterPath == "" {
			episodeCanonical.SeasonPosterPath = episodeCanonical.PosterPath
		}
		episodePosterPath := automaticEpisodePosterSource(
			ctx,
			h.Artwork,
			settings,
			showTitle,
			seriesTMDBID,
			tvdbID,
			ep.IMDbID,
			ref.Season,
			ref.Episode,
			ep.PosterURL,
			ep.Provider,
		)
		if err := updateMetadataWithRetry(h.DB, table, ref.RefID, ep.Title, ep.Overview, episodePosterPath, ep.BackdropURL, ep.ReleaseDate, ep.VoteAverage, ep.IMDbID, ep.IMDbRating, seriesTMDBID, tvdbID, ref.Season, ref.Episode, episodeCanonical, metadataReviewNeeded, metadataConfirmed, true); err != nil {
			continue
		}
		updatedRefIDs = append(updatedRefIDs, ref.RefID)
		if cache == nil {
			time.Sleep(identifyEpisodeRateLimit)
		}
	}
	if len(updatedRefIDs) > 0 && queueSearch && len(refs) > 0 && h.SearchIndex != nil {
		var libraryID int
		if err := h.DB.QueryRow(`SELECT library_id FROM `+table+` WHERE id = ?`, refs[0].RefID).Scan(&libraryID); err == nil {
			h.SearchIndex.Queue(libraryID, false)
		}
	}
	return updatedRefIDs, nil
}

func (h *LibraryHandler) applyTMDBSeriesToRefs(
	ctx context.Context,
	seriesTMDBID int,
	refs []db.ShowEpisodeRef,
	metadataReviewNeeded bool,
	metadataConfirmed bool,
) (int, error) {
	updatedRefIDs, err := h.applySeriesToRefs(ctx, seriesTMDBID, refs, metadataReviewNeeded, metadataConfirmed, nil, true)
	return len(updatedRefIDs), err
}

func (h *LibraryHandler) applySeriesMatchToRefs(
	ctx context.Context,
	provider string,
	externalID string,
	refs []db.ShowEpisodeRef,
	metadataReviewNeeded bool,
	metadataConfirmed bool,
) (int, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	externalID = strings.TrimSpace(externalID)
	if provider == "" || externalID == "" || len(refs) == 0 || h.SeriesQuery == nil {
		return 0, nil
	}
	if provider == "tmdb" {
		seriesTMDBID, err := strconv.Atoi(externalID)
		if err != nil || seriesTMDBID <= 0 {
			return 0, nil
		}
		return h.applyTMDBSeriesToRefs(ctx, seriesTMDBID, refs, metadataReviewNeeded, metadataConfirmed)
	}

	table := db.MediaTableForKind(refs[0].Kind)
	updatedRefIDs := make([]int, 0, len(refs))
	for _, ref := range refs {
		ep, err := h.SeriesQuery.GetEpisode(ctx, provider, externalID, ref.Season, ref.Episode)
		if err != nil || ep == nil {
			continue
		}
		tmdbID := 0
		tvdbID := ""
		switch ep.Provider {
		case "tmdb":
			tmdbID, _ = strconv.Atoi(ep.ExternalID)
		case "tvdb":
			tvdbID = ep.ExternalID
		}
		if tvdbID == "" && provider == "tvdb" {
			tvdbID = externalID
		}
		showTitle := showTitleFromEpisodeTitle(ep.Title)
		settings := loadMetadataArtworkSettings(h.DB)
		posterPath := automaticEpisodePosterSource(
			ctx,
			h.Artwork,
			settings,
			showTitle,
			tmdbID,
			tvdbID,
			ep.IMDbID,
			ref.Season,
			ref.Episode,
			ep.PosterURL,
			ep.Provider,
		)
		canonical := db.CanonicalMetadata{
			Title:            showTitle,
			Overview:         ep.Overview,
			PosterPath:       posterPath,
			SeasonPosterPath: posterPath,
			BackdropPath:     ep.BackdropURL,
			ReleaseDate:      ep.ReleaseDate,
			// Show vote_average must come from provider series metadata (see migration 23), not per-episode scores.
			VoteAverage: 0,
			IMDbID:      ep.IMDbID,
			IMDbRating:  ep.IMDbRating,
			Genres:      ep.Genres,
			Runtime:     ep.Runtime,
		}
		if err := updateMetadataWithRetry(h.DB, table, ref.RefID, ep.Title, ep.Overview, posterPath, ep.BackdropURL, ep.ReleaseDate, ep.VoteAverage, ep.IMDbID, ep.IMDbRating, tmdbID, tvdbID, ref.Season, ref.Episode, canonical, metadataReviewNeeded, metadataConfirmed, false); err != nil {
			continue
		}
		updatedRefIDs = append(updatedRefIDs, ref.RefID)
	}
	if len(updatedRefIDs) > 0 && h.SearchIndex != nil {
		var libraryID int
		if err := h.DB.QueryRow(`SELECT library_id FROM `+table+` WHERE id = ?`, refs[0].RefID).Scan(&libraryID); err == nil {
			h.SearchIndex.Queue(libraryID, false)
		}
	}
	return len(updatedRefIDs), nil
}

func identifyMediaInfo(row db.IdentificationRow, libraryPath string) metadata.MediaInfo {
	base := filepath.Base(row.Path)
	relPath, _ := filepath.Rel(libraryPath, row.Path)
	applyProviderHints := func(info metadata.MediaInfo) metadata.MediaInfo {
		if info.TMDBID <= 0 && row.TMDBID > 0 {
			info.TMDBID = row.TMDBID
		}
		if info.TVDBID == "" && row.TVDBID != "" {
			info.TVDBID = row.TVDBID
		}
		return info
	}
	switch row.Kind {
	case db.LibraryTypeMovie:
		return applyProviderHints(metadata.MovieMediaInfo(metadata.ParseMovie(relPath, base)))
	case db.LibraryTypeTV, db.LibraryTypeAnime:
		info := metadata.ParseFilename(base)
		pathInfo := metadata.ParsePathForTV(relPath, base)
		info = metadata.MergePathInfo(pathInfo, info)
		showRoot := metadata.ShowRootPath(libraryPath, row.Path)
		metadata.ApplyShowNFO(&info, showRoot)
		if row.Kind == db.LibraryTypeAnime && info.IsSpecial && info.Episode > 0 {
			info.Season = 0
		}
		return applyProviderHints(info)
	default:
		return applyProviderHints(metadata.ParseFilename(base))
	}
}

func sortIdentifyJobs(jobs []identifyJob, libraryPath string) {
	sort.SliceStable(jobs, func(i, j int) bool {
		a := identifyJobPriority(jobs[i], libraryPath)
		b := identifyJobPriority(jobs[j], libraryPath)
		if a != b {
			return a < b
		}
		if jobs[i].row.Kind != jobs[j].row.Kind {
			return jobs[i].row.Kind < jobs[j].row.Kind
		}
		if jobs[i].row.Season != jobs[j].row.Season {
			return jobs[i].row.Season < jobs[j].row.Season
		}
		if jobs[i].row.Episode != jobs[j].row.Episode {
			return jobs[i].row.Episode < jobs[j].row.Episode
		}
		return jobs[i].row.Path < jobs[j].row.Path
	})
}

func identifyJobPriority(job identifyJob, libraryPath string) int {
	info := identifyMediaInfo(job.row, libraryPath)
	switch job.row.Kind {
	case db.LibraryTypeMovie:
		if info.TMDBID > 0 || info.TVDBID != "" {
			return 0
		}
		if info.Year > 0 {
			return 1
		}
		return 2
	case db.LibraryTypeTV, db.LibraryTypeAnime:
		season := info.Season
		if season == 0 {
			season = job.row.Season
		}
		episode := info.Episode
		if episode == 0 {
			episode = job.row.Episode
		}
		if (season == 1 || season == 0) && episode == 1 {
			return 0
		}
		if episode > 0 && episode <= 3 {
			return 1
		}
		return 2
	default:
		return 3
	}
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

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(added)
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
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
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

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(h.ScanJobs.status(libraryID))
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

func (h *LibraryHandler) GetSeriesDetails(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	tmdbIDStr := chi.URLParam(r, "tmdbId")
	tmdbID, err := strconv.Atoi(tmdbIDStr)
	if err != nil || tmdbID <= 0 {
		http.Error(w, "invalid tmdb id", http.StatusBadRequest)
		return
	}
	if h.Series == nil {
		http.Error(w, "series metadata not configured", http.StatusServiceUnavailable)
		return
	}
	details, err := h.Series.GetSeriesDetails(r.Context(), tmdbID)
	if err != nil {
		http.Error(w, "failed to fetch series: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if details == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(details)
}

func (h *LibraryHandler) ListLibraryMedia(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	idStr := chi.URLParam(r, "id")
	var libraryID, ownerID int
	err := h.DB.QueryRow(`SELECT id, user_id FROM libraries WHERE id = ?`, idStr).Scan(&libraryID, &ownerID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if ownerID != u.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil || parsed < 0 {
			http.Error(w, "invalid offset", http.StatusBadRequest)
			return
		}
		offset = parsed
	}
	limit := 60
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil || parsed <= 0 {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		if parsed > 200 {
			parsed = 200
		}
		limit = parsed
	}

	page, err := db.GetMediaPageByLibraryIDForUser(h.DB, libraryID, u.ID, offset, limit)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if identifyStates := h.identifyRun.stateForLibrary(libraryID); len(identifyStates) > 0 {
		for i := range page.Items {
			if state, ok := identifyStates[identifyRowKey(page.Items[i].Type, page.Items[i].Path)]; ok {
				page.Items[i].IdentifyState = state
			}
		}
	}
	response := libraryMediaPageResponse{
		Items:      make([]libraryBrowseItemResponse, 0, len(page.Items)),
		NextOffset: page.NextOffset,
		HasMore:    page.HasMore,
		Total:      page.Total,
	}
	for _, item := range page.Items {
		response.Items = append(response.Items, buildLibraryBrowseItemResponse(item))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func (h *LibraryHandler) GetHomeDashboard(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	dashboard, err := db.GetHomeDashboardForUser(h.DB, u.ID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(dashboard)
}

func (h *LibraryHandler) GetDiscover(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.Discover == nil {
		http.Error(w, metadata.ErrTMDBNotConfigured.Error(), http.StatusServiceUnavailable)
		return
	}
	payload, err := h.Discover.GetDiscover(r.Context())
	if err != nil {
		status, message := discoverHTTPStatus(err)
		http.Error(w, message, status)
		return
	}
	if payload == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	for i := range payload.Shelves {
		if err := db.AttachDiscoverLibraryMatches(h.DB, u.ID, payload.Shelves[i].Items); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	h.enrichDiscoverShelvesAcquisition(r.Context(), payload)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func (h *LibraryHandler) GetDiscoverGenres(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.Discover == nil {
		http.Error(w, metadata.ErrTMDBNotConfigured.Error(), http.StatusServiceUnavailable)
		return
	}

	payload, err := h.Discover.GetDiscoverGenres(r.Context())
	if err != nil {
		status, message := discoverHTTPStatus(err)
		http.Error(w, message, status)
		return
	}
	if payload == nil {
		payload = &metadata.DiscoverGenresResponse{
			MovieGenres: []metadata.DiscoverGenre{},
			TVGenres:    []metadata.DiscoverGenre{},
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func parseDiscoverBrowseCategory(raw string) (metadata.DiscoverBrowseCategory, bool) {
	switch metadata.DiscoverBrowseCategory(strings.TrimSpace(raw)) {
	case "":
		return "", true
	case metadata.DiscoverBrowseCategoryTrending,
		metadata.DiscoverBrowseCategoryPopularMovies,
		metadata.DiscoverBrowseCategoryPopularTV,
		metadata.DiscoverBrowseCategoryNowPlaying,
		metadata.DiscoverBrowseCategoryUpcoming,
		metadata.DiscoverBrowseCategoryOnTheAir,
		metadata.DiscoverBrowseCategoryTopRated:
		return metadata.DiscoverBrowseCategory(strings.TrimSpace(raw)), true
	default:
		return "", false
	}
}

func parseDiscoverBrowseMediaType(raw string) (metadata.DiscoverMediaType, bool) {
	switch metadata.DiscoverMediaType(strings.TrimSpace(raw)) {
	case "":
		return "", true
	case metadata.DiscoverMediaTypeMovie, metadata.DiscoverMediaTypeTV:
		return metadata.DiscoverMediaType(strings.TrimSpace(raw)), true
	default:
		return "", false
	}
}

func (h *LibraryHandler) BrowseDiscover(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.Discover == nil {
		http.Error(w, metadata.ErrTMDBNotConfigured.Error(), http.StatusServiceUnavailable)
		return
	}

	category, ok := parseDiscoverBrowseCategory(r.URL.Query().Get("category"))
	if !ok {
		http.Error(w, "invalid discover category", http.StatusBadRequest)
		return
	}
	mediaType, ok := parseDiscoverBrowseMediaType(r.URL.Query().Get("media_type"))
	if !ok {
		http.Error(w, "invalid discover media type", http.StatusBadRequest)
		return
	}

	genreID := 0
	if rawGenre := strings.TrimSpace(r.URL.Query().Get("genre")); rawGenre != "" {
		parsedGenre, err := strconv.Atoi(rawGenre)
		if err != nil || parsedGenre <= 0 {
			http.Error(w, "invalid discover genre", http.StatusBadRequest)
			return
		}
		genreID = parsedGenre
	}

	page := 1
	if rawPage := strings.TrimSpace(r.URL.Query().Get("page")); rawPage != "" {
		parsedPage, err := strconv.Atoi(rawPage)
		if err != nil || parsedPage <= 0 {
			http.Error(w, "invalid discover page", http.StatusBadRequest)
			return
		}
		page = parsedPage
	}

	payload, err := h.Discover.BrowseDiscover(r.Context(), category, mediaType, genreID, page)
	if err != nil {
		status, message := discoverHTTPStatus(err)
		http.Error(w, message, status)
		return
	}
	if payload == nil {
		payload = &metadata.DiscoverBrowseResponse{
			Items:        []metadata.DiscoverItem{},
			Page:         page,
			TotalPages:   1,
			TotalResults: 0,
			Category:     category,
			MediaType:    mediaType,
		}
	}
	if err := db.AttachDiscoverLibraryMatches(h.DB, u.ID, payload.Items); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.enrichDiscoverItemsAcquisition(r.Context(), payload.Items)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func (h *LibraryHandler) SearchDiscover(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&metadata.DiscoverSearchResponse{
			Movies: []metadata.DiscoverItem{},
			TV:     []metadata.DiscoverItem{},
		})
		return
	}
	if h.Discover == nil {
		http.Error(w, metadata.ErrTMDBNotConfigured.Error(), http.StatusServiceUnavailable)
		return
	}
	payload, err := h.Discover.SearchDiscover(r.Context(), query)
	if err != nil {
		status, message := discoverHTTPStatus(err)
		http.Error(w, message, status)
		return
	}
	if payload == nil {
		payload = &metadata.DiscoverSearchResponse{
			Movies: []metadata.DiscoverItem{},
			TV:     []metadata.DiscoverItem{},
		}
	}
	if err := db.AttachDiscoverLibraryMatches(h.DB, u.ID, payload.Movies); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := db.AttachDiscoverLibraryMatches(h.DB, u.ID, payload.TV); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.enrichDiscoverSearchAcquisition(r.Context(), payload)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func (h *LibraryHandler) GetDiscoverTitleDetails(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.Discover == nil {
		http.Error(w, metadata.ErrTMDBNotConfigured.Error(), http.StatusServiceUnavailable)
		return
	}

	mediaType := metadata.DiscoverMediaType(strings.TrimSpace(chi.URLParam(r, "mediaType")))
	if mediaType != metadata.DiscoverMediaTypeMovie && mediaType != metadata.DiscoverMediaTypeTV {
		http.Error(w, "invalid media type", http.StatusBadRequest)
		return
	}
	tmdbID, err := strconv.Atoi(chi.URLParam(r, "tmdbId"))
	if err != nil || tmdbID <= 0 {
		http.Error(w, "invalid tmdb id", http.StatusBadRequest)
		return
	}

	details, err := h.Discover.GetDiscoverTitleDetails(r.Context(), mediaType, tmdbID)
	if err != nil {
		status, message := discoverHTTPStatus(err)
		http.Error(w, message, status)
		return
	}
	if details == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err := db.AttachDiscoverTitleLibraryMatches(h.DB, u.ID, details); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.enrichDiscoverTitleAcquisition(r.Context(), details)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(details)
}

func (h *LibraryHandler) UpdateMediaProgress(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	mediaID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || mediaID <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	item, err := db.GetMediaByID(h.DB, mediaID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if item == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	var ownerID int
	if err := h.DB.QueryRow(`SELECT user_id FROM libraries WHERE id = ?`, item.LibraryID).Scan(&ownerID); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if ownerID != u.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var payload updateMediaProgressRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := db.UpsertPlaybackProgress(h.DB, u.ID, mediaID, payload.PositionSeconds, payload.DurationSeconds, payload.Completed); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *LibraryHandler) GetMovieSearch(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
		return
	}
	if h.MovieQuery == nil {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
		return
	}
	results, err := h.MovieQuery.SearchMovie(r.Context(), q)
	if err != nil {
		http.Error(w, "search failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if results == nil {
		results = []metadata.MatchResult{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(results)
}

func (h *LibraryHandler) GetSeriesSearch(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	q := r.URL.Query().Get("q")
	if q == "" {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
		return
	}
	if h.SeriesQuery == nil {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
		return
	}
	results, err := h.SeriesQuery.SearchTV(r.Context(), q)
	if err != nil {
		http.Error(w, "search failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if results == nil {
		results = []metadata.MatchResult{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(results)
}

func (h *LibraryHandler) SearchLibraryMedia(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	libraryID := 0
	if rawLibraryID := strings.TrimSpace(r.URL.Query().Get("library_id")); rawLibraryID != "" {
		parsed, err := strconv.Atoi(rawLibraryID)
		if err != nil || parsed <= 0 {
			http.Error(w, "invalid library_id", http.StatusBadRequest)
			return
		}
		libraryID = parsed
	}
	limit := 30
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		limit = parsed
	}
	searchType := strings.TrimSpace(r.URL.Query().Get("type"))
	if searchType != "" && searchType != "movie" && searchType != "show" {
		http.Error(w, "invalid type", http.StatusBadRequest)
		return
	}
	results, err := db.SearchLibraryMedia(h.DB, db.SearchQuery{
		UserID:    u.ID,
		Query:     strings.TrimSpace(r.URL.Query().Get("q")),
		LibraryID: libraryID,
		Type:      searchType,
		Genre:     strings.TrimSpace(r.URL.Query().Get("genre")),
		Limit:     limit,
	})
	if err != nil {
		http.Error(w, "search failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(results)
}

func (h *LibraryHandler) GetLibraryMovieDetails(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	libraryID, _, _, _, ok := h.authorizeLibraryRequest(w, r, u.ID)
	if !ok {
		return
	}
	mediaID, err := strconv.Atoi(chi.URLParam(r, "mediaId"))
	if err != nil || mediaID <= 0 {
		http.Error(w, "invalid media id", http.StatusBadRequest)
		return
	}
	details, err := db.GetLibraryMovieDetails(h.DB, libraryID, mediaID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if details == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err := db.AttachPlaybackProgressToLibraryMovieDetails(h.DB, u.ID, details.MediaID, details); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(details)
}

func (h *LibraryHandler) IdentifyMovie(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	idStr := chi.URLParam(r, "id")
	var libraryID, ownerID int
	err := h.DB.QueryRow(`SELECT id, user_id FROM libraries WHERE id = ?`, idStr).Scan(&libraryID, &ownerID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if ownerID != u.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var payload identifyMovieRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	payload.Provider = strings.ToLower(strings.TrimSpace(payload.Provider))
	payload.ExternalID = strings.TrimSpace(payload.ExternalID)
	if payload.Provider == "" && payload.TmdbID > 0 {
		payload.Provider = "tmdb"
	}
	if payload.ExternalID == "" && payload.TmdbID > 0 {
		payload.ExternalID = strconv.Itoa(payload.TmdbID)
	}
	if payload.MediaID <= 0 || payload.Provider == "" || payload.ExternalID == "" {
		http.Error(w, "mediaId, provider, and externalId are required", http.StatusBadRequest)
		return
	}
	if h.MovieLookup == nil {
		http.Error(w, "metadata not configured", http.StatusServiceUnavailable)
		return
	}

	var refID int
	// Browse/detail APIs expose media_global.id (g.id); movies row primary key is movies.id.
	err = h.DB.QueryRow(`
SELECT m.id FROM movies m
JOIN media_global g ON g.kind = 'movie' AND g.ref_id = m.id
WHERE m.library_id = ? AND g.id = ?`, libraryID, payload.MediaID).Scan(&refID)
	if err != nil {
		if err != sql.ErrNoRows {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		// Backwards compatibility if a client ever sent movies.id directly.
		err = h.DB.QueryRow(`SELECT id FROM movies WHERE library_id = ? AND id = ?`, libraryID, payload.MediaID).Scan(&refID)
	}
	if err != nil {
		if err == sql.ErrNoRows {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(showActionResult{Updated: 0})
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	match, err := h.MovieLookup.GetMovie(r.Context(), payload.ExternalID)
	if err != nil {
		http.Error(w, "identify failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if match == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(showActionResult{Updated: 0})
		return
	}

	tmdbID := 0
	if match.Provider == "tmdb" {
		tmdbID, _ = strconv.Atoi(match.ExternalID)
	}
	cast := make([]db.CastCredit, 0, len(match.Cast))
	for _, member := range match.Cast {
		cast = append(cast, db.CastCredit{
			Name:        member.Name,
			Character:   member.Character,
			Order:       member.Order,
			ProfilePath: member.ProfilePath,
			Provider:    member.Provider,
			ProviderID:  member.ProviderID,
		})
	}
	settings := loadMetadataArtworkSettings(h.DB)
	posterPath := automaticMoviePosterSource(
		r.Context(),
		h.Artwork,
		settings,
		tmdbID,
		match.IMDbID,
		match.PosterURL,
		match.Provider,
	)
	canonical := db.CanonicalMetadata{
		Title:        match.Title,
		Overview:     match.Overview,
		PosterPath:   posterPath,
		BackdropPath: match.BackdropURL,
		ReleaseDate:  match.ReleaseDate,
		VoteAverage:  match.VoteAverage,
		IMDbID:       match.IMDbID,
		IMDbRating:   match.IMDbRating,
		Genres:       match.Genres,
		Cast:         cast,
		Runtime:      match.Runtime,
	}
	if err := updateMetadataWithRetry(
		h.DB,
		db.MediaTableForKind(db.LibraryTypeMovie),
		refID,
		match.Title,
		match.Overview,
		posterPath,
		match.BackdropURL,
		match.ReleaseDate,
		match.VoteAverage,
		match.IMDbID,
		match.IMDbRating,
		tmdbID,
		"",
		0,
		0,
		canonical,
		false,
		true,
		true,
	); err != nil {
		http.Error(w, "identify failed", http.StatusInternalServerError)
		return
	}
	if h.SearchIndex != nil {
		h.SearchIndex.Queue(libraryID, false)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(showActionResult{Updated: 1})
}

func (h *LibraryHandler) GetLibraryShowDetails(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	libraryID, _, _, _, ok := h.authorizeLibraryRequest(w, r, u.ID)
	if !ok {
		return
	}
	showKey := strings.TrimSpace(chi.URLParam(r, "showKey"))
	if showKey == "" {
		http.Error(w, "invalid show key", http.StatusBadRequest)
		return
	}
	details, err := db.GetLibraryShowDetails(h.DB, libraryID, showKey)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if details == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(details)
}

type showSeasonEpisodesResponse struct {
	SeasonNumber int                         `json:"seasonNumber"`
	Label        string                      `json:"label"`
	Episodes     []libraryBrowseItemResponse `json:"episodes"`
}

type showEpisodesResponse struct {
	IntroSkipMode string                       `json:"intro_skip_mode,omitempty"`
	Seasons       []showSeasonEpisodesResponse `json:"seasons"`
}

func (h *LibraryHandler) GetLibraryShowEpisodes(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	libraryID, _, _, _, ok := h.authorizeLibraryRequest(w, r, u.ID)
	if !ok {
		return
	}
	showKey := strings.TrimSpace(chi.URLParam(r, "showKey"))
	if showKey == "" {
		http.Error(w, "invalid show key", http.StatusBadRequest)
		return
	}
	items, err := db.GetLibraryShowEpisodesForUser(h.DB, libraryID, u.ID, showKey)
	if err != nil {
		if errors.Is(err, db.ErrShowNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if identifyStates := h.identifyRun.stateForLibrary(libraryID); len(identifyStates) > 0 {
		for i := range items {
			if state, ok := identifyStates[identifyRowKey(items[i].Type, items[i].Path)]; ok {
				items[i].IdentifyState = state
			}
		}
	}
	bySeason := make(map[int][]libraryBrowseItemResponse)
	for _, item := range items {
		s := item.Season
		bySeason[s] = append(bySeason[s], buildLibraryBrowseItemResponse(item))
	}
	seasonNums := make([]int, 0, len(bySeason))
	for s := range bySeason {
		seasonNums = append(seasonNums, s)
	}
	sort.Ints(seasonNums)
	seasons := make([]showSeasonEpisodesResponse, 0, len(seasonNums))
	for _, s := range seasonNums {
		label := "Specials"
		if s != 0 {
			label = "Season " + strconv.Itoa(s)
		}
		seasons = append(seasons, showSeasonEpisodesResponse{
			SeasonNumber: s,
			Label:        label,
			Episodes:     bySeason[s],
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(showEpisodesResponse{
		IntroSkipMode: db.GetLibraryIntroSkipMode(h.DB, libraryID),
		Seasons:       seasons,
	})
}

type refreshShowRequest struct {
	ShowKey string `json:"showKey"`
}

type confirmShowRequest struct {
	ShowKey string `json:"showKey"`
}

type identifyShowRequest struct {
	ShowKey    string `json:"showKey"`
	Provider   string `json:"provider"`
	ExternalID string `json:"externalId"`
	TmdbID     int    `json:"tmdbId"`
}

type identifyMovieRequest struct {
	MediaID    int    `json:"mediaId"`
	Provider   string `json:"provider"`
	ExternalID string `json:"externalId"`
	TmdbID     int    `json:"tmdbId"`
}

type showActionResult struct {
	Updated int `json:"updated"`
}

type updateMediaProgressRequest struct {
	PositionSeconds float64 `json:"position_seconds"`
	DurationSeconds float64 `json:"duration_seconds"`
	Completed       bool    `json:"completed"`
}

func (h *LibraryHandler) tryStartLibraryPlaybackRefresh(libraryID int) bool {
	h.playbackRefreshMu.Lock()
	defer h.playbackRefreshMu.Unlock()
	if h.playbackRefreshRunning == nil {
		h.playbackRefreshRunning = make(map[int]struct{})
	}
	if _, ok := h.playbackRefreshRunning[libraryID]; ok {
		return false
	}
	h.playbackRefreshRunning[libraryID] = struct{}{}
	return true
}

func (h *LibraryHandler) finishLibraryPlaybackRefresh(libraryID int) {
	h.playbackRefreshMu.Lock()
	defer h.playbackRefreshMu.Unlock()
	delete(h.playbackRefreshRunning, libraryID)
}

func (h *LibraryHandler) RefreshLibraryPlaybackTracks(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	idStr := chi.URLParam(r, "id")
	var libraryID, ownerID int
	err := h.DB.QueryRow(`SELECT id, user_id FROM libraries WHERE id = ?`, idStr).Scan(&libraryID, &ownerID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if ownerID != u.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !h.tryStartLibraryPlaybackRefresh(libraryID) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"accepted":  false,
			"libraryId": libraryID,
			"error":     "playback track refresh already running for this library",
		})
		return
	}
	go func(libID int) {
		defer h.finishLibraryPlaybackRefresh(libID)
		ctx := context.Background()
		refreshed, failed, runErr := db.RefreshPlaybackTrackMetadataForLibrary(ctx, h.DB, libID)
		if runErr != nil {
			log.Printf("library playback refresh library=%d: %v", libID, runErr)
			return
		}
		log.Printf("library playback refresh library=%d done refreshed=%d failed=%d", libID, refreshed, failed)
	}(libraryID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"accepted":  true,
		"libraryId": libraryID,
	})
}

func (h *LibraryHandler) RefreshShow(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	idStr := chi.URLParam(r, "id")
	var libraryID, ownerID int
	err := h.DB.QueryRow(`SELECT id, user_id FROM libraries WHERE id = ?`, idStr).Scan(&libraryID, &ownerID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if ownerID != u.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var payload refreshShowRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if payload.ShowKey == "" {
		http.Error(w, "showKey is required", http.StatusBadRequest)
		return
	}
	refs, err := db.ListShowEpisodeRefs(h.DB, libraryID, payload.ShowKey)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if len(refs) == 0 {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(showActionResult{Updated: 0})
		return
	}
	// Use first episode's TMDB ID (series id) for the show.
	seriesTMDBID := refs[0].TMDBID
	if h.SeriesQuery == nil || seriesTMDBID <= 0 {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(showActionResult{Updated: 0})
		return
	}
	updated, _ := h.applyTMDBSeriesToRefs(r.Context(), seriesTMDBID, refs, false, true)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(showActionResult{Updated: updated})
}

func (h *LibraryHandler) IdentifyShow(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	idStr := chi.URLParam(r, "id")
	var libraryID, ownerID int
	err := h.DB.QueryRow(`SELECT id, user_id FROM libraries WHERE id = ?`, idStr).Scan(&libraryID, &ownerID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if ownerID != u.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var payload identifyShowRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	payload.ShowKey = strings.TrimSpace(payload.ShowKey)
	payload.Provider = strings.ToLower(strings.TrimSpace(payload.Provider))
	payload.ExternalID = strings.TrimSpace(payload.ExternalID)
	if payload.Provider == "" && payload.TmdbID > 0 {
		payload.Provider = "tmdb"
	}
	if payload.ExternalID == "" && payload.TmdbID > 0 {
		payload.ExternalID = strconv.Itoa(payload.TmdbID)
	}
	if payload.ShowKey == "" || payload.Provider == "" || payload.ExternalID == "" {
		http.Error(w, "showKey, provider, and externalId are required", http.StatusBadRequest)
		return
	}
	refs, err := db.ListShowEpisodeRefs(h.DB, libraryID, payload.ShowKey)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if len(refs) == 0 {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(showActionResult{Updated: 0})
		return
	}
	if h.SeriesQuery == nil {
		http.Error(w, "metadata not configured", http.StatusServiceUnavailable)
		return
	}
	updated, _ := h.applySeriesMatchToRefs(r.Context(), payload.Provider, payload.ExternalID, refs, false, true)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(showActionResult{Updated: updated})
}

func (h *LibraryHandler) ConfirmShow(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	idStr := chi.URLParam(r, "id")
	var libraryID, ownerID int
	err := h.DB.QueryRow(`SELECT id, user_id FROM libraries WHERE id = ?`, idStr).Scan(&libraryID, &ownerID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if ownerID != u.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var payload confirmShowRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if payload.ShowKey == "" {
		http.Error(w, "showKey is required", http.StatusBadRequest)
		return
	}
	refs, err := db.ListShowEpisodeRefs(h.DB, libraryID, payload.ShowKey)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if len(refs) == 0 {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(showActionResult{Updated: 0})
		return
	}
	refIDs := make([]int, 0, len(refs))
	for _, ref := range refs {
		refIDs = append(refIDs, ref.RefID)
	}
	updated, err := db.UpdateShowMetadataState(h.DB, db.MediaTableForKind(refs[0].Kind), refIDs, false, true)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(showActionResult{Updated: updated})
}
