package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"plum/internal/db"
	"plum/internal/metadata"

	_ "modernc.org/sqlite"
)

func TestUpdateLibraryPlaybackPreferences(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(
		`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`,
		"test@test.com",
		"hash",
		now,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	var libraryID int
	if err := dbConn.QueryRow(
		`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		userID,
		"Anime",
		db.LibraryTypeAnime,
		"/anime",
		now,
	).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	handler := &LibraryHandler{DB: dbConn}
	req := httptest.NewRequest(
		http.MethodPut,
		"/api/libraries/"+strconv.Itoa(libraryID)+"/playback-preferences",
		strings.NewReader(`{"preferred_audio_language":"ja","preferred_subtitle_language":"en","subtitles_enabled_by_default":true,"intro_skip_mode":"manual"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.UpdateLibraryPlaybackPreferences(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		PreferredAudioLanguage    string `json:"preferred_audio_language"`
		PreferredSubtitleLanguage string `json:"preferred_subtitle_language"`
		SubtitlesEnabledByDefault bool   `json:"subtitles_enabled_by_default"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.PreferredAudioLanguage != "ja" {
		t.Fatalf("preferred_audio_language = %q", payload.PreferredAudioLanguage)
	}
	if payload.PreferredSubtitleLanguage != "en" {
		t.Fatalf("preferred_subtitle_language = %q", payload.PreferredSubtitleLanguage)
	}
	if !payload.SubtitlesEnabledByDefault {
		t.Fatalf("subtitles_enabled_by_default = false")
	}

	var (
		preferredAudio    sql.NullString
		preferredSubtitle sql.NullString
		subtitlesEnabled  sql.NullBool
	)
	if err := dbConn.QueryRow(
		`SELECT preferred_audio_language, preferred_subtitle_language, subtitles_enabled_by_default FROM libraries WHERE id = ?`,
		libraryID,
	).Scan(&preferredAudio, &preferredSubtitle, &subtitlesEnabled); err != nil {
		t.Fatalf("query library: %v", err)
	}
	if preferredAudio.String != "ja" || preferredSubtitle.String != "en" || !subtitlesEnabled.Bool {
		t.Fatalf("unexpected library prefs: audio=%q subtitle=%q enabled=%v", preferredAudio.String, preferredSubtitle.String, subtitlesEnabled.Bool)
	}
}

func TestUpdateLibraryPlaybackPreferences_PreservesAutomationWhenOmitted(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(
		`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`,
		"test@test.com",
		"hash",
		now,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	var libraryID int
	if err := dbConn.QueryRow(
		`INSERT INTO libraries (
			user_id, name, type, path, preferred_audio_language, preferred_subtitle_language,
			subtitles_enabled_by_default, watcher_enabled, watcher_mode, scan_interval_minutes, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		userID,
		"TV",
		db.LibraryTypeTV,
		"/tv",
		"en",
		"en",
		true,
		true,
		db.LibraryWatcherModePoll,
		15,
		now,
	).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	handler := &LibraryHandler{DB: dbConn}
	req := httptest.NewRequest(
		http.MethodPut,
		"/api/libraries/"+strconv.Itoa(libraryID)+"/playback-preferences",
		strings.NewReader(`{"preferred_audio_language":"ja","preferred_subtitle_language":"fr","subtitles_enabled_by_default":false}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.UpdateLibraryPlaybackPreferences(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		WatcherEnabled      bool   `json:"watcher_enabled"`
		WatcherMode         string `json:"watcher_mode"`
		ScanIntervalMinutes int    `json:"scan_interval_minutes"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !payload.WatcherEnabled || payload.WatcherMode != db.LibraryWatcherModePoll || payload.ScanIntervalMinutes != 15 {
		t.Fatalf("unexpected automation response: %+v", payload)
	}

	var (
		watcherEnabled bool
		watcherMode    string
		scanInterval   int
	)
	if err := dbConn.QueryRow(
		`SELECT watcher_enabled, watcher_mode, scan_interval_minutes FROM libraries WHERE id = ?`,
		libraryID,
	).Scan(&watcherEnabled, &watcherMode, &scanInterval); err != nil {
		t.Fatalf("query library automation: %v", err)
	}
	if !watcherEnabled || watcherMode != db.LibraryWatcherModePoll || scanInterval != 15 {
		t.Fatalf("unexpected library automation: enabled=%v mode=%q interval=%d", watcherEnabled, watcherMode, scanInterval)
	}
}

type identifyStub struct {
	tv          func(context.Context, metadata.MediaInfo) *metadata.MatchResult
	movie       func(context.Context, metadata.MediaInfo) *metadata.MatchResult
	movieResult func(context.Context, metadata.MediaInfo) (*metadata.MatchResult, error)
	anime       func(context.Context, metadata.MediaInfo) *metadata.MatchResult
}

type identifyMusicStub struct {
	identifyStub
	music func(context.Context, metadata.MusicInfo) *metadata.MusicMatchResult
}

type seriesQueryStub struct {
	searchTV   func(context.Context, string) ([]metadata.MatchResult, error)
	getEpisode func(context.Context, string, string, int, int) (*metadata.MatchResult, error)
}

type seriesDetailsStub struct {
	getSeriesDetails func(context.Context, int) (*metadata.SeriesDetails, error)
}

type discoverStub struct {
	getDiscover            func(context.Context, string) (*metadata.DiscoverResponse, error)
	getDiscoverGenres      func(context.Context) (*metadata.DiscoverGenresResponse, error)
	browseDiscover         func(context.Context, metadata.DiscoverBrowseCategory, metadata.DiscoverMediaType, int, int, string) (*metadata.DiscoverBrowseResponse, error)
	searchDiscover         func(context.Context, string) (*metadata.DiscoverSearchResponse, error)
	getDiscoverTitleDetail func(context.Context, metadata.DiscoverMediaType, int) (*metadata.DiscoverTitleDetails, error)
}

type nonRetryableNetError struct {
	msg string
}

func (s *seriesQueryStub) SearchTV(ctx context.Context, query string) ([]metadata.MatchResult, error) {
	if s.searchTV == nil {
		return nil, nil
	}
	return s.searchTV(ctx, query)
}

func (s *seriesQueryStub) GetEpisode(
	ctx context.Context,
	provider string,
	seriesID string,
	season int,
	episode int,
) (*metadata.MatchResult, error) {
	if s.getEpisode == nil {
		return nil, nil
	}
	return s.getEpisode(ctx, provider, seriesID, season, episode)
}

func (s *seriesDetailsStub) GetSeriesDetails(ctx context.Context, tmdbID int) (*metadata.SeriesDetails, error) {
	if s.getSeriesDetails == nil {
		return nil, nil
	}
	return s.getSeriesDetails(ctx, tmdbID)
}

func (s *discoverStub) GetDiscover(ctx context.Context, originCountry string) (*metadata.DiscoverResponse, error) {
	if s.getDiscover == nil {
		return nil, nil
	}
	return s.getDiscover(ctx, originCountry)
}

func (s *discoverStub) GetDiscoverGenres(ctx context.Context) (*metadata.DiscoverGenresResponse, error) {
	if s.getDiscoverGenres == nil {
		return nil, nil
	}
	return s.getDiscoverGenres(ctx)
}

func (s *discoverStub) BrowseDiscover(
	ctx context.Context,
	category metadata.DiscoverBrowseCategory,
	mediaType metadata.DiscoverMediaType,
	genreID int,
	page int,
	originCountry string,
) (*metadata.DiscoverBrowseResponse, error) {
	if s.browseDiscover == nil {
		return nil, nil
	}
	return s.browseDiscover(ctx, category, mediaType, genreID, page, originCountry)
}

func (s *discoverStub) SearchDiscover(ctx context.Context, query string) (*metadata.DiscoverSearchResponse, error) {
	if s.searchDiscover == nil {
		return nil, nil
	}
	return s.searchDiscover(ctx, query)
}

func (s *discoverStub) GetDiscoverTitleDetails(ctx context.Context, mediaType metadata.DiscoverMediaType, tmdbID int) (*metadata.DiscoverTitleDetails, error) {
	if s.getDiscoverTitleDetail == nil {
		return nil, nil
	}
	return s.getDiscoverTitleDetail(ctx, mediaType, tmdbID)
}

func (e nonRetryableNetError) Error() string   { return e.msg }
func (e nonRetryableNetError) Timeout() bool   { return false }
func (e nonRetryableNetError) Temporary() bool { return false }

func (s *identifyStub) IdentifyTV(ctx context.Context, info metadata.MediaInfo) *metadata.MatchResult {
	if s.tv == nil {
		return nil
	}
	return s.tv(ctx, info)
}

func (s *identifyStub) IdentifyAnime(ctx context.Context, info metadata.MediaInfo) *metadata.MatchResult {
	if s.anime == nil {
		return nil
	}
	return s.anime(ctx, info)
}

func (s *identifyStub) IdentifyMovie(ctx context.Context, info metadata.MediaInfo) *metadata.MatchResult {
	if s.movieResult != nil {
		result, _ := s.movieResult(ctx, info)
		return result
	}
	if s.movie == nil {
		return nil
	}
	return s.movie(ctx, info)
}

func (s *identifyStub) IdentifyMovieResult(
	ctx context.Context,
	info metadata.MediaInfo,
) (*metadata.MatchResult, error) {
	if s.movieResult != nil {
		return s.movieResult(ctx, info)
	}
	return s.IdentifyMovie(ctx, info), nil
}

func (s *identifyMusicStub) IdentifyMusic(
	ctx context.Context,
	info metadata.MusicInfo,
) *metadata.MusicMatchResult {
	if s.music == nil {
		return nil
	}
	return s.music(ctx, info)
}

func TestIdentifyLibrary_UsesRelativeMovieParsing(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "test@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "Movies", db.LibraryTypeMovie, "/movies", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	var movieID int
	path := "/movies/Die My Love (2025)/Die My Love 2025 BluRay 1080p DD 5 1 x264-BHDStudio.mp4"
	if err := dbConn.QueryRow(`INSERT INTO movies (library_id, title, path, duration, match_status) VALUES (?, ?, ?, ?, ?) RETURNING id`, libraryID, "Die My Love 2025 BluRay 1080p DD 5 1 x264-BHDStudio", path, 0, db.MatchStatusUnmatched).Scan(&movieID); err != nil {
		t.Fatalf("insert movie: %v", err)
	}

	handler := &LibraryHandler{
		DB: dbConn,
		Meta: &identifyStub{
			movie: func(_ context.Context, info metadata.MediaInfo) *metadata.MatchResult {
				if info.Title != "die my love" {
					t.Fatalf("title = %q", info.Title)
				}
				if info.Year != 2025 {
					t.Fatalf("year = %d", info.Year)
				}
				return &metadata.MatchResult{
					Title:      "Die My Love",
					Provider:   "tmdb",
					ExternalID: "456",
				}
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/libraries/"+strconv.Itoa(libraryID)+"/identify", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.IdentifyLibrary(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Identified int `json:"identified"`
		Failed     int `json:"failed"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Identified != 1 || payload.Failed != 0 {
		t.Fatalf("unexpected payload: %+v", payload)
	}

	var title, matchStatus string
	var tmdbID int
	if err := dbConn.QueryRow(`SELECT title, match_status, COALESCE(tmdb_id, 0) FROM movies WHERE id = ?`, movieID).Scan(&title, &matchStatus, &tmdbID); err != nil {
		t.Fatalf("query movie: %v", err)
	}
	if title != "Die My Love" {
		t.Fatalf("title = %q", title)
	}
	if matchStatus != db.MatchStatusIdentified {
		t.Fatalf("match_status = %q", matchStatus)
	}
	if tmdbID != 456 {
		t.Fatalf("tmdb_id = %d", tmdbID)
	}
}

func TestIdentifyLibrary_UsesAnimeIdentifier(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "test@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "Anime", db.LibraryTypeAnime, "/anime", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	var episodeID int
	path := "/anime/Frieren/Season 1/Frieren - S01E12.mkv"
	if err := dbConn.QueryRow(`INSERT INTO anime_episodes (library_id, title, path, duration, match_status, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`, libraryID, "Frieren - S01E12", path, 0, db.MatchStatusUnmatched, 1, 12).Scan(&episodeID); err != nil {
		t.Fatalf("insert anime episode: %v", err)
	}

	handler := &LibraryHandler{
		DB: dbConn,
		Meta: &identifyStub{
			anime: func(_ context.Context, info metadata.MediaInfo) *metadata.MatchResult {
				if info.Title != "frieren" {
					t.Fatalf("title = %q", info.Title)
				}
				if info.Season != 1 || info.Episode != 12 {
					t.Fatalf("unexpected episode info: %+v", info)
				}
				return &metadata.MatchResult{
					Title:      "Frieren - S01E12 - Episode",
					Provider:   "tmdb",
					ExternalID: "123",
				}
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/libraries/"+strconv.Itoa(libraryID)+"/identify", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.IdentifyLibrary(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var title, matchStatus string
	if err := dbConn.QueryRow(`SELECT title, match_status FROM anime_episodes WHERE id = ?`, episodeID).Scan(&title, &matchStatus); err != nil {
		t.Fatalf("query anime episode: %v", err)
	}
	if title != "Frieren - S01E12 - Episode" {
		t.Fatalf("title = %q", title)
	}
	if matchStatus != db.MatchStatusIdentified {
		t.Fatalf("match_status = %q", matchStatus)
	}
}

func TestIdentifyLibrary_UsesAnimeSearchFallbackAndAutoConfirmsExactMatch(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "test@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "Anime", db.LibraryTypeAnime, "/anime", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	var episodeID int
	path := "/anime/Frieren/Season 1/Frieren - S01E12.mkv"
	if err := dbConn.QueryRow(`INSERT INTO anime_episodes (library_id, title, path, duration, match_status, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`, libraryID, "Frieren - S01E12", path, 0, db.MatchStatusUnmatched, 1, 12).Scan(&episodeID); err != nil {
		t.Fatalf("insert anime episode: %v", err)
	}

	handler := &LibraryHandler{
		DB: dbConn,
		Meta: &identifyStub{
			anime: func(_ context.Context, _ metadata.MediaInfo) *metadata.MatchResult {
				return nil
			},
		},
		SeriesQuery: &seriesQueryStub{
			searchTV: func(_ context.Context, query string) ([]metadata.MatchResult, error) {
				if query != "Frieren" {
					t.Fatalf("query = %q", query)
				}
				return []metadata.MatchResult{
					{
						Title:      "Frieren",
						Provider:   "tmdb",
						ExternalID: "123",
					},
				}, nil
			},
			getEpisode: func(_ context.Context, provider, seriesID string, season, episode int) (*metadata.MatchResult, error) {
				if provider != "tmdb" || seriesID != "123" {
					t.Fatalf("unexpected provider/series = %q/%q", provider, seriesID)
				}
				if season != 1 || episode != 12 {
					t.Fatalf("unexpected episode = S%02dE%02d", season, episode)
				}
				return &metadata.MatchResult{
					Title:      "Frieren - S01E12 - Episode",
					Provider:   "tmdb",
					ExternalID: "123",
				}, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/libraries/"+strconv.Itoa(libraryID)+"/identify", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.IdentifyLibrary(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Identified int `json:"identified"`
		Failed     int `json:"failed"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Identified != 1 || payload.Failed != 0 {
		t.Fatalf("unexpected payload: %+v", payload)
	}

	var title, matchStatus string
	var tmdbID int
	var reviewNeeded bool
	if err := dbConn.QueryRow(`SELECT title, match_status, COALESCE(tmdb_id, 0), COALESCE(metadata_review_needed, 0) FROM anime_episodes WHERE id = ?`, episodeID).Scan(&title, &matchStatus, &tmdbID, &reviewNeeded); err != nil {
		t.Fatalf("query anime episode: %v", err)
	}
	if title != "Frieren - S01E12 - Episode" {
		t.Fatalf("title = %q", title)
	}
	if matchStatus != db.MatchStatusIdentified {
		t.Fatalf("match_status = %q", matchStatus)
	}
	if tmdbID != 123 {
		t.Fatalf("tmdb_id = %d", tmdbID)
	}
	if reviewNeeded {
		t.Fatal("expected metadata_review_needed to be false")
	}
}

func TestIdentifyLibrary_UsesTVSearchFallbackAndAutoConfirmsExactMatch(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "test@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "TV", db.LibraryTypeTV, "/tv", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	var episodeID int
	path := "/tv/Slow Horses/Season 1/Slow Horses - S01E01.mkv"
	if err := dbConn.QueryRow(`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`, libraryID, "Slow Horses - S01E01", path, 0, db.MatchStatusUnmatched, 1, 1).Scan(&episodeID); err != nil {
		t.Fatalf("insert tv episode: %v", err)
	}

	handler := &LibraryHandler{
		DB: dbConn,
		Meta: &identifyStub{
			tv: func(_ context.Context, _ metadata.MediaInfo) *metadata.MatchResult {
				return nil
			},
		},
		SeriesQuery: &seriesQueryStub{
			searchTV: func(_ context.Context, query string) ([]metadata.MatchResult, error) {
				if query != "Slow Horses" {
					t.Fatalf("query = %q", query)
				}
				return []metadata.MatchResult{
					{
						Title:      "Slow Horses",
						Provider:   "tmdb",
						ExternalID: "321",
					},
				}, nil
			},
			getEpisode: func(_ context.Context, provider, seriesID string, season, episode int) (*metadata.MatchResult, error) {
				if provider != "tmdb" || seriesID != "321" {
					t.Fatalf("unexpected provider/series = %q/%q", provider, seriesID)
				}
				if season != 1 || episode != 1 {
					t.Fatalf("unexpected episode = S%02dE%02d", season, episode)
				}
				return &metadata.MatchResult{
					Title:      "Slow Horses - S01E01 - Failure's Contagious",
					Provider:   "tmdb",
					ExternalID: "321",
				}, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/libraries/"+strconv.Itoa(libraryID)+"/identify", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.IdentifyLibrary(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Identified int `json:"identified"`
		Failed     int `json:"failed"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Identified != 1 || payload.Failed != 0 {
		t.Fatalf("unexpected payload: %+v", payload)
	}

	var title, matchStatus string
	var tmdbID int
	var reviewNeeded bool
	if err := dbConn.QueryRow(`SELECT title, match_status, COALESCE(tmdb_id, 0), COALESCE(metadata_review_needed, 0) FROM tv_episodes WHERE id = ?`, episodeID).Scan(&title, &matchStatus, &tmdbID, &reviewNeeded); err != nil {
		t.Fatalf("query tv episode: %v", err)
	}
	if title != "Slow Horses - S01E01 - Failure's Contagious" {
		t.Fatalf("title = %q", title)
	}
	if matchStatus != db.MatchStatusIdentified {
		t.Fatalf("match_status = %q", matchStatus)
	}
	if tmdbID != 321 {
		t.Fatalf("tmdb_id = %d", tmdbID)
	}
	if reviewNeeded {
		t.Fatal("expected metadata_review_needed to be false")
	}
}

func TestIdentifyLibrary_UsesTVSearchFallbackAndMarksAmbiguousMatchForReview(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "ambiguous@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "TV", db.LibraryTypeTV, "/tv", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	var episodeID int
	path := "/tv/Slow Horses/Season 1/Slow Horses - S01E01.mkv"
	if err := dbConn.QueryRow(`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`, libraryID, "Slow Horses - S01E01", path, 0, db.MatchStatusUnmatched, 1, 1).Scan(&episodeID); err != nil {
		t.Fatalf("insert tv episode: %v", err)
	}

	handler := &LibraryHandler{
		DB: dbConn,
		Meta: &identifyStub{
			tv: func(_ context.Context, _ metadata.MediaInfo) *metadata.MatchResult {
				return nil
			},
		},
		SeriesQuery: &seriesQueryStub{
			searchTV: func(_ context.Context, query string) ([]metadata.MatchResult, error) {
				if query != "Slow Horses" {
					t.Fatalf("query = %q", query)
				}
				return []metadata.MatchResult{
					{
						Title:      "Slow Horses",
						Provider:   "tmdb",
						ExternalID: "321",
					},
					{
						Title:      "Slow Horses",
						Provider:   "tmdb",
						ExternalID: "654",
					},
				}, nil
			},
			getEpisode: func(_ context.Context, provider, seriesID string, season, episode int) (*metadata.MatchResult, error) {
				if provider != "tmdb" {
					t.Fatalf("unexpected provider = %q", provider)
				}
				if seriesID != "321" && seriesID != "654" {
					t.Fatalf("unexpected series id = %q", seriesID)
				}
				if season != 1 || episode != 1 {
					t.Fatalf("unexpected episode = S%02dE%02d", season, episode)
				}
				return &metadata.MatchResult{
					Title:      "Slow Horses - S01E01 - Failure's Contagious",
					Provider:   "tmdb",
					ExternalID: seriesID,
				}, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/libraries/"+strconv.Itoa(libraryID)+"/identify", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.IdentifyLibrary(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Identified int `json:"identified"`
		Failed     int `json:"failed"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Identified != 1 || payload.Failed != 0 {
		t.Fatalf("unexpected payload: %+v", payload)
	}

	var title, matchStatus string
	var tmdbID int
	var reviewNeeded bool
	if err := dbConn.QueryRow(`SELECT title, match_status, COALESCE(tmdb_id, 0), COALESCE(metadata_review_needed, 0) FROM tv_episodes WHERE id = ?`, episodeID).Scan(&title, &matchStatus, &tmdbID, &reviewNeeded); err != nil {
		t.Fatalf("query tv episode: %v", err)
	}
	if title != "Slow Horses - S01E01 - Failure's Contagious" {
		t.Fatalf("title = %q", title)
	}
	if matchStatus != db.MatchStatusIdentified {
		t.Fatalf("match_status = %q", matchStatus)
	}
	if tmdbID != 321 {
		t.Fatalf("tmdb_id = %d", tmdbID)
	}
	if !reviewNeeded {
		t.Fatal("expected metadata_review_needed to be true")
	}
}

func TestIdentifyLibrary_AnimeSearchFallbackPrefersShowTitleFromEpisodeTitle(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "test@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "Anime", db.LibraryTypeAnime, "/anime", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	var episodeID int
	path := "/anime/Fallback Folder/Season 1/Episode 12.mkv"
	if err := dbConn.QueryRow(`INSERT INTO anime_episodes (library_id, title, path, duration, match_status, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`, libraryID, "Correct Show - S01E12", path, 0, db.MatchStatusUnmatched, 1, 12).Scan(&episodeID); err != nil {
		t.Fatalf("insert anime episode: %v", err)
	}
	_ = episodeID

	searchCalls := make([]string, 0, 2)
	handler := &LibraryHandler{
		DB: dbConn,
		Meta: &identifyStub{
			anime: func(_ context.Context, _ metadata.MediaInfo) *metadata.MatchResult {
				return nil
			},
		},
		SeriesQuery: &seriesQueryStub{
			searchTV: func(_ context.Context, query string) ([]metadata.MatchResult, error) {
				searchCalls = append(searchCalls, query)
				if query != "Correct Show" {
					return nil, nil
				}
				return []metadata.MatchResult{{Title: "Correct Show", Provider: "tmdb", ExternalID: "123"}}, nil
			},
			getEpisode: func(_ context.Context, provider, seriesID string, season, episode int) (*metadata.MatchResult, error) {
				return &metadata.MatchResult{
					Title:      "Correct Show - S01E12 - Episode",
					Provider:   "tmdb",
					ExternalID: "123",
				}, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/libraries/"+strconv.Itoa(libraryID)+"/identify", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.IdentifyLibrary(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(searchCalls) == 0 || searchCalls[0] != "Correct Show" {
		t.Fatalf("unexpected search calls: %#v", searchCalls)
	}
}

func TestIdentifyShow_ClearsMetadataReviewNeeded(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "test@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "Anime", db.LibraryTypeAnime, "/anime", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	var episodeID int
	if err := dbConn.QueryRow(`INSERT INTO anime_episodes (library_id, title, path, duration, match_status, tmdb_id, metadata_review_needed, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		libraryID,
		"Frieren - S01E12 - Episode",
		"/anime/Frieren/Season 1/Frieren - S01E12.mkv",
		0,
		db.MatchStatusIdentified,
		123,
		true,
		1,
		12,
	).Scan(&episodeID); err != nil {
		t.Fatalf("insert anime episode: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO media_global (kind, ref_id) VALUES (?, ?)`, db.LibraryTypeAnime, episodeID); err != nil {
		t.Fatalf("insert media global row: %v", err)
	}

	handler := &LibraryHandler{
		DB: dbConn,
		SeriesQuery: &seriesQueryStub{
			getEpisode: func(_ context.Context, provider, seriesID string, season, episode int) (*metadata.MatchResult, error) {
				if provider != "tmdb" || seriesID != "456" {
					t.Fatalf("unexpected provider/series = %q/%q", provider, seriesID)
				}
				return &metadata.MatchResult{
					Title:      "Frieren - S01E12 - Episode",
					Provider:   "tmdb",
					ExternalID: "456",
				}, nil
			},
		},
	}

	body := strings.NewReader(`{"showKey":"tmdb-123","provider":"tmdb","externalId":"456","tmdbId":456}`)
	req := httptest.NewRequest(http.MethodPost, "/api/libraries/"+strconv.Itoa(libraryID)+"/shows/identify", body)
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.IdentifyShow(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var reviewNeeded bool
	var metadataConfirmed bool
	var tmdbID int
	if err := dbConn.QueryRow(`SELECT COALESCE(metadata_review_needed, 0), COALESCE(metadata_confirmed, 0), COALESCE(tmdb_id, 0) FROM anime_episodes WHERE id = ?`, episodeID).Scan(&reviewNeeded, &metadataConfirmed, &tmdbID); err != nil {
		t.Fatalf("query anime episode: %v", err)
	}
	if reviewNeeded {
		t.Fatal("expected metadata_review_needed to be cleared")
	}
	if !metadataConfirmed {
		t.Fatal("expected metadata_confirmed to be set")
	}
	if tmdbID != 456 {
		t.Fatalf("tmdb_id = %d", tmdbID)
	}
}

func TestIdentifyShow_UsesTitleShowKeyForUnmatchedRows(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "test@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "TV", db.LibraryTypeTV, "/tv", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	var episodeID int
	if err := dbConn.QueryRow(`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		libraryID,
		"Missing Show (2024) - S01E01 - Pilot",
		"/tv/Missing Show (2024)/Season 1/Missing Show (2024) - S01E01.mkv",
		0,
		db.MatchStatusUnmatched,
		1,
		1,
	).Scan(&episodeID); err != nil {
		t.Fatalf("insert tv episode: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO media_global (kind, ref_id) VALUES (?, ?)`, db.LibraryTypeTV, episodeID); err != nil {
		t.Fatalf("insert media global row: %v", err)
	}

	handler := &LibraryHandler{
		DB: dbConn,
		SeriesQuery: &seriesQueryStub{
			getEpisode: func(_ context.Context, provider, seriesID string, season, episode int) (*metadata.MatchResult, error) {
				if provider != "tmdb" || seriesID != "456" {
					t.Fatalf("unexpected provider/series = %q/%q", provider, seriesID)
				}
				if season != 1 || episode != 1 {
					t.Fatalf("unexpected episode = S%02dE%02d", season, episode)
				}
				return &metadata.MatchResult{
					Title:      "Missing Show - S01E01 - Pilot",
					Provider:   "tmdb",
					ExternalID: "456",
				}, nil
			},
		},
	}

	body := strings.NewReader(`{"showKey":"title-missingshow2024","provider":"tmdb","externalId":"456","tmdbId":456}`)
	req := httptest.NewRequest(http.MethodPost, "/api/libraries/"+strconv.Itoa(libraryID)+"/shows/identify", body)
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.IdentifyShow(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Updated int `json:"updated"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Updated != 1 {
		t.Fatalf("updated = %d", payload.Updated)
	}
	var tmdbID int
	var metadataConfirmed bool
	if err := dbConn.QueryRow(`SELECT COALESCE(tmdb_id, 0), COALESCE(metadata_confirmed, 0) FROM tv_episodes WHERE id = ?`, episodeID).Scan(&tmdbID, &metadataConfirmed); err != nil {
		t.Fatalf("query tv episode: %v", err)
	}
	if tmdbID != 456 {
		t.Fatalf("tmdb_id = %d", tmdbID)
	}
	if !metadataConfirmed {
		t.Fatal("expected metadata_confirmed to be set")
	}
}

func TestIdentifyShow_OnlyUpdatesEpisodesForMatchingYearQualifiedShowKey(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "test@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "TV", db.LibraryTypeTV, "/tv", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	insertEpisode := func(title, path string) int {
		var episodeID int
		if err := dbConn.QueryRow(`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			libraryID,
			title,
			path,
			0,
			db.MatchStatusUnmatched,
			1,
			1,
		).Scan(&episodeID); err != nil {
			t.Fatalf("insert tv episode %q: %v", title, err)
		}
		if _, err := dbConn.Exec(`INSERT INTO media_global (kind, ref_id) VALUES (?, ?)`, db.LibraryTypeTV, episodeID); err != nil {
			t.Fatalf("insert media global row for %q: %v", title, err)
		}
		return episodeID
	}

	episode1978ID := insertEpisode(
		"Battlestar Galactica (1978) - S01E01 - Saga of a Star World",
		"/tv/Battlestar Galactica (1978)/Season 1/Battlestar Galactica (1978) - S01E01.mkv",
	)
	episode2004ID := insertEpisode(
		"Battlestar Galactica (2004) - S01E01 - 33",
		"/tv/Battlestar Galactica (2004)/Season 1/Battlestar Galactica (2004) - S01E01.mkv",
	)

	handler := &LibraryHandler{
		DB: dbConn,
		SeriesQuery: &seriesQueryStub{
			getEpisode: func(_ context.Context, provider, seriesID string, season, episode int) (*metadata.MatchResult, error) {
				if provider != "tmdb" || seriesID != "456" {
					t.Fatalf("unexpected provider/series = %q/%q", provider, seriesID)
				}
				if season != 1 || episode != 1 {
					t.Fatalf("unexpected episode = S%02dE%02d", season, episode)
				}
				return &metadata.MatchResult{
					Title:      "Battlestar Galactica - S01E01 - Saga of a Star World",
					Provider:   "tmdb",
					ExternalID: "456",
				}, nil
			},
		},
	}

	body := strings.NewReader(`{"showKey":"title-battlestargalactica1978","provider":"tmdb","externalId":"456","tmdbId":456}`)
	req := httptest.NewRequest(http.MethodPost, "/api/libraries/"+strconv.Itoa(libraryID)+"/shows/identify", body)
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.IdentifyShow(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var tmdbID1978 int
	if err := dbConn.QueryRow(`SELECT COALESCE(tmdb_id, 0) FROM tv_episodes WHERE id = ?`, episode1978ID).Scan(&tmdbID1978); err != nil {
		t.Fatalf("query 1978 episode: %v", err)
	}
	if tmdbID1978 != 456 {
		t.Fatalf("1978 tmdb_id = %d", tmdbID1978)
	}

	var tmdbID2004 int
	if err := dbConn.QueryRow(`SELECT COALESCE(tmdb_id, 0) FROM tv_episodes WHERE id = ?`, episode2004ID).Scan(&tmdbID2004); err != nil {
		t.Fatalf("query 2004 episode: %v", err)
	}
	if tmdbID2004 != 0 {
		t.Fatalf("expected 2004 tmdb_id to remain unset, got %d", tmdbID2004)
	}
}

func TestIdentifyShow_UsesSelectedTVDBSeries(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "test@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "TV", db.LibraryTypeTV, "/tv", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	var episodeID int
	if err := dbConn.QueryRow(`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		libraryID,
		"Black Spot (Zone Blanche) - S01E01 - Episode",
		"/tv/Black Spot (Zone Blanche)/Season 1/Black Spot (Zone Blanche) - S01E01.mkv",
		0,
		db.MatchStatusUnmatched,
		1,
		1,
	).Scan(&episodeID); err != nil {
		t.Fatalf("insert tv episode: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO media_global (kind, ref_id) VALUES (?, ?)`, db.LibraryTypeTV, episodeID); err != nil {
		t.Fatalf("insert media global row: %v", err)
	}

	handler := &LibraryHandler{
		DB: dbConn,
		SeriesQuery: &seriesQueryStub{
			getEpisode: func(_ context.Context, provider, seriesID string, season, episode int) (*metadata.MatchResult, error) {
				if provider != "tvdb" || seriesID != "36647" {
					t.Fatalf("unexpected provider/series = %q/%q", provider, seriesID)
				}
				if season != 1 || episode != 1 {
					t.Fatalf("unexpected episode = S%02dE%02d", season, episode)
				}
				return &metadata.MatchResult{
					Title:       "Black Spot (Zone Blanche) - S01E01 - Episode",
					Overview:    "Pilot",
					Provider:    "tvdb",
					ExternalID:  "36647",
					ReleaseDate: "2017-04-10",
				}, nil
			},
		},
	}

	body := strings.NewReader(`{"showKey":"title-blackspotzoneblanche","provider":"tvdb","externalId":"36647"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/libraries/"+strconv.Itoa(libraryID)+"/shows/identify", body)
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.IdentifyShow(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Updated int `json:"updated"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Updated != 1 {
		t.Fatalf("updated = %d", payload.Updated)
	}
	var storedTVDBID string
	var metadataConfirmed bool
	if err := dbConn.QueryRow(`SELECT COALESCE(tvdb_id, ''), COALESCE(metadata_confirmed, 0) FROM tv_episodes WHERE id = ?`, episodeID).Scan(&storedTVDBID, &metadataConfirmed); err != nil {
		t.Fatalf("query tv episode: %v", err)
	}
	if storedTVDBID != "36647" {
		t.Fatalf("tvdb_id = %q", storedTVDBID)
	}
	if !metadataConfirmed {
		t.Fatal("expected metadata_confirmed to be set")
	}
}

// Release-folder junk like " - Sno" before the real " - S01E01" must not change the show key vs the web
// app (showGrouping.ts getShowName); otherwise manual identify sends a key ListShowEpisodeRefs never matches.
func TestIdentifyShow_MatchesFrontendShowKeyWhenHyphenSAppearsBeforeSeasonMarker(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "test@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "TV", db.LibraryTypeTV, "/tv", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	epTitle := "Black Spot (Zone Blanche) S01 - Hardcoded Eng Subs - Sno - S01E01 - Pilot"
	var episodeID int
	if err := dbConn.QueryRow(`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		libraryID,
		epTitle,
		"/tv/Black Spot (Zone Blanche) S01 - Hardcoded Eng Subs - Sno/S01E01.mkv",
		0,
		db.MatchStatusUnmatched,
		1,
		1,
	).Scan(&episodeID); err != nil {
		t.Fatalf("insert tv episode: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO media_global (kind, ref_id) VALUES (?, ?)`, db.LibraryTypeTV, episodeID); err != nil {
		t.Fatalf("insert media global row: %v", err)
	}

	handler := &LibraryHandler{
		DB: dbConn,
		SeriesQuery: &seriesQueryStub{
			getEpisode: func(_ context.Context, provider, seriesID string, season, episode int) (*metadata.MatchResult, error) {
				if provider != "tmdb" || seriesID != "789" {
					t.Fatalf("unexpected provider/series = %q/%q", provider, seriesID)
				}
				if season != 1 || episode != 1 {
					t.Fatalf("unexpected episode = S%02dE%02d", season, episode)
				}
				return &metadata.MatchResult{
					Title:      "Black Spot (Zone Blanche) - S01E01 - Pilot",
					Provider:   "tmdb",
					ExternalID: "789",
				}, nil
			},
		},
	}

	// Same key groupMediaByShow / getShowKey would produce for this title.
	showKey := "title-blackspotzoneblanches01hardcodedengsubssno"
	body := strings.NewReader(`{"showKey":"` + showKey + `","provider":"tmdb","externalId":"789","tmdbId":789}`)
	req := httptest.NewRequest(http.MethodPost, "/api/libraries/"+strconv.Itoa(libraryID)+"/shows/identify", body)
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.IdentifyShow(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Updated int `json:"updated"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Updated != 1 {
		t.Fatalf("updated = %d (ListShowEpisodeRefs show key likely out of sync with web)", payload.Updated)
	}
}

func TestIdentifyShow_UsesSeriesMetadataWhenEpisodeLookupFails(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "test@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "TV", db.LibraryTypeTV, "/tv", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	var episodeID int
	if err := dbConn.QueryRow(`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		libraryID,
		"Kfulim (False Flag) - S01E01 - Episode 1",
		"/tv/Kfulim (False Flag)/Season 1/Kfulim (False Flag) - S01E01.mkv",
		0,
		db.MatchStatusUnmatched,
		1,
		1,
	).Scan(&episodeID); err != nil {
		t.Fatalf("insert tv episode: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO media_global (kind, ref_id) VALUES (?, ?)`, db.LibraryTypeTV, episodeID); err != nil {
		t.Fatalf("insert media global row: %v", err)
	}

	handler := &LibraryHandler{
		DB: dbConn,
		Series: &seriesDetailsStub{
			getSeriesDetails: func(_ context.Context, tmdbID int) (*metadata.SeriesDetails, error) {
				if tmdbID != 456 {
					t.Fatalf("tmdbID = %d", tmdbID)
				}
				return &metadata.SeriesDetails{
					Name:         "False Flag",
					Overview:     "Series overview",
					PosterPath:   "series poster",
					BackdropPath: "series backdrop",
					FirstAirDate: "2015-10-29",
				}, nil
			},
		},
		SeriesQuery: &seriesQueryStub{
			getEpisode: func(_ context.Context, provider, seriesID string, season, episode int) (*metadata.MatchResult, error) {
				if provider != "tmdb" || seriesID != "456" {
					t.Fatalf("unexpected provider/series = %q/%q", provider, seriesID)
				}
				return nil, nil
			},
		},
	}

	body := strings.NewReader(`{"showKey":"title-kfulimfalseflag","provider":"tmdb","externalId":"456","tmdbId":456}`)
	req := httptest.NewRequest(http.MethodPost, "/api/libraries/"+strconv.Itoa(libraryID)+"/shows/identify", body)
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.IdentifyShow(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Updated int `json:"updated"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Updated != 1 {
		t.Fatalf("updated = %d", payload.Updated)
	}

	var title string
	var tmdbID int
	var metadataConfirmed bool
	var showID int
	if err := dbConn.QueryRow(`SELECT title, COALESCE(tmdb_id, 0), COALESCE(metadata_confirmed, 0), COALESCE(show_id, 0) FROM tv_episodes WHERE id = ?`, episodeID).
		Scan(&title, &tmdbID, &metadataConfirmed, &showID); err != nil {
		t.Fatalf("query tv episode: %v", err)
	}
	if title != "Kfulim (False Flag) - S01E01 - Episode 1" {
		t.Fatalf("title = %q", title)
	}
	if tmdbID != 456 {
		t.Fatalf("tmdb_id = %d", tmdbID)
	}
	if !metadataConfirmed {
		t.Fatal("expected metadata_confirmed to be set")
	}
	if showID == 0 {
		t.Fatal("expected show link to be created")
	}

	var showTitle string
	if err := dbConn.QueryRow(`SELECT title FROM shows WHERE id = ?`, showID).Scan(&showTitle); err != nil {
		t.Fatalf("query show row: %v", err)
	}
	if showTitle != "False Flag" {
		t.Fatalf("show title = %q", showTitle)
	}
}

func TestRefreshShow_UsesSeriesDetailsForCanonicalMetadata(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "test@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "TV", db.LibraryTypeTV, "/tv", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	var episodeID int
	if err := dbConn.QueryRow(`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, tmdb_id, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		libraryID,
		"Series Name - S01E01 - Episode One",
		"/tv/Series Name/Season 1/Series Name - S01E01.mkv",
		0,
		db.MatchStatusUnmatched,
		456,
		1,
		1,
	).Scan(&episodeID); err != nil {
		t.Fatalf("insert tv episode: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO media_global (kind, ref_id) VALUES (?, ?)`, db.LibraryTypeTV, episodeID); err != nil {
		t.Fatalf("insert media global row: %v", err)
	}

	handler := &LibraryHandler{
		DB: dbConn,
		Series: &seriesDetailsStub{
			getSeriesDetails: func(_ context.Context, tmdbID int) (*metadata.SeriesDetails, error) {
				if tmdbID != 456 {
					t.Fatalf("tmdbID = %d", tmdbID)
				}
				return &metadata.SeriesDetails{
					Name:         "Series Name",
					Overview:     "series overview",
					PosterPath:   "series poster",
					BackdropPath: "series backdrop",
					FirstAirDate: "2024-01-01",
					IMDbID:       "ttseries",
				}, nil
			},
		},
		SeriesQuery: &seriesQueryStub{
			getEpisode: func(_ context.Context, provider, seriesID string, season, episode int) (*metadata.MatchResult, error) {
				if provider != "tmdb" || seriesID != "456" {
					t.Fatalf("unexpected provider/series = %q/%q", provider, seriesID)
				}
				if season != 1 || episode != 1 {
					t.Fatalf("unexpected episode = S%02dE%02d", season, episode)
				}
				return &metadata.MatchResult{
					Title:       "Series Name - S01E01 - Episode One",
					Overview:    "episode overview",
					PosterURL:   "episode poster",
					BackdropURL: "episode backdrop",
					ReleaseDate: "2024-01-02",
					VoteAverage: 7.5,
					IMDbID:      "ttepisode",
					IMDbRating:  8.1,
					Provider:    "tmdb",
					ExternalID:  "456",
				}, nil
			},
		},
	}

	body := strings.NewReader(`{"showKey":"tmdb-456"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/libraries/"+strconv.Itoa(libraryID)+"/shows/refresh", body)
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.RefreshShow(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var showID, seasonID int
	if err := dbConn.QueryRow(`SELECT COALESCE(show_id, 0), COALESCE(season_id, 0) FROM tv_episodes WHERE id = ?`, episodeID).Scan(&showID, &seasonID); err != nil {
		t.Fatalf("query episode links: %v", err)
	}
	if showID == 0 || seasonID == 0 {
		t.Fatalf("expected show/season links, got show=%d season=%d", showID, seasonID)
	}

	var showTitle, showOverview, showPoster, showBackdrop, showFirstAir, showIMDbID string
	if err := dbConn.QueryRow(`SELECT title, COALESCE(overview, ''), COALESCE(poster_path, ''), COALESCE(backdrop_path, ''), COALESCE(first_air_date, ''), COALESCE(imdb_id, '') FROM shows WHERE id = ?`, showID).
		Scan(&showTitle, &showOverview, &showPoster, &showBackdrop, &showFirstAir, &showIMDbID); err != nil {
		t.Fatalf("query show row: %v", err)
	}
	if showTitle != "Series Name" {
		t.Fatalf("show title = %q", showTitle)
	}
	if showOverview != "series overview" || showPoster != "series poster" || showBackdrop != "series backdrop" || showFirstAir != "2024-01-01" || showIMDbID != "ttseries" {
		t.Fatalf("unexpected show metadata: overview=%q poster=%q backdrop=%q first_air=%q imdb=%q", showOverview, showPoster, showBackdrop, showFirstAir, showIMDbID)
	}

	var seasonTitle, seasonOverview, seasonPoster, seasonAir string
	if err := dbConn.QueryRow(`SELECT title, COALESCE(overview, ''), COALESCE(poster_path, ''), COALESCE(air_date, '') FROM seasons WHERE id = ?`, seasonID).
		Scan(&seasonTitle, &seasonOverview, &seasonPoster, &seasonAir); err != nil {
		t.Fatalf("query season row: %v", err)
	}
	if seasonTitle != "Season 1" {
		t.Fatalf("season title = %q", seasonTitle)
	}
	if seasonOverview != "series overview" || seasonPoster != "series poster" || seasonAir != "2024-01-01" {
		t.Fatalf("unexpected season metadata: overview=%q poster=%q air=%q", seasonOverview, seasonPoster, seasonAir)
	}

	var episodeTitle, episodeOverview, episodePoster, episodeBackdrop, releaseDate string
	if err := dbConn.QueryRow(`SELECT title, COALESCE(overview, ''), COALESCE(poster_path, ''), COALESCE(backdrop_path, ''), COALESCE(release_date, '') FROM tv_episodes WHERE id = ?`, episodeID).
		Scan(&episodeTitle, &episodeOverview, &episodePoster, &episodeBackdrop, &releaseDate); err != nil {
		t.Fatalf("query episode row: %v", err)
	}
	if episodeTitle != "Series Name - S01E01 - Episode One" {
		t.Fatalf("episode title = %q", episodeTitle)
	}
	if episodeOverview != "episode overview" || episodePoster != "episode poster" || episodeBackdrop != "episode backdrop" || releaseDate != "2024-01-02" {
		t.Fatalf("unexpected episode metadata: overview=%q poster=%q backdrop=%q release=%q", episodeOverview, episodePoster, episodeBackdrop, releaseDate)
	}
}

func TestConfirmShow_ClearsMetadataReviewNeeded(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "test@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "Anime", db.LibraryTypeAnime, "/anime", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	var episodeID int
	if err := dbConn.QueryRow(`INSERT INTO anime_episodes (library_id, title, path, duration, match_status, tmdb_id, metadata_review_needed, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		libraryID,
		"Frieren - S01E12 - Episode",
		"/anime/Frieren/Season 1/Frieren - S01E12.mkv",
		0,
		db.MatchStatusIdentified,
		123,
		true,
		1,
		12,
	).Scan(&episodeID); err != nil {
		t.Fatalf("insert anime episode: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO media_global (kind, ref_id) VALUES (?, ?)`, db.LibraryTypeAnime, episodeID); err != nil {
		t.Fatalf("insert media global row: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/libraries/"+strconv.Itoa(libraryID)+"/shows/confirm", strings.NewReader(`{"showKey":"tmdb-123"}`))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler := &LibraryHandler{DB: dbConn}
	handler.ConfirmShow(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Updated int `json:"updated"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Updated != 1 {
		t.Fatalf("updated = %d", payload.Updated)
	}
	var reviewNeeded bool
	var metadataConfirmed bool
	if err := dbConn.QueryRow(`SELECT COALESCE(metadata_review_needed, 0), COALESCE(metadata_confirmed, 0) FROM anime_episodes WHERE id = ?`, episodeID).Scan(&reviewNeeded, &metadataConfirmed); err != nil {
		t.Fatalf("query anime episode: %v", err)
	}
	if reviewNeeded {
		t.Fatal("expected metadata_review_needed to be cleared")
	}
	if !metadataConfirmed {
		t.Fatal("expected metadata_confirmed to be set")
	}
}

func TestIdentifyLibrary_TimesOutHungRowsAndSkipsFinishedRows(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	prevInitialTimeout := identifyInitialTimeout
	prevRetryTimeout := identifyRetryTimeout
	prevWorkers := identifyMovieWorkers
	prevRateInterval := identifyMovieRateLimit
	prevRateBurst := identifyMovieRateBurst
	identifyInitialTimeout = 10 * time.Millisecond
	identifyRetryTimeout = 25 * time.Millisecond
	identifyMovieWorkers = 2
	identifyMovieRateLimit = time.Millisecond
	identifyMovieRateBurst = 1
	t.Cleanup(func() {
		identifyInitialTimeout = prevInitialTimeout
		identifyRetryTimeout = prevRetryTimeout
		identifyMovieWorkers = prevWorkers
		identifyMovieRateLimit = prevRateInterval
		identifyMovieRateBurst = prevRateBurst
	})

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "test@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "Movies", db.LibraryTypeMovie, "/movies", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	stuckPath := "/movies/Stuck Movie (2024)/Stuck Movie (2024).mp4"
	var ignoredID int
	if err := dbConn.QueryRow(`INSERT INTO movies (library_id, title, path, duration, match_status) VALUES (?, ?, ?, ?, ?) RETURNING id`, libraryID, "Stuck Movie", stuckPath, 0, db.MatchStatusUnmatched).Scan(&ignoredID); err != nil {
		t.Fatalf("insert stuck movie: %v", err)
	}
	var quickID int
	quickPath := "/movies/Quick Movie (2024)/Quick Movie (2024).mp4"
	if err := dbConn.QueryRow(`INSERT INTO movies (library_id, title, path, duration, match_status) VALUES (?, ?, ?, ?, ?) RETURNING id`, libraryID, "Quick Movie", quickPath, 0, db.MatchStatusLocal).Scan(&quickID); err != nil {
		t.Fatalf("insert quick movie: %v", err)
	}
	var finishedID int
	finishedPath := "/movies/Finished Movie (2024)/Finished Movie (2024).mp4"
	if err := dbConn.QueryRow(`INSERT INTO movies (library_id, title, path, duration, match_status, tmdb_id, poster_path, imdb_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`, libraryID, "Finished Movie", finishedPath, 0, db.MatchStatusIdentified, 999, "/poster.jpg", "tt9999999").Scan(&finishedID); err != nil {
		t.Fatalf("insert finished movie: %v", err)
	}

	var calls int32
	handler := &LibraryHandler{
		DB: dbConn,
		Meta: &identifyStub{
			movie: func(ctx context.Context, info metadata.MediaInfo) *metadata.MatchResult {
				atomic.AddInt32(&calls, 1)
				switch info.Title {
				case "stuck movie":
					<-ctx.Done()
					return nil
				case "quick movie":
					return &metadata.MatchResult{
						Title:      "Quick Movie",
						Provider:   "tmdb",
						ExternalID: "456",
					}
				default:
					t.Fatalf("unexpected title = %q", info.Title)
					return nil
				}
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/libraries/"+strconv.Itoa(libraryID)+"/identify", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.IdentifyLibrary(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Identified int `json:"identified"`
		Failed     int `json:"failed"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Identified != 1 || payload.Failed != 1 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("identifier call count = %d", got)
	}

	var quickTitle, quickStatus string
	var quickTMDBID int
	if err := dbConn.QueryRow(`SELECT title, match_status, COALESCE(tmdb_id, 0) FROM movies WHERE id = ?`, quickID).Scan(&quickTitle, &quickStatus, &quickTMDBID); err != nil {
		t.Fatalf("query quick movie: %v", err)
	}
	if quickTitle != "Quick Movie" || quickStatus != db.MatchStatusIdentified || quickTMDBID != 456 {
		t.Fatalf("unexpected quick movie state: title=%q status=%q tmdb=%d", quickTitle, quickStatus, quickTMDBID)
	}

	var finishedTitle, finishedStatus string
	var finishedTMDBID int
	if err := dbConn.QueryRow(`SELECT title, match_status, COALESCE(tmdb_id, 0) FROM movies WHERE id = ?`, finishedID).Scan(&finishedTitle, &finishedStatus, &finishedTMDBID); err != nil {
		t.Fatalf("query finished movie: %v", err)
	}
	if finishedTitle != "Finished Movie" || finishedStatus != db.MatchStatusIdentified || finishedTMDBID != 999 {
		t.Fatalf("unexpected finished movie state: title=%q status=%q tmdb=%d", finishedTitle, finishedStatus, finishedTMDBID)
	}
}

func TestIdentifyLibrary_DedupesDuplicateMovieLookupsWithinRun(t *testing.T) {
	dbConn, err := db.InitDB(filepath.Join(t.TempDir(), "plum.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	prevWorkers := identifyMovieWorkers
	prevRateInterval := identifyMovieRateLimit
	prevRateBurst := identifyMovieRateBurst
	identifyMovieWorkers = 4
	identifyMovieRateLimit = time.Millisecond
	identifyMovieRateBurst = 4
	t.Cleanup(func() {
		identifyMovieWorkers = prevWorkers
		identifyMovieRateLimit = prevRateInterval
		identifyMovieRateBurst = prevRateBurst
	})

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "dedupe@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "Movies", db.LibraryTypeMovie, "/movies", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	for _, row := range []struct {
		title string
		path  string
	}{
		{title: "Shared Movie", path: "/movies/Shared Movie (2024)/Shared Movie (2024) [1080p].mp4"},
		{title: "Shared Movie", path: "/movies/Shared Movie (2024)/Shared Movie (2024) [4k].mkv"},
		{title: "Unique Movie", path: "/movies/Unique Movie (2024)/Unique Movie (2024).mp4"},
	} {
		if _, err := dbConn.Exec(`INSERT INTO movies (library_id, title, path, duration, match_status) VALUES (?, ?, ?, ?, ?)`, libraryID, row.title, row.path, 0, db.MatchStatusLocal); err != nil {
			t.Fatalf("insert movie %q: %v", row.path, err)
		}
	}

	var (
		mu    sync.Mutex
		calls = map[string]int{}
	)
	handler := &LibraryHandler{
		DB: dbConn,
		Meta: &identifyStub{
			movie: func(ctx context.Context, info metadata.MediaInfo) *metadata.MatchResult {
				key := metadata.NormalizeTitle(info.Title)
				mu.Lock()
				calls[key]++
				mu.Unlock()
				return &metadata.MatchResult{
					Title:      info.Title,
					Provider:   "tmdb",
					ExternalID: "101",
				}
			},
		},
	}

	result, err := handler.identifyLibrary(context.Background(), libraryID)
	if err != nil {
		t.Fatalf("identify library: %v", err)
	}
	if result.Identified != 3 || result.Failed != 0 {
		rows, queryErr := dbConn.Query(`SELECT title, path, match_status, COALESCE(tmdb_id, 0) FROM movies WHERE library_id = ? ORDER BY path`, libraryID)
		if queryErr != nil {
			t.Fatalf("unexpected result: %+v (query err: %v)", result, queryErr)
		}
		defer rows.Close()
		var states []string
		for rows.Next() {
			var title, path, status string
			var tmdbID int
			if err := rows.Scan(&title, &path, &status, &tmdbID); err != nil {
				t.Fatalf("scan state: %v", err)
			}
			states = append(states, fmt.Sprintf("%s|%s|%s|%d", title, path, status, tmdbID))
		}
		t.Fatalf("unexpected result: %+v states=%v calls=%v", result, states, calls)
	}

	mu.Lock()
	defer mu.Unlock()
	if calls["shared movie"] != 1 {
		t.Fatalf("shared movie calls = %d, want 1", calls["shared movie"])
	}
	if calls["unique movie"] != 1 {
		t.Fatalf("unique movie calls = %d, want 1", calls["unique movie"])
	}
}

func TestIdentifyLibrary_RetriesRetryableMovieProviderFailures(t *testing.T) {
	dbConn, err := db.InitDB(filepath.Join(t.TempDir(), "plum.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "retry@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "Movies", db.LibraryTypeMovie, "/movies", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO movies (library_id, title, path, duration, match_status) VALUES (?, ?, ?, ?, ?)`,
		libraryID,
		"Blade",
		"/movies/Blade (1998)/Blade.1998.2160p.mkv",
		0,
		db.MatchStatusLocal,
	); err != nil {
		t.Fatalf("insert movie: %v", err)
	}

	var calls int32
	handler := &LibraryHandler{
		DB: dbConn,
		Meta: &identifyStub{
			movieResult: func(_ context.Context, info metadata.MediaInfo) (*metadata.MatchResult, error) {
				if info.Title != "blade" || info.Year != 1998 {
					t.Fatalf("unexpected info: %+v", info)
				}
				if atomic.AddInt32(&calls, 1) == 1 {
					return nil, &metadata.ProviderError{
						Provider:   "tmdb",
						StatusCode: http.StatusTooManyRequests,
						Retryable:  true,
					}
				}
				return &metadata.MatchResult{
					Title:      "Blade",
					Provider:   "tmdb",
					ExternalID: "36647",
				}, nil
			},
		},
	}

	result, err := handler.identifyLibrary(context.Background(), libraryID)
	if err != nil {
		t.Fatalf("identify library: %v", err)
	}
	if result.Identified != 1 || result.Failed != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("identify calls = %d, want 2", got)
	}
}

func TestIdentifyLibrary_RetriesRetryableMovieTransportFailures(t *testing.T) {
	dbConn, err := db.InitDB(filepath.Join(t.TempDir(), "plum.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "transport@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "Movies", db.LibraryTypeMovie, "/movies", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO movies (library_id, title, path, duration, match_status) VALUES (?, ?, ?, ?, ?)`,
		libraryID,
		"Blade",
		"/movies/Blade (1998)/Blade.1998.2160p.mkv",
		0,
		db.MatchStatusLocal,
	); err != nil {
		t.Fatalf("insert movie: %v", err)
	}

	var calls int32
	handler := &LibraryHandler{
		DB: dbConn,
		Meta: &identifyStub{
			movieResult: func(_ context.Context, info metadata.MediaInfo) (*metadata.MatchResult, error) {
				if info.Title != "blade" || info.Year != 1998 {
					t.Fatalf("unexpected info: %+v", info)
				}
				if atomic.AddInt32(&calls, 1) == 1 {
					return nil, &metadata.ProviderError{
						Provider: "tmdb",
						Cause: &os.SyscallError{
							Syscall: "read",
							Err:     syscall.ECONNRESET,
						},
						Retryable: true,
					}
				}
				return &metadata.MatchResult{
					Title:      "Blade",
					Provider:   "tmdb",
					ExternalID: "36647",
				}, nil
			},
		},
	}

	result, err := handler.identifyLibrary(context.Background(), libraryID)
	if err != nil {
		t.Fatalf("identify library: %v", err)
	}
	if result.Identified != 1 || result.Failed != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("identify calls = %d, want 2", got)
	}
}

func TestIdentifyLibrary_DoesNotRetryNonRetryableMovieTransportFailures(t *testing.T) {
	dbConn, err := db.InitDB(filepath.Join(t.TempDir(), "plum.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "nonretry@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "Movies", db.LibraryTypeMovie, "/movies", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO movies (library_id, title, path, duration, match_status) VALUES (?, ?, ?, ?, ?)`,
		libraryID,
		"Blade",
		"/movies/Blade (1998)/Blade.1998.2160p.mkv",
		0,
		db.MatchStatusLocal,
	); err != nil {
		t.Fatalf("insert movie: %v", err)
	}

	var calls int32
	handler := &LibraryHandler{
		DB: dbConn,
		Meta: &identifyStub{
			movieResult: func(_ context.Context, info metadata.MediaInfo) (*metadata.MatchResult, error) {
				if info.Title != "blade" || info.Year != 1998 {
					t.Fatalf("unexpected info: %+v", info)
				}
				atomic.AddInt32(&calls, 1)
				return nil, &metadata.ProviderError{
					Provider: "tmdb",
					Cause: &url.Error{
						Op:  "Get",
						URL: "https://example.com/test",
						Err: nonRetryableNetError{msg: "temporary provider hiccup"},
					},
				}
			},
		},
	}

	result, err := handler.identifyLibrary(context.Background(), libraryID)
	if err != nil {
		t.Fatalf("identify library: %v", err)
	}
	if result.Identified != 0 || result.Failed != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("identify calls = %d, want 1", got)
	}
}

func TestLibraryScanManager_FailedMovieNoMatchReindexesAfterTerminalState(t *testing.T) {
	dbConn, err := db.InitDB(filepath.Join(t.TempDir(), "plum.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "nomatch@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "Movies", db.LibraryTypeMovie, "/movies", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO movies (library_id, title, path, duration, match_status) VALUES (?, ?, ?, ?, ?)`,
		libraryID,
		"Unknown Title",
		"/movies/Unknown Title (2026)/Unknown.Title.2026.mkv",
		0,
		db.MatchStatusLocal,
	); err != nil {
		t.Fatalf("insert movie: %v", err)
	}

	var calls int32
	searchIndex := NewSearchIndexManager(context.Background(), dbConn, nil, nil)
	var queueCalls int32
	searchIndex.onQueue = func(gotLibraryID int, full bool) {
		if gotLibraryID != libraryID {
			t.Fatalf("queued library id = %d", gotLibraryID)
		}
		if full {
			t.Fatal("expected incremental queue")
		}
		atomic.AddInt32(&queueCalls, 1)
	}
	searchIndex.refresh = func(gotLibraryID int, full bool) error {
		if gotLibraryID != libraryID {
			t.Fatalf("refresh library id = %d", gotLibraryID)
		}
		if full {
			t.Fatal("expected incremental refresh")
		}
		return nil
	}

	scanJobs := NewLibraryScanManager(context.Background(), dbConn, &identifyStub{
		movieResult: func(_ context.Context, _ metadata.MediaInfo) (*metadata.MatchResult, error) {
			atomic.AddInt32(&calls, 1)
			return nil, nil
		},
	}, nil, "")
	scanJobs.handler = &LibraryHandler{
		DB:          dbConn,
		Meta:        scanJobs.meta,
		ScanJobs:    scanJobs,
		SearchIndex: searchIndex,
	}
	scanJobs.mu.Lock()
	scanJobs.jobs[libraryID] = libraryScanStatus{
		LibraryID:         libraryID,
		Phase:             libraryScanPhaseCompleted,
		IdentifyPhase:     libraryIdentifyPhaseIdle,
		IdentifyRequested: true,
		MaxRetries:        3,
	}
	scanJobs.types[libraryID] = db.LibraryTypeMovie
	scanJobs.paths[libraryID] = "/movies"
	scanJobs.mu.Unlock()

	scanJobs.startIdentify(libraryID)

	deadline := time.Now().Add(2 * time.Second)
	for {
		status := scanJobs.status(libraryID)
		if status.IdentifyPhase == libraryIdentifyPhaseFailed {
			if status.IdentifyFailed != 1 {
				t.Fatalf("identify failed count = %d", status.IdentifyFailed)
			}
			if status.LastError != "1 item(s) could not be identified automatically" {
				t.Fatalf("last error = %q", status.LastError)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("identify did not fail: %+v", status)
		}
		time.Sleep(10 * time.Millisecond)
	}

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("identify calls = %d, want 1", got)
	}
	deadline = time.Now().Add(time.Second)
	for {
		if atomic.LoadInt32(&queueCalls) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("search index queue calls = %d, want 1", atomic.LoadInt32(&queueCalls))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestIdentifyLibrary_GroupsTVEpisodesByShow(t *testing.T) {
	dbConn, err := db.InitDB(filepath.Join(t.TempDir(), "plum.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	prevWorkers := identifyEpisodeWorkers
	prevRateInterval := identifyEpisodeRateLimit
	prevRateBurst := identifyEpisodeRateBurst
	identifyEpisodeWorkers = 4
	identifyEpisodeRateLimit = time.Millisecond
	identifyEpisodeRateBurst = 4
	t.Cleanup(func() {
		identifyEpisodeWorkers = prevWorkers
		identifyEpisodeRateLimit = prevRateInterval
		identifyEpisodeRateBurst = prevRateBurst
	})

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "tv-group@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "TV", db.LibraryTypeTV, "/tv", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	for episode := 1; episode <= 2; episode++ {
		path := fmt.Sprintf("/tv/Slow Horses/Season 1/Slow Horses - S01E%02d.mkv", episode)
		title := fmt.Sprintf("Slow Horses - S01E%02d", episode)
		if _, err := dbConn.Exec(`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			libraryID,
			title,
			path,
			0,
			db.MatchStatusUnmatched,
			1,
			episode,
		); err != nil {
			t.Fatalf("insert episode %d: %v", episode, err)
		}
	}

	var (
		searchCalls  int32
		detailsCalls int32
		episodeCalls int32
		queueCalls   int32
	)
	searchIndex := NewSearchIndexManager(context.Background(), dbConn, nil, nil)
	searchIndex.onQueue = func(gotLibraryID int, full bool) {
		if gotLibraryID != libraryID {
			t.Fatalf("queued library id = %d", gotLibraryID)
		}
		if full {
			t.Fatal("expected incremental queue")
		}
		atomic.AddInt32(&queueCalls, 1)
	}
	searchIndex.refresh = func(gotLibraryID int, full bool) error {
		if gotLibraryID != libraryID {
			t.Fatalf("refresh library id = %d", gotLibraryID)
		}
		if full {
			t.Fatal("expected incremental refresh")
		}
		return nil
	}

	handler := &LibraryHandler{
		DB: dbConn,
		Meta: &identifyStub{
			tv: func(_ context.Context, info metadata.MediaInfo) *metadata.MatchResult {
				t.Fatalf("unexpected per-row TV identify for %+v", info)
				return nil
			},
		},
		SeriesQuery: &seriesQueryStub{
			searchTV: func(_ context.Context, query string) ([]metadata.MatchResult, error) {
				atomic.AddInt32(&searchCalls, 1)
				if !strings.EqualFold(query, "Slow Horses") {
					t.Fatalf("query = %q", query)
				}
				return []metadata.MatchResult{{
					Title:      "Slow Horses",
					Provider:   "tmdb",
					ExternalID: "321",
				}}, nil
			},
			getEpisode: func(_ context.Context, provider, seriesID string, season, episode int) (*metadata.MatchResult, error) {
				atomic.AddInt32(&episodeCalls, 1)
				if provider != "tmdb" || seriesID != "321" {
					t.Fatalf("unexpected provider/series = %q/%q", provider, seriesID)
				}
				return &metadata.MatchResult{
					Title:      fmt.Sprintf("Slow Horses - S01E%02d - Episode %d", episode, episode),
					Provider:   "tmdb",
					ExternalID: "321",
				}, nil
			},
		},
		Series: &seriesDetailsStub{
			getSeriesDetails: func(_ context.Context, tmdbID int) (*metadata.SeriesDetails, error) {
				atomic.AddInt32(&detailsCalls, 1)
				if tmdbID != 321 {
					t.Fatalf("tmdb id = %d", tmdbID)
				}
				return &metadata.SeriesDetails{Name: "Slow Horses"}, nil
			},
		},
		SearchIndex: searchIndex,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/libraries/"+strconv.Itoa(libraryID)+"/identify", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.IdentifyLibrary(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Identified int `json:"identified"`
		Failed     int `json:"failed"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Identified != 2 || payload.Failed != 0 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if got := atomic.LoadInt32(&searchCalls); got != 1 {
		t.Fatalf("search calls = %d", got)
	}
	if got := atomic.LoadInt32(&detailsCalls); got != 1 {
		t.Fatalf("series detail calls = %d", got)
	}
	if got := atomic.LoadInt32(&episodeCalls); got != 2 {
		t.Fatalf("episode calls = %d", got)
	}
	if got := atomic.LoadInt32(&queueCalls); got != 1 {
		t.Fatalf("queue calls = %d", got)
	}
	var reviewNeededCount int
	if err := dbConn.QueryRow(`SELECT COUNT(*) FROM tv_episodes WHERE library_id = ? AND COALESCE(metadata_review_needed, 0) = 1`, libraryID).Scan(&reviewNeededCount); err != nil {
		t.Fatalf("count review-needed episodes: %v", err)
	}
	if reviewNeededCount != 0 {
		t.Fatalf("review-needed episodes = %d", reviewNeededCount)
	}
}

func TestIdentifyLibrary_GroupsTVDBEpisodesAndFallsBackToTitleSearch(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	prevWorkers := identifyEpisodeWorkers
	prevRateInterval := identifyEpisodeRateLimit
	prevRateBurst := identifyEpisodeRateBurst
	identifyEpisodeWorkers = 1
	identifyEpisodeRateLimit = time.Millisecond
	identifyEpisodeRateBurst = 1
	t.Cleanup(func() {
		identifyEpisodeWorkers = prevWorkers
		identifyEpisodeRateLimit = prevRateInterval
		identifyEpisodeRateBurst = prevRateBurst
	})

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "tvdb-group@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "TV", db.LibraryTypeTV, "/tv", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, tvdb_id, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		libraryID,
		"Slow Horses - S01E01",
		"/tv/Slow Horses/Season 1/Slow Horses - S01E01.mkv",
		0,
		db.MatchStatusIdentified,
		"tvdb-321",
		1,
		1,
	); err != nil {
		t.Fatalf("insert episode: %v", err)
	}

	var (
		searchCalls  int32
		episodeCalls int32
	)
	handler := &LibraryHandler{
		DB: dbConn,
		Meta: &identifyStub{
			tv: func(_ context.Context, info metadata.MediaInfo) *metadata.MatchResult {
				t.Fatalf("unexpected per-row TV identify for %+v", info)
				return nil
			},
		},
		SeriesQuery: &seriesQueryStub{
			searchTV: func(_ context.Context, query string) ([]metadata.MatchResult, error) {
				atomic.AddInt32(&searchCalls, 1)
				if !strings.EqualFold(query, "Slow Horses") {
					t.Fatalf("query = %q", query)
				}
				return []metadata.MatchResult{{
					Title:      "Slow Horses",
					Provider:   "tmdb",
					ExternalID: "321",
				}}, nil
			},
			getEpisode: func(_ context.Context, provider, seriesID string, season, episode int) (*metadata.MatchResult, error) {
				atomic.AddInt32(&episodeCalls, 1)
				if provider != "tmdb" || seriesID != "321" {
					t.Fatalf("unexpected provider/series = %q/%q", provider, seriesID)
				}
				return &metadata.MatchResult{
					Title:      "Slow Horses - S01E01 - Episode 1",
					Provider:   "tmdb",
					ExternalID: "321",
				}, nil
			},
		},
		Series: &seriesDetailsStub{
			getSeriesDetails: func(_ context.Context, tmdbID int) (*metadata.SeriesDetails, error) {
				return &metadata.SeriesDetails{Name: "Slow Horses"}, nil
			},
		},
	}

	result, err := handler.identifyLibrary(context.Background(), libraryID)
	if err != nil {
		t.Fatalf("identify library: %v", err)
	}
	if result.Identified != 1 || result.Failed != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if got := atomic.LoadInt32(&searchCalls); got != 1 {
		t.Fatalf("search calls = %d", got)
	}
	if got := atomic.LoadInt32(&episodeCalls); got != 1 {
		t.Fatalf("episode calls = %d", got)
	}
}

func TestIdentifyLibrary_GroupedEpisodeRetryUsesLongerTimeoutAndFreshLookup(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	prevInitialTimeout := identifyInitialTimeout
	prevRetryTimeout := identifyRetryTimeout
	prevWorkers := identifyEpisodeWorkers
	prevRateInterval := identifyEpisodeRateLimit
	prevRateBurst := identifyEpisodeRateBurst
	identifyInitialTimeout = 5 * time.Millisecond
	identifyRetryTimeout = 50 * time.Millisecond
	identifyEpisodeWorkers = 1
	identifyEpisodeRateLimit = time.Millisecond
	identifyEpisodeRateBurst = 1
	t.Cleanup(func() {
		identifyInitialTimeout = prevInitialTimeout
		identifyRetryTimeout = prevRetryTimeout
		identifyEpisodeWorkers = prevWorkers
		identifyEpisodeRateLimit = prevRateInterval
		identifyEpisodeRateBurst = prevRateBurst
	})

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "retry-group@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "TV", db.LibraryTypeTV, "/tv", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, tmdb_id, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		libraryID,
		"Slow Horses - S01E01",
		"/tv/Slow Horses/Season 1/Slow Horses - S01E01.mkv",
		0,
		db.MatchStatusIdentified,
		321,
		1,
		1,
	); err != nil {
		t.Fatalf("insert episode: %v", err)
	}

	var episodeCalls int32
	handler := &LibraryHandler{
		DB: dbConn,
		Meta: &identifyStub{
			tv: func(_ context.Context, info metadata.MediaInfo) *metadata.MatchResult {
				t.Fatalf("unexpected per-row TV identify for %+v", info)
				return nil
			},
		},
		SeriesQuery: &seriesQueryStub{
			searchTV: func(_ context.Context, query string) ([]metadata.MatchResult, error) {
				t.Fatalf("unexpected fallback search for %q", query)
				return nil, nil
			},
			getEpisode: func(ctx context.Context, provider, seriesID string, season, episode int) (*metadata.MatchResult, error) {
				atomic.AddInt32(&episodeCalls, 1)
				deadline, ok := ctx.Deadline()
				if !ok {
					t.Fatal("expected identify timeout on grouped lookup")
				}
				if time.Until(deadline) < 20*time.Millisecond {
					return nil, nil
				}
				return &metadata.MatchResult{
					Title:      "Slow Horses - S01E01 - Episode 1",
					Provider:   "tmdb",
					ExternalID: "321",
				}, nil
			},
		},
		Series: &seriesDetailsStub{
			getSeriesDetails: func(_ context.Context, tmdbID int) (*metadata.SeriesDetails, error) {
				return &metadata.SeriesDetails{Name: "Slow Horses"}, nil
			},
		},
	}

	result, err := handler.identifyLibrary(context.Background(), libraryID)
	if err != nil {
		t.Fatalf("identify library: %v", err)
	}
	if result.Identified != 1 || result.Failed != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if got := atomic.LoadInt32(&episodeCalls); got != 2 {
		t.Fatalf("episode calls = %d", got)
	}
}

func TestIdentifyLibrary_GroupedEpisodesFallbackOnlyForUnresolvedRows(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	prevWorkers := identifyEpisodeWorkers
	prevRateInterval := identifyEpisodeRateLimit
	prevRateBurst := identifyEpisodeRateBurst
	identifyEpisodeWorkers = 4
	identifyEpisodeRateLimit = time.Millisecond
	identifyEpisodeRateBurst = 4
	t.Cleanup(func() {
		identifyEpisodeWorkers = prevWorkers
		identifyEpisodeRateLimit = prevRateInterval
		identifyEpisodeRateBurst = prevRateBurst
	})

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "tv-partial@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "TV", db.LibraryTypeTV, "/tv", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	for episode := 1; episode <= 2; episode++ {
		path := fmt.Sprintf("/tv/Slow Horses/Season 1/Slow Horses - S01E%02d.mkv", episode)
		title := fmt.Sprintf("Slow Horses - S01E%02d", episode)
		if _, err := dbConn.Exec(`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			libraryID,
			title,
			path,
			0,
			db.MatchStatusUnmatched,
			1,
			episode,
		); err != nil {
			t.Fatalf("insert episode %d: %v", episode, err)
		}
	}

	var (
		searchCalls  int32
		detailsCalls int32
		episodeCalls int32
	)
	handler := &LibraryHandler{
		DB: dbConn,
		Meta: &identifyStub{
			tv: func(_ context.Context, info metadata.MediaInfo) *metadata.MatchResult {
				t.Fatalf("unexpected per-row TV identify for %+v", info)
				return nil
			},
		},
		SeriesQuery: &seriesQueryStub{
			searchTV: func(_ context.Context, query string) ([]metadata.MatchResult, error) {
				atomic.AddInt32(&searchCalls, 1)
				return []metadata.MatchResult{{Title: "Slow Horses", Provider: "tmdb", ExternalID: "321"}}, nil
			},
			getEpisode: func(_ context.Context, provider, seriesID string, season, episode int) (*metadata.MatchResult, error) {
				atomic.AddInt32(&episodeCalls, 1)
				if episode == 2 {
					return nil, nil
				}
				return &metadata.MatchResult{
					Title:      "Slow Horses - S01E01 - Pilot",
					Provider:   "tmdb",
					ExternalID: "321",
				}, nil
			},
		},
		Series: &seriesDetailsStub{
			getSeriesDetails: func(_ context.Context, tmdbID int) (*metadata.SeriesDetails, error) {
				atomic.AddInt32(&detailsCalls, 1)
				return &metadata.SeriesDetails{Name: "Slow Horses"}, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/libraries/"+strconv.Itoa(libraryID)+"/identify", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.IdentifyLibrary(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Identified int `json:"identified"`
		Failed     int `json:"failed"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Identified != 1 || payload.Failed != 1 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if got := atomic.LoadInt32(&searchCalls); got != 1 {
		t.Fatalf("search calls = %d", got)
	}
	if got := atomic.LoadInt32(&detailsCalls); got != 1 {
		t.Fatalf("series detail calls = %d", got)
	}
	if got := atomic.LoadInt32(&episodeCalls); got != 3 {
		t.Fatalf("episode calls = %d", got)
	}
}

func TestIdentifyLibrary_GroupsSafeAnimeAndLeavesAbsoluteEpisodesOnResidualPath(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	prevWorkers := identifyEpisodeWorkers
	prevRateInterval := identifyEpisodeRateLimit
	prevRateBurst := identifyEpisodeRateBurst
	identifyEpisodeWorkers = 4
	identifyEpisodeRateLimit = time.Millisecond
	identifyEpisodeRateBurst = 4
	t.Cleanup(func() {
		identifyEpisodeWorkers = prevWorkers
		identifyEpisodeRateLimit = prevRateInterval
		identifyEpisodeRateBurst = prevRateBurst
	})

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "anime-group@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "Anime", db.LibraryTypeAnime, "/anime", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	for episode := 1; episode <= 2; episode++ {
		path := fmt.Sprintf("/anime/Frieren/Season 1/Frieren - S01E%02d.mkv", episode)
		title := fmt.Sprintf("Frieren - S01E%02d", episode)
		if _, err := dbConn.Exec(`INSERT INTO anime_episodes (library_id, title, path, duration, match_status, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			libraryID,
			title,
			path,
			0,
			db.MatchStatusUnmatched,
			1,
			episode,
		); err != nil {
			t.Fatalf("insert safe anime episode %d: %v", episode, err)
		}
	}
	var absoluteID int
	if err := dbConn.QueryRow(`INSERT INTO anime_episodes (library_id, title, path, duration, match_status, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		libraryID,
		"Frieren - 12",
		"/anime/Frieren/Frieren - 12.mkv",
		0,
		db.MatchStatusUnmatched,
		0,
		0,
	).Scan(&absoluteID); err != nil {
		t.Fatalf("insert absolute anime episode: %v", err)
	}

	var (
		searchCalls   int32
		episodeCalls  int32
		identifyCalls int32
	)
	handler := &LibraryHandler{
		DB: dbConn,
		Meta: &identifyStub{
			anime: func(_ context.Context, info metadata.MediaInfo) *metadata.MatchResult {
				atomic.AddInt32(&identifyCalls, 1)
				if info.AbsoluteEpisode != 12 {
					t.Fatalf("unexpected residual anime info: %+v", info)
				}
				return &metadata.MatchResult{
					Title:      "Frieren - Episode 12",
					Provider:   "tmdb",
					ExternalID: "123",
				}
			},
		},
		SeriesQuery: &seriesQueryStub{
			searchTV: func(_ context.Context, query string) ([]metadata.MatchResult, error) {
				atomic.AddInt32(&searchCalls, 1)
				if !strings.EqualFold(query, "Frieren") {
					t.Fatalf("query = %q", query)
				}
				return []metadata.MatchResult{{Title: "Frieren", Provider: "tmdb", ExternalID: "123"}}, nil
			},
			getEpisode: func(_ context.Context, provider, seriesID string, season, episode int) (*metadata.MatchResult, error) {
				atomic.AddInt32(&episodeCalls, 1)
				return &metadata.MatchResult{
					Title:      fmt.Sprintf("Frieren - S01E%02d - Episode %d", episode, episode),
					Provider:   "tmdb",
					ExternalID: "123",
				}, nil
			},
		},
		Series: &seriesDetailsStub{
			getSeriesDetails: func(_ context.Context, tmdbID int) (*metadata.SeriesDetails, error) {
				return &metadata.SeriesDetails{Name: "Frieren"}, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/libraries/"+strconv.Itoa(libraryID)+"/identify", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.IdentifyLibrary(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Identified int `json:"identified"`
		Failed     int `json:"failed"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Identified != 3 || payload.Failed != 0 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if got := atomic.LoadInt32(&searchCalls); got != 1 {
		t.Fatalf("search calls = %d", got)
	}
	if got := atomic.LoadInt32(&episodeCalls); got != 2 {
		t.Fatalf("episode calls = %d", got)
	}
	if got := atomic.LoadInt32(&identifyCalls); got != 1 {
		t.Fatalf("anime identify calls = %d", got)
	}

	var absoluteTitle string
	if err := dbConn.QueryRow(`SELECT title FROM anime_episodes WHERE id = ?`, absoluteID).Scan(&absoluteTitle); err != nil {
		t.Fatalf("query absolute anime episode: %v", err)
	}
	if absoluteTitle != "Frieren - Episode 12" {
		t.Fatalf("absolute episode title = %q", absoluteTitle)
	}
}

func TestIdentifyLibrary_DoesNotCountMatchedTVMetadataRefreshAsFailed(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "tv-refresh@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "TV", db.LibraryTypeTV, "/tv", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	if _, err := dbConn.Exec(
		`INSERT INTO tv_episodes (
			library_id, title, path, duration, match_status, season, episode, tmdb_id, poster_path, imdb_id, last_metadata_refresh_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		libraryID,
		"Slow Horses - S01E01 - Pilot",
		"/tv/Slow Horses/Season 1/Slow Horses - S01E01.mkv",
		0,
		db.MatchStatusIdentified,
		1,
		1,
		321,
		"",
		"",
		"",
	); err != nil {
		t.Fatalf("insert episode: %v", err)
	}

	var getEpisodeCalls int32
	handler := &LibraryHandler{
		DB: dbConn,
		Meta: &identifyStub{
			tv: func(_ context.Context, info metadata.MediaInfo) *metadata.MatchResult {
				t.Fatalf("unexpected per-row TV identify for %+v", info)
				return nil
			},
		},
		SeriesQuery: &seriesQueryStub{
			searchTV: func(_ context.Context, query string) ([]metadata.MatchResult, error) {
				t.Fatalf("unexpected search fallback query=%q", query)
				return nil, nil
			},
			getEpisode: func(_ context.Context, provider, seriesID string, season, episode int) (*metadata.MatchResult, error) {
				atomic.AddInt32(&getEpisodeCalls, 1)
				if provider != "tmdb" || seriesID != "321" || season != 1 || episode != 1 {
					t.Fatalf("unexpected episode lookup provider=%s series=%s season=%d episode=%d", provider, seriesID, season, episode)
				}
				return &metadata.MatchResult{
					Title:      "Slow Horses - S01E01 - Pilot",
					Provider:   "tmdb",
					ExternalID: "321",
				}, nil
			},
		},
		Series: &seriesDetailsStub{
			getSeriesDetails: func(_ context.Context, tmdbID int) (*metadata.SeriesDetails, error) {
				if tmdbID != 321 {
					t.Fatalf("unexpected series details lookup tmdbID=%d", tmdbID)
				}
				return &metadata.SeriesDetails{Name: "Slow Horses"}, nil
			},
		},
	}

	result, err := handler.identifyLibrary(context.Background(), libraryID)
	if err != nil {
		t.Fatalf("identify library: %v", err)
	}
	if result.Failed != 0 {
		t.Fatalf("failed = %d", result.Failed)
	}
	if result.Identified != 1 {
		t.Fatalf("identified = %d", result.Identified)
	}
	if got := atomic.LoadInt32(&getEpisodeCalls); got != 1 {
		t.Fatalf("episode calls = %d", got)
	}
	if states := handler.identifyRun.stateForLibrary(libraryID); len(states) != 0 {
		t.Fatalf("unexpected identify states: %+v", states)
	}
}

func TestIdentifyLibrary_DoesNotCountMatchedMovieMetadataRefreshAsFailed(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "movie-refresh@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "Movies", db.LibraryTypeMovie, "/movies", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	if _, err := dbConn.Exec(
		`INSERT INTO movies (
			library_id, title, path, duration, match_status, tmdb_id, poster_path, imdb_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		libraryID,
		"Die My Love",
		"/movies/Die My Love (2025)/Die My Love.mp4",
		0,
		db.MatchStatusIdentified,
		444,
		"",
		"",
	); err != nil {
		t.Fatalf("insert movie: %v", err)
	}

	var identifyCalls int32
	handler := &LibraryHandler{
		DB: dbConn,
		Meta: &identifyStub{
			movie: func(_ context.Context, info metadata.MediaInfo) *metadata.MatchResult {
				atomic.AddInt32(&identifyCalls, 1)
				if info.TMDBID != 444 {
					t.Fatalf("unexpected movie info: %+v", info)
				}
				return &metadata.MatchResult{
					Title:      "Die My Love",
					Provider:   "tmdb",
					ExternalID: "444",
				}
			},
		},
	}

	result, err := handler.identifyLibrary(context.Background(), libraryID)
	if err != nil {
		t.Fatalf("identify library: %v", err)
	}
	if result.Failed != 0 {
		t.Fatalf("failed = %d", result.Failed)
	}
	if result.Identified != 1 {
		t.Fatalf("identified = %d", result.Identified)
	}
	if got := atomic.LoadInt32(&identifyCalls); got != 1 {
		t.Fatalf("identify calls = %d", got)
	}
	if states := handler.identifyRun.stateForLibrary(libraryID); len(states) != 0 {
		t.Fatalf("unexpected identify states: %+v", states)
	}
}

func TestIdentifyConfigForKind_UsesIndependentEpisodeTuning(t *testing.T) {
	prevMovieWorkers := identifyMovieWorkers
	prevMovieRateLimit := identifyMovieRateLimit
	prevMovieRateBurst := identifyMovieRateBurst
	prevEpisodeWorkers := identifyEpisodeWorkers
	prevEpisodeRateLimit := identifyEpisodeRateLimit
	prevEpisodeRateBurst := identifyEpisodeRateBurst

	identifyMovieWorkers = 6
	identifyMovieRateLimit = 100 * time.Millisecond
	identifyMovieRateBurst = 6
	identifyEpisodeWorkers = 4
	identifyEpisodeRateLimit = 150 * time.Millisecond
	identifyEpisodeRateBurst = 4
	t.Cleanup(func() {
		identifyMovieWorkers = prevMovieWorkers
		identifyMovieRateLimit = prevMovieRateLimit
		identifyMovieRateBurst = prevMovieRateBurst
		identifyEpisodeWorkers = prevEpisodeWorkers
		identifyEpisodeRateLimit = prevEpisodeRateLimit
		identifyEpisodeRateBurst = prevEpisodeRateBurst
	})

	movieConfig := identifyConfigForKind(db.LibraryTypeMovie)
	episodeConfig := identifyConfigForKind(db.LibraryTypeTV)

	if movieConfig.workers != 6 || movieConfig.rateInterval != 100*time.Millisecond || movieConfig.rateBurst != 6 {
		t.Fatalf("unexpected movie config: %+v", movieConfig)
	}
	if episodeConfig.workers != 4 || episodeConfig.rateInterval != 150*time.Millisecond || episodeConfig.rateBurst != 4 {
		t.Fatalf("unexpected episodic config: %+v", episodeConfig)
	}
}

func TestSearchIndexManager_QueueWhileRunningSchedulesRerun(t *testing.T) {
	manager := NewSearchIndexManager(context.Background(), nil, nil, nil)
	firstRunStarted := make(chan struct{}, 1)
	releaseFirstRun := make(chan struct{})

	var (
		mu    sync.Mutex
		calls []bool
	)
	manager.refresh = func(libraryID int, full bool) error {
		mu.Lock()
		calls = append(calls, full)
		callCount := len(calls)
		mu.Unlock()

		if libraryID != 7 {
			t.Fatalf("library id = %d", libraryID)
		}
		if callCount == 1 {
			firstRunStarted <- struct{}{}
			<-releaseFirstRun
		}
		return nil
	}

	manager.Queue(7, false)
	select {
	case <-firstRunStarted:
	case <-time.After(time.Second):
		t.Fatal("expected first refresh to start")
	}

	manager.Queue(7, false)
	close(releaseFirstRun)

	deadline := time.Now().Add(time.Second)
	for {
		mu.Lock()
		callCount := len(calls)
		mu.Unlock()
		if callCount == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected rerun after in-flight queue, got %d refreshes", callCount)
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 2 {
		t.Fatalf("refresh calls = %d", len(calls))
	}
	if calls[0] || calls[1] {
		t.Fatalf("unexpected full refresh flags: %+v", calls)
	}
}

func TestLibraryScanManager_FinishSkipsSearchIndexQueueWhenIdentifyRequested(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	manager := NewLibraryScanManager(context.Background(), dbConn, nil, nil, "")
	searchIndex := NewSearchIndexManager(context.Background(), dbConn, nil, nil)
	var queueCalls int32
	searchIndex.onQueue = func(gotLibraryID int, full bool) {
		if gotLibraryID != 7 {
			t.Fatalf("queued library id = %d", gotLibraryID)
		}
		if full {
			t.Fatal("expected incremental queue")
		}
		atomic.AddInt32(&queueCalls, 1)
	}
	manager.handler = &LibraryHandler{DB: dbConn, SearchIndex: searchIndex}

	manager.mu.Lock()
	manager.jobs[7] = libraryScanStatus{
		LibraryID:         7,
		Phase:             libraryScanPhaseScanning,
		IdentifyRequested: true,
		MaxRetries:        3,
	}
	manager.paths[7] = "/library"
	manager.types[7] = db.LibraryTypeMovie
	manager.mu.Unlock()

	manager.finish(7, libraryScanPhaseCompleted, db.ScanResult{Added: 3}, "")

	if got := atomic.LoadInt32(&queueCalls); got != 0 {
		t.Fatalf("search index queue calls = %d, want 0", got)
	}
}

func TestLibraryScanManager_StartIdentifyQueuesSearchIndexOnceAtTerminalState(t *testing.T) {
	dbConn, err := db.InitDB(filepath.Join(t.TempDir(), "plum.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "queue@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "Movies", db.LibraryTypeMovie, "/movies", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO movies (library_id, title, path, duration, match_status) VALUES (?, ?, ?, ?, ?)`,
		libraryID,
		"Contact",
		"/movies/Contact (1997)/Contact.1997.mkv",
		0,
		db.MatchStatusLocal,
	); err != nil {
		t.Fatalf("insert movie: %v", err)
	}

	searchIndex := NewSearchIndexManager(context.Background(), dbConn, nil, nil)
	var queueCalls int32
	searchIndex.onQueue = func(gotLibraryID int, full bool) {
		if gotLibraryID != libraryID {
			t.Fatalf("queued library id = %d", gotLibraryID)
		}
		if full {
			t.Fatal("expected incremental queue")
		}
		atomic.AddInt32(&queueCalls, 1)
	}
	searchIndex.refresh = func(gotLibraryID int, full bool) error {
		if gotLibraryID != libraryID {
			t.Fatalf("refresh library id = %d", gotLibraryID)
		}
		if full {
			t.Fatal("expected incremental refresh")
		}
		return nil
	}

	manager := NewLibraryScanManager(context.Background(), dbConn, &identifyStub{
		movieResult: func(_ context.Context, info metadata.MediaInfo) (*metadata.MatchResult, error) {
			if info.Title != "contact" || info.Year != 1997 {
				t.Fatalf("unexpected info: %+v", info)
			}
			return &metadata.MatchResult{
				Title:      "Contact",
				Provider:   "tmdb",
				ExternalID: "686",
			}, nil
		},
	}, nil, "")
	manager.handler = &LibraryHandler{
		DB:          dbConn,
		Meta:        manager.meta,
		ScanJobs:    manager,
		SearchIndex: searchIndex,
	}
	manager.mu.Lock()
	manager.jobs[libraryID] = libraryScanStatus{
		LibraryID:         libraryID,
		Phase:             libraryScanPhaseCompleted,
		IdentifyPhase:     libraryIdentifyPhaseIdle,
		IdentifyRequested: true,
		MaxRetries:        3,
	}
	manager.paths[libraryID] = "/movies"
	manager.types[libraryID] = db.LibraryTypeMovie
	manager.mu.Unlock()

	manager.startIdentify(libraryID)

	deadline := time.Now().Add(2 * time.Second)
	for {
		status := manager.status(libraryID)
		if status.IdentifyPhase == libraryIdentifyPhaseCompleted {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("identify did not complete: %+v", status)
		}
		time.Sleep(10 * time.Millisecond)
	}

	deadline = time.Now().Add(time.Second)
	for {
		if atomic.LoadInt32(&queueCalls) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("search index queue calls = %d, want 1", atomic.LoadInt32(&queueCalls))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestLibraryScanStatus_ReturnsIdleWhenNoScanHasStarted(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "test@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "TV", db.LibraryTypeTV, "/tv", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	handler := &LibraryHandler{
		DB:       dbConn,
		ScanJobs: NewLibraryScanManager(context.Background(), dbConn, nil, nil, ""),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/libraries/"+strconv.Itoa(libraryID)+"/scan", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.GetLibraryScanStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		LibraryID int    `json:"libraryId"`
		Phase     string `json:"phase"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.LibraryID != libraryID || payload.Phase != "idle" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestStartLibraryScan_ImportsMediaInBackground(t *testing.T) {
	dbConn, err := db.InitDB(filepath.Join(t.TempDir(), "plum.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	root := t.TempDir()
	showDir := filepath.Join(root, "Test Show", "Season 1")
	if err := os.MkdirAll(showDir, 0o755); err != nil {
		t.Fatalf("mkdir show dir: %v", err)
	}
	file := filepath.Join(showDir, "Test Show - S01E01.mkv")
	if err := os.WriteFile(file, []byte("not a real video"), 0o644); err != nil {
		t.Fatalf("write media file: %v", err)
	}

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "test@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "TV", db.LibraryTypeTV, root, now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	scanJobs := NewLibraryScanManager(context.Background(), dbConn, nil, nil, "")
	handler := &LibraryHandler{
		DB:       dbConn,
		ScanJobs: scanJobs,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/libraries/"+strconv.Itoa(libraryID)+"/scan/start?identify=false", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.StartLibraryScan(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		status := scanJobs.status(libraryID)
		if status.Phase == libraryScanPhaseCompleted {
			break
		}
		if status.Phase == libraryScanPhaseFailed {
			t.Fatalf("scan failed: %+v", status)
		}
		if time.Now().After(deadline) {
			t.Fatalf("scan did not complete: %+v", status)
		}
		time.Sleep(20 * time.Millisecond)
	}

	var count int
	if err := dbConn.QueryRow(`SELECT COUNT(1) FROM tv_episodes WHERE library_id = ?`, libraryID).Scan(&count); err != nil {
		t.Fatalf("count imported rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 imported row, got %d", count)
	}
}

func TestLibraryScanManager_RecoverResumesQueuedScan(t *testing.T) {
	dbConn, err := db.InitDB(filepath.Join(t.TempDir(), "plum.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	root := t.TempDir()
	showDir := filepath.Join(root, "Recovered Show", "Season 1")
	if err := os.MkdirAll(showDir, 0o755); err != nil {
		t.Fatalf("mkdir show dir: %v", err)
	}
	file := filepath.Join(showDir, "Recovered Show - S01E01.mkv")
	if err := os.WriteFile(file, []byte("not a real video"), 0o644); err != nil {
		t.Fatalf("write media file: %v", err)
	}

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "test@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "TV", db.LibraryTypeTV, root, now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	if err := db.UpsertLibraryJobStatus(dbConn, db.LibraryJobStatus{
		LibraryID:         libraryID,
		Path:              root,
		Type:              db.LibraryTypeTV,
		Phase:             libraryScanPhaseQueued,
		IdentifyPhase:     libraryIdentifyPhaseIdle,
		IdentifyRequested: false,
		StartedAt:         now.Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("seed library job status: %v", err)
	}

	scanJobs := NewLibraryScanManager(context.Background(), dbConn, nil, nil, "")
	if err := scanJobs.Recover(); err != nil {
		t.Fatalf("recover scan jobs: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		status := scanJobs.status(libraryID)
		if status.Phase == libraryScanPhaseCompleted {
			break
		}
		if status.Phase == libraryScanPhaseFailed {
			t.Fatalf("scan failed after recovery: %+v", status)
		}
		if time.Now().After(deadline) {
			t.Fatalf("recovered scan did not complete: %+v", status)
		}
		time.Sleep(20 * time.Millisecond)
	}

	var count int
	if err := dbConn.QueryRow(`SELECT COUNT(1) FROM tv_episodes WHERE library_id = ?`, libraryID).Scan(&count); err != nil {
		t.Fatalf("count imported rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 imported row after recovery, got %d", count)
	}
}

func TestLibraryScanManager_RecoverPreservesFIFOOrder(t *testing.T) {
	dbConn, err := db.InitDB(filepath.Join(t.TempDir(), "plum.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "test@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	bigRoot := t.TempDir()
	bigShowDir := filepath.Join(bigRoot, "Big Show", "Season 1")
	if err := os.MkdirAll(bigShowDir, 0o755); err != nil {
		t.Fatalf("mkdir big show dir: %v", err)
	}
	for i := 1; i <= 3; i++ {
		file := filepath.Join(bigShowDir, "Big Show - S01E0"+strconv.Itoa(i)+".mkv")
		if err := os.WriteFile(file, []byte("not a real video"), 0o644); err != nil {
			t.Fatalf("write big media file: %v", err)
		}
	}

	smallRoot := t.TempDir()
	smallShowDir := filepath.Join(smallRoot, "Small Show", "Season 1")
	if err := os.MkdirAll(smallShowDir, 0o755); err != nil {
		t.Fatalf("mkdir small show dir: %v", err)
	}
	smallFile := filepath.Join(smallShowDir, "Small Show - S01E01.mkv")
	if err := os.WriteFile(smallFile, []byte("not a real video"), 0o644); err != nil {
		t.Fatalf("write small media file: %v", err)
	}

	var bigLibraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "Big TV", db.LibraryTypeTV, bigRoot, now).Scan(&bigLibraryID); err != nil {
		t.Fatalf("insert big library: %v", err)
	}
	var smallLibraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "Small TV", db.LibraryTypeTV, smallRoot, now).Scan(&smallLibraryID); err != nil {
		t.Fatalf("insert small library: %v", err)
	}

	for _, status := range []db.LibraryJobStatus{
		{
			LibraryID:         bigLibraryID,
			Path:              bigRoot,
			Type:              db.LibraryTypeTV,
			Phase:             libraryScanPhaseQueued,
			IdentifyPhase:     libraryIdentifyPhaseIdle,
			IdentifyRequested: false,
			QueuedAt:          now.Add(-1 * time.Minute).Format(time.RFC3339),
			StartedAt:         now.Add(-1 * time.Minute).Format(time.RFC3339),
		},
		{
			LibraryID:         smallLibraryID,
			Path:              smallRoot,
			Type:              db.LibraryTypeTV,
			Phase:             libraryScanPhaseQueued,
			IdentifyPhase:     libraryIdentifyPhaseIdle,
			IdentifyRequested: false,
			QueuedAt:          now.Format(time.RFC3339),
			StartedAt:         now.Format(time.RFC3339),
		},
	} {
		if err := db.UpsertLibraryJobStatus(dbConn, status); err != nil {
			t.Fatalf("seed library job status: %v", err)
		}
	}

	scanJobs := NewLibraryScanManager(context.Background(), dbConn, nil, nil, "")
	if err := scanJobs.Recover(); err != nil {
		t.Fatalf("recover scan jobs: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		bigStatus := scanJobs.status(bigLibraryID)
		smallStatus := scanJobs.status(smallLibraryID)
		// Fast scans can finish both jobs before we observe queuePosition==1; still verify FIFO via timestamps.
		if bigStatus.Phase == libraryScanPhaseCompleted && smallStatus.Phase == libraryScanPhaseCompleted {
			bigFin, e1 := time.Parse(time.RFC3339, bigStatus.FinishedAt)
			smallStart, e2 := time.Parse(time.RFC3339, smallStatus.StartedAt)
			if e1 == nil && e2 == nil && !smallStart.Before(bigFin) {
				return
			}
		}
		if (bigStatus.Phase == libraryScanPhaseScanning || bigStatus.Phase == libraryScanPhaseCompleted) && smallStatus.QueuePosition == 1 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("unexpected statuses big=%+v small=%+v", bigStatus, smallStatus)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestLibraryScanManager_RecoverResumesEnrichmentWithoutRescanning(t *testing.T) {
	dbConn, err := db.InitDB(filepath.Join(t.TempDir(), "plum.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	root := t.TempDir()
	albumDir := filepath.Join(root, "Recovered Artist", "Recovered Album")
	if err := os.MkdirAll(albumDir, 0o755); err != nil {
		t.Fatalf("mkdir album dir: %v", err)
	}
	trackPath := filepath.Join(albumDir, "01 - Recovered Track.flac")
	if err := os.WriteFile(trackPath, []byte("not real audio"), 0o644); err != nil {
		t.Fatalf("write track: %v", err)
	}

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "test@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "Music", db.LibraryTypeMusic, root, now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	if _, err := db.HandleScanLibraryWithOptions(context.Background(), dbConn, root, db.LibraryTypeMusic, libraryID, db.ScanOptions{
		ProbeMedia:             false,
		ProbeEmbeddedSubtitles: false,
		ScanSidecarSubtitles:   false,
		HashMode:               db.ScanHashModeDefer,
	}); err != nil {
		t.Fatalf("seed music library: %v", err)
	}

	if err := db.UpsertLibraryJobStatus(dbConn, db.LibraryJobStatus{
		LibraryID:         libraryID,
		Path:              root,
		Type:              db.LibraryTypeMusic,
		Phase:             libraryScanPhaseCompleted,
		EnrichmentPhase:   libraryEnrichmentPhaseRunning,
		Enriching:         true,
		IdentifyPhase:     libraryIdentifyPhaseIdle,
		IdentifyRequested: true,
		StartedAt:         now.Add(-1 * time.Minute).Format(time.RFC3339),
		FinishedAt:        now.Add(-30 * time.Second).Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("seed library job status: %v", err)
	}

	originalDiscovery := scanLibraryDiscovery
	originalEnrichment := enrichLibraryTasks
	var discoveryCalls atomic.Int32
	enrichmentCalled := make(chan db.ScanOptions, 1)
	scanLibraryDiscovery = func(
		ctx context.Context,
		dbConn *sql.DB,
		root, mediaType string,
		libraryID int,
		options db.ScanOptions,
	) (db.ScanDelta, error) {
		discoveryCalls.Add(1)
		return db.ScanDelta{}, nil
	}
	enrichLibraryTasks = func(
		ctx context.Context,
		dbConn *sql.DB,
		root, mediaType string,
		libraryID int,
		tasks []db.EnrichmentTask,
		options db.ScanOptions,
	) error {
		if len(tasks) != 1 || tasks[0].Path != trackPath {
			t.Fatalf("unexpected recovered enrichment tasks: %+v", tasks)
		}
		select {
		case enrichmentCalled <- options:
		default:
		}
		return nil
	}
	t.Cleanup(func() {
		scanLibraryDiscovery = originalDiscovery
		enrichLibraryTasks = originalEnrichment
	})

	scanJobs := NewLibraryScanManager(context.Background(), dbConn, &identifyMusicStub{
		music: func(context.Context, metadata.MusicInfo) *metadata.MusicMatchResult {
			return &metadata.MusicMatchResult{Artist: "Recovered Artist"}
		},
	}, nil, "")
	if err := scanJobs.Recover(); err != nil {
		t.Fatalf("recover scan jobs: %v", err)
	}

	select {
	case options := <-enrichmentCalled:
		if options.MusicIdentifier == nil {
			t.Fatal("expected recovered music enrichment to reuse the music identifier")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected enrichment to resume after recovery")
	}

	if got := discoveryCalls.Load(); got != 0 {
		t.Fatalf("expected recovery to skip discovery rescan, got %d calls", got)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		status := scanJobs.status(libraryID)
		if status.Phase == libraryScanPhaseCompleted && status.EnrichmentPhase == libraryEnrichmentPhaseIdle && !status.Enriching {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected enrichment recovery to settle, got %+v", status)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestLibraryScanManager_RecoverSkipsEnrichmentWhenNothingPending(t *testing.T) {
	dbConn, err := db.InitDB(filepath.Join(t.TempDir(), "plum.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	root := t.TempDir()
	albumDir := filepath.Join(root, "Artist", "Album")
	if err := os.MkdirAll(albumDir, 0o755); err != nil {
		t.Fatalf("mkdir album dir: %v", err)
	}
	trackPath := filepath.Join(albumDir, "01 - Track.flac")
	if err := os.WriteFile(trackPath, []byte("not real audio"), 0o644); err != nil {
		t.Fatalf("write track: %v", err)
	}

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "test@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "Music", db.LibraryTypeMusic, root, now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	if _, err := db.HandleScanLibraryWithOptions(context.Background(), dbConn, root, db.LibraryTypeMusic, libraryID, db.ScanOptions{
		ProbeMedia:             false,
		ProbeEmbeddedSubtitles: false,
		ScanSidecarSubtitles:   false,
		HashMode:               db.ScanHashModeInline,
	}); err != nil {
		t.Fatalf("seed music library: %v", err)
	}

	if err := db.UpsertLibraryJobStatus(dbConn, db.LibraryJobStatus{
		LibraryID:         libraryID,
		Path:              root,
		Type:              db.LibraryTypeMusic,
		Phase:             libraryScanPhaseCompleted,
		EnrichmentPhase:   libraryEnrichmentPhaseRunning,
		Enriching:         true,
		IdentifyPhase:     libraryIdentifyPhaseIdle,
		IdentifyRequested: false,
		StartedAt:         now.Add(-1 * time.Minute).Format(time.RFC3339),
		FinishedAt:        now.Add(-30 * time.Second).Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("seed library job status: %v", err)
	}

	originalDiscovery := scanLibraryDiscovery
	originalEnrichment := enrichLibraryTasks
	var discoveryCalls atomic.Int32
	var enrichmentCalls atomic.Int32
	scanLibraryDiscovery = func(
		ctx context.Context,
		dbConn *sql.DB,
		root, mediaType string,
		libraryID int,
		options db.ScanOptions,
	) (db.ScanDelta, error) {
		discoveryCalls.Add(1)
		return db.ScanDelta{}, nil
	}
	enrichLibraryTasks = func(
		ctx context.Context,
		dbConn *sql.DB,
		root, mediaType string,
		libraryID int,
		tasks []db.EnrichmentTask,
		options db.ScanOptions,
	) error {
		enrichmentCalls.Add(1)
		t.Fatalf("enrichment should not run when every row is already hashed; got %d tasks", len(tasks))
		return nil
	}
	t.Cleanup(func() {
		scanLibraryDiscovery = originalDiscovery
		enrichLibraryTasks = originalEnrichment
	})

	scanJobs := NewLibraryScanManager(context.Background(), dbConn, nil, nil, "")
	if err := scanJobs.Recover(); err != nil {
		t.Fatalf("recover scan jobs: %v", err)
	}

	if got := discoveryCalls.Load(); got != 0 {
		t.Fatalf("expected recovery to skip discovery rescan, got %d calls", got)
	}
	if got := enrichmentCalls.Load(); got != 0 {
		t.Fatalf("expected recovery to skip enrichment, got %d calls", got)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		status := scanJobs.status(libraryID)
		if status.Phase == libraryScanPhaseCompleted && status.EnrichmentPhase == libraryEnrichmentPhaseIdle && !status.Enriching {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected enrichment recovery to settle, got %+v", status)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestLibraryScanManager_RequeueDoesNotDuplicateQueuedJob(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "test@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	bigRoot := t.TempDir()
	bigDir := filepath.Join(bigRoot, "Big Show", "Season 1")
	if err := os.MkdirAll(bigDir, 0o755); err != nil {
		t.Fatalf("mkdir big dir: %v", err)
	}
	for i := 1; i <= 3; i++ {
		file := filepath.Join(bigDir, "Big Show - S01E0"+strconv.Itoa(i)+".mkv")
		if err := os.WriteFile(file, []byte("not a real video"), 0o644); err != nil {
			t.Fatalf("write big file: %v", err)
		}
	}

	smallRoot := t.TempDir()
	smallDir := filepath.Join(smallRoot, "Small Show", "Season 1")
	if err := os.MkdirAll(smallDir, 0o755); err != nil {
		t.Fatalf("mkdir small dir: %v", err)
	}
	smallFile := filepath.Join(smallDir, "Small Show - S01E01.mkv")
	if err := os.WriteFile(smallFile, []byte("not a real video"), 0o644); err != nil {
		t.Fatalf("write small file: %v", err)
	}

	scanJobs := NewLibraryScanManager(context.Background(), dbConn, nil, nil, "")
	bigStatus := scanJobs.start(1, bigRoot, db.LibraryTypeTV, false, nil)
	if bigStatus.Phase == "" {
		t.Fatal("expected big status")
	}

	firstQueued := scanJobs.start(2, smallRoot, db.LibraryTypeTV, false, nil)
	secondQueued := scanJobs.start(2, smallRoot, db.LibraryTypeTV, false, nil)

	if firstQueued.LibraryID != secondQueued.LibraryID {
		t.Fatalf("requeue returned different jobs: %+v vs %+v", firstQueued, secondQueued)
	}
	if len(scanJobs.jobs) != 2 {
		t.Fatalf("expected 2 jobs tracked, got %d", len(scanJobs.jobs))
	}

	status := scanJobs.status(2)
	if status.QueuePosition != 2 {
		t.Fatalf("queue position = %d", status.QueuePosition)
	}
}

func TestLibraryScanManager_CompletedScanAdvancesQueueWhileFirstLibraryEnriches(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	originalDiscovery := scanLibraryDiscovery
	originalEnrichment := enrichLibraryTasks
	enrichmentStarted := make(chan struct{}, 1)
	secondScanStarted := make(chan struct{}, 1)
	releaseEnrichment := make(chan struct{})
	var releaseOnce sync.Once
	firstRoot := filepath.Join(t.TempDir(), "library-a")
	secondRoot := filepath.Join(t.TempDir(), "library-b")
	scanLibraryDiscovery = func(
		ctx context.Context,
		dbConn *sql.DB,
		root, mediaType string,
		libraryID int,
		options db.ScanOptions,
	) (db.ScanDelta, error) {
		if root == secondRoot {
			select {
			case secondScanStarted <- struct{}{}:
			default:
			}
		}
		return db.ScanDelta{
			Result: db.ScanResult{Added: 1},
			TouchedFiles: []db.EnrichmentTask{{
				LibraryID: libraryID,
				Kind:      mediaType,
				Path:      filepath.Join(root, "file.mkv"),
			}},
		}, nil
	}
	enrichLibraryTasks = func(
		ctx context.Context,
		dbConn *sql.DB,
		root, mediaType string,
		libraryID int,
		tasks []db.EnrichmentTask,
		options db.ScanOptions,
	) error {
		if root == firstRoot {
			select {
			case enrichmentStarted <- struct{}{}:
			default:
			}
			<-releaseEnrichment
		}
		return nil
	}
	t.Cleanup(func() {
		scanLibraryDiscovery = originalDiscovery
		enrichLibraryTasks = originalEnrichment
		releaseOnce.Do(func() {
			close(releaseEnrichment)
		})
	})

	scanJobs := NewLibraryScanManager(context.Background(), dbConn, nil, nil, "")
	scanJobs.start(1, firstRoot, db.LibraryTypeTV, false, nil)
	scanJobs.start(2, secondRoot, db.LibraryTypeTV, false, nil)

	select {
	case <-enrichmentStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("expected first library enrichment to start")
	}

	select {
	case <-secondScanStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("expected second library scan to start while first enriches")
	}

	firstStatus := scanJobs.status(1)
	if firstStatus.Phase != libraryScanPhaseCompleted || !firstStatus.Enriching || firstStatus.EnrichmentPhase != libraryEnrichmentPhaseRunning {
		t.Fatalf("unexpected first status while second scans: %+v", firstStatus)
	}
	secondStatus := scanJobs.status(2)
	if secondStatus.Phase != libraryScanPhaseScanning && secondStatus.Phase != libraryScanPhaseCompleted {
		t.Fatalf("unexpected second status while first enriches: %+v", secondStatus)
	}

	releaseOnce.Do(func() {
		close(releaseEnrichment)
	})

	deadline := time.Now().Add(2 * time.Second)
	for {
		status := scanJobs.status(1)
		if !status.Enriching && status.EnrichmentPhase == libraryEnrichmentPhaseIdle {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected first enrichment to finish, got %+v", status)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestLibraryScanManager_StartEnrichmentMarksQueuedBeforeWorkerSlotOpens(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	manager := NewLibraryScanManager(context.Background(), dbConn, nil, nil, "")
	manager.mu.Lock()
	manager.jobs[2] = libraryScanStatus{
		LibraryID:       2,
		Phase:           libraryScanPhaseCompleted,
		IdentifyPhase:   libraryIdentifyPhaseIdle,
		MaxRetries:      3,
		StartedAt:       time.Now().UTC().Format(time.RFC3339),
		FinishedAt:      time.Now().UTC().Format(time.RFC3339),
		EnrichmentPhase: libraryEnrichmentPhaseIdle,
	}
	manager.paths[2] = "/library"
	manager.types[2] = db.LibraryTypeTV
	manager.mu.Unlock()

	manager.enrichSem <- struct{}{}
	defer func() { <-manager.enrichSem }()

	manager.startEnrichment(2, db.LibraryTypeTV, "/library", nil, []db.EnrichmentTask{{
		LibraryID: 2,
		Kind:      db.LibraryTypeTV,
		Path:      "/library/file.mkv",
	}}, false)

	status := manager.status(2)
	if status.Enriching || status.EnrichmentPhase != libraryEnrichmentPhaseQueued {
		t.Fatalf("expected queued enrichment while waiting for worker slot, got %+v", status)
	}
	manager.mu.Lock()
	cancel := manager.enrichCancels[2]
	manager.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func TestLibraryScanManager_StartEnrichmentUsesPriorityLaneForTargetedScan(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	originalEnrichment := enrichLibraryTasks
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	enrichLibraryTasks = func(
		ctx context.Context,
		dbConn *sql.DB,
		root, mediaType string,
		libraryID int,
		tasks []db.EnrichmentTask,
		options db.ScanOptions,
	) error {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		return nil
	}
	t.Cleanup(func() {
		enrichLibraryTasks = originalEnrichment
		close(release)
	})

	manager := NewLibraryScanManager(context.Background(), dbConn, nil, nil, "")
	manager.mu.Lock()
	manager.jobs[2] = libraryScanStatus{
		LibraryID:       2,
		Phase:           libraryScanPhaseCompleted,
		IdentifyPhase:   libraryIdentifyPhaseIdle,
		MaxRetries:      3,
		StartedAt:       time.Now().UTC().Format(time.RFC3339),
		FinishedAt:      time.Now().UTC().Format(time.RFC3339),
		EnrichmentPhase: libraryEnrichmentPhaseIdle,
	}
	manager.paths[2] = "/library"
	manager.types[2] = db.LibraryTypeTV
	manager.mu.Unlock()

	// Keep the standard lane occupied; targeted scans should still run via the priority lane.
	manager.enrichSem <- struct{}{}
	defer func() { <-manager.enrichSem }()

	manager.startEnrichment(2, db.LibraryTypeTV, "/library", []string{"Show A"}, []db.EnrichmentTask{{
		LibraryID: 2,
		Kind:      db.LibraryTypeTV,
		Path:      "/library/Show A/episode.mkv",
	}}, false)

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("expected targeted enrichment to start while standard lane is blocked")
	}

	status := manager.status(2)
	if !status.Enriching || status.EnrichmentPhase != libraryEnrichmentPhaseRunning {
		t.Fatalf("expected priority enrichment running, got %+v", status)
	}
}

func TestLibraryScanManager_EnrichmentFailureMarksJobFailedAndSchedulesRetry(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	originalDiscovery := scanLibraryDiscovery
	originalEnrichment := enrichLibraryTasks
	enrichErr := errors.New("hash failed")
	scanLibraryDiscovery = func(
		ctx context.Context,
		dbConn *sql.DB,
		root, mediaType string,
		libraryID int,
		options db.ScanOptions,
	) (db.ScanDelta, error) {
		return db.ScanDelta{
			Result: db.ScanResult{Added: 1},
			TouchedFiles: []db.EnrichmentTask{{
				LibraryID: libraryID,
				Kind:      mediaType,
				Path:      filepath.Join(root, "file.mkv"),
			}},
		}, nil
	}
	enrichLibraryTasks = func(
		ctx context.Context,
		dbConn *sql.DB,
		root, mediaType string,
		libraryID int,
		tasks []db.EnrichmentTask,
		options db.ScanOptions,
	) error {
		return enrichErr
	}
	t.Cleanup(func() {
		scanLibraryDiscovery = originalDiscovery
		enrichLibraryTasks = originalEnrichment
	})

	scanJobs := NewLibraryScanManager(context.Background(), dbConn, nil, nil, "")
	scanJobs.start(1, "/tv", db.LibraryTypeTV, false, nil)

	deadline := time.Now().Add(2 * time.Second)
	for {
		status := scanJobs.status(1)
		if status.Phase == libraryScanPhaseFailed {
			if status.Enriching || status.EnrichmentPhase != libraryEnrichmentPhaseIdle {
				t.Fatalf("expected enrichment to stop after failure, got %+v", status)
			}
			if status.Added != 1 || status.Processed != 1 {
				t.Fatalf("expected discovery counts to be preserved, got %+v", status)
			}
			if status.Error != enrichErr.Error() {
				t.Fatalf("error = %q, want %q", status.Error, enrichErr.Error())
			}
			if status.RetryCount != 1 || status.NextRetryAt == "" {
				t.Fatalf("expected retry to be scheduled, got %+v", status)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected enrichment failure to mark job failed, got %+v", status)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestLibraryScanManager_RunUsesOriginalIdentifyRequestForMusicEnrichment(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	originalDiscovery := scanLibraryDiscovery
	originalEnrichment := enrichLibraryTasks
	usedMusicIdentifier := make(chan bool, 1)
	scanLibraryDiscovery = func(
		ctx context.Context,
		dbConn *sql.DB,
		root, mediaType string,
		libraryID int,
		options db.ScanOptions,
	) (db.ScanDelta, error) {
		return db.ScanDelta{
			Result: db.ScanResult{Added: 1},
			TouchedFiles: []db.EnrichmentTask{{
				LibraryID: libraryID,
				Kind:      mediaType,
				Path:      filepath.Join(root, "track.flac"),
			}},
		}, nil
	}
	enrichLibraryTasks = func(
		ctx context.Context,
		dbConn *sql.DB,
		root, mediaType string,
		libraryID int,
		tasks []db.EnrichmentTask,
		options db.ScanOptions,
	) error {
		select {
		case usedMusicIdentifier <- options.MusicIdentifier != nil:
		default:
		}
		return nil
	}
	t.Cleanup(func() {
		scanLibraryDiscovery = originalDiscovery
		enrichLibraryTasks = originalEnrichment
	})

	scanJobs := NewLibraryScanManager(context.Background(), dbConn, &identifyMusicStub{}, nil, "")
	scanJobs.mu.Lock()
	scanJobs.jobs[1] = libraryScanStatus{
		LibraryID:         1,
		Phase:             libraryScanPhaseScanning,
		IdentifyRequested: true,
	}
	scanJobs.types[1] = db.LibraryTypeMusic
	scanJobs.paths[1] = "/music"
	scanJobs.mu.Unlock()

	scanJobs.run(
		1,
		libraryScanStatus{
			LibraryID:         1,
			Phase:             libraryScanPhaseScanning,
			IdentifyRequested: false,
		},
		db.LibraryTypeMusic,
		"/music",
	)

	select {
	case got := <-usedMusicIdentifier:
		if got {
			t.Fatal("expected enrichment to ignore identify requested by a queued rerun")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected enrichment to run")
	}
}

func TestLibraryScanManager_PreservesRerunPartialSubpathsWhileScanning(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	scanJobs := NewLibraryScanManager(context.Background(), dbConn, nil, nil, "")
	scanJobs.mu.Lock()
	scanJobs.jobs[1] = libraryScanStatus{LibraryID: 1, Phase: libraryScanPhaseScanning}
	scanJobs.mu.Unlock()

	scanJobs.start(1, "/tv", db.LibraryTypeTV, false, []string{"Show A"})

	got := scanJobs.reruns[1].subpaths
	if len(got) != 1 || got[0] != "Show A" {
		t.Fatalf("rerun subpaths = %#v", got)
	}
}

func TestLibraryScanManager_QueueAutomatedScanUsesSeriesFolderForTV(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	root := t.TempDir()
	seasonDir := filepath.Join(root, "Show A", "Season 1")
	if err := os.MkdirAll(seasonDir, 0o755); err != nil {
		t.Fatalf("mkdir season dir: %v", err)
	}
	episodePath := filepath.Join(seasonDir, "Show A - S01E01.mkv")
	if err := os.WriteFile(episodePath, []byte("not a real video"), 0o644); err != nil {
		t.Fatalf("write episode: %v", err)
	}
	if err := os.Remove(episodePath); err != nil {
		t.Fatalf("remove episode: %v", err)
	}

	scanJobs := NewLibraryScanManager(context.Background(), dbConn, nil, nil, "")
	scanJobs.queueAutomatedScan(1, root, db.LibraryTypeTV, episodePath)

	got := scanJobs.scanSubpaths(1)
	want := "Show A"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("scan subpaths = %#v, want [%q]", got, want)
	}
}

func TestDetectLibraryPollChanges_IgnoresUnchangedSnapshots(t *testing.T) {
	root := t.TempDir()
	showDir := filepath.Join(root, "Show A")
	if err := os.MkdirAll(showDir, 0o755); err != nil {
		t.Fatalf("mkdir show dir: %v", err)
	}
	episodePath := filepath.Join(showDir, "Show A - S01E01.mkv")
	if err := os.WriteFile(episodePath, []byte("not a real video"), 0o644); err != nil {
		t.Fatalf("write episode: %v", err)
	}

	first, err := snapshotLibraryPollState(root)
	if err != nil {
		t.Fatalf("first snapshot: %v", err)
	}
	second, err := snapshotLibraryPollState(root)
	if err != nil {
		t.Fatalf("second snapshot: %v", err)
	}

	if got := detectLibraryPollChanges(root, first, second); len(got) != 0 {
		t.Fatalf("unexpected changed paths: %#v", got)
	}
}

func TestDetectLibraryPollChanges_ReportsChangedEntries(t *testing.T) {
	root := t.TempDir()
	showDir := filepath.Join(root, "Show A")
	if err := os.MkdirAll(showDir, 0o755); err != nil {
		t.Fatalf("mkdir show dir: %v", err)
	}
	episodePath := filepath.Join(showDir, "Show A - S01E01.mkv")
	if err := os.WriteFile(episodePath, []byte("not a real video"), 0o644); err != nil {
		t.Fatalf("write episode: %v", err)
	}

	first, err := snapshotLibraryPollState(root)
	if err != nil {
		t.Fatalf("first snapshot: %v", err)
	}
	if err := os.WriteFile(episodePath, []byte("updated video bytes"), 0o644); err != nil {
		t.Fatalf("rewrite episode: %v", err)
	}
	second, err := snapshotLibraryPollState(root)
	if err != nil {
		t.Fatalf("second snapshot: %v", err)
	}

	got := detectLibraryPollChanges(root, first, second)
	if len(got) != 1 || got[0] != episodePath {
		t.Fatalf("changed paths = %#v, want [%q]", got, episodePath)
	}
}

func TestStartLibraryScan_RejectsInvalidSubpath(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "test@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	root := t.TempDir()
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`, userID, "TV", db.LibraryTypeTV, root, now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	handler := &LibraryHandler{
		DB:       dbConn,
		ScanJobs: NewLibraryScanManager(context.Background(), dbConn, nil, nil, ""),
	}
	req := httptest.NewRequest(http.MethodPost, "/api/libraries/"+strconv.Itoa(libraryID)+"/scan/start?subpath=../outside", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.StartLibraryScan(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestLibraryScanManager_StartDoesNotBlockOnEstimate(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	root := t.TempDir()
	showDir := filepath.Join(root, "Show", "Season 1")
	if err := os.MkdirAll(showDir, 0o755); err != nil {
		t.Fatalf("mkdir show dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(showDir, "Show - S01E01.mkv"), []byte("not a real video"), 0o644); err != nil {
		t.Fatalf("write episode: %v", err)
	}
	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "estimate@test.com", "hash", now).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO libraries (id, user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?, ?)`, 7, userID, "TV", db.LibraryTypeTV, root, now); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	scanJobs := NewLibraryScanManager(context.Background(), dbConn, nil, nil, "")
	startedAt := time.Now()
	status := scanJobs.start(7, root, db.LibraryTypeTV, false, nil)
	if elapsed := time.Since(startedAt); elapsed > 100*time.Millisecond {
		t.Fatalf("start took too long: %s", elapsed)
	}
	if status.LibraryID != 7 {
		t.Fatalf("unexpected status: %+v", status)
	}
	if status.EstimatedItems != 0 {
		t.Fatalf("estimated items = %d, want 0", status.EstimatedItems)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		status = scanJobs.status(7)
		if status.Phase == libraryScanPhaseCompleted {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected scan to complete, got %+v", status)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestLibraryScanManager_StartClearsPendingRetryState(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	root := t.TempDir()
	scanJobs := NewLibraryScanManager(context.Background(), dbConn, nil, nil, "")
	scanJobs.mu.Lock()
	scanJobs.jobs[7] = libraryScanStatus{
		LibraryID:   7,
		Phase:       libraryScanPhaseCompleted,
		RetryCount:  3,
		MaxRetries:  3,
		NextRetryAt: time.Now().UTC().Add(time.Minute).Format(time.RFC3339),
		Error:       "temporary failure",
		LastError:   "temporary failure",
	}
	scanJobs.retryTimers[7] = time.AfterFunc(time.Hour, func() {})
	scanJobs.mu.Unlock()

	status := scanJobs.start(7, root, db.LibraryTypeTV, false, nil)

	if status.RetryCount != 0 || status.NextRetryAt != "" || status.Error != "" || status.LastError != "" {
		t.Fatalf("unexpected retry state after start: %+v", status)
	}
	if _, ok := scanJobs.retryTimers[7]; ok {
		t.Fatal("expected pending retry timer to be cleared")
	}
}

func TestLibraryScanManager_FinishSuccessResetsRetryCount(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	scanJobs := NewLibraryScanManager(context.Background(), dbConn, nil, nil, "")
	scanJobs.mu.Lock()
	scanJobs.jobs[9] = libraryScanStatus{
		LibraryID:   9,
		Phase:       libraryScanPhaseScanning,
		RetryCount:  2,
		MaxRetries:  3,
		NextRetryAt: time.Now().UTC().Add(time.Minute).Format(time.RFC3339),
		Error:       "temporary failure",
		LastError:   "temporary failure",
	}
	scanJobs.retryTimers[9] = time.AfterFunc(time.Hour, func() {})
	scanJobs.activeScanID = 9
	scanJobs.mu.Unlock()

	scanJobs.finish(9, libraryScanPhaseCompleted, db.ScanResult{}, "")

	status := scanJobs.status(9)
	if status.RetryCount != 0 || status.NextRetryAt != "" || status.Error != "" || status.LastError != "" {
		t.Fatalf("unexpected retry state after finish: %+v", status)
	}
	if _, ok := scanJobs.retryTimers[9]; ok {
		t.Fatal("expected pending retry timer to be cleared")
	}
}

func TestLibraryScanManager_StatusWarnsWhenCompletedScanFindsNoFiles(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	scanJobs := NewLibraryScanManager(context.Background(), dbConn, nil, nil, "")
	scanJobs.jobs[7] = libraryScanStatus{
		LibraryID: 7,
		Phase:     libraryScanPhaseCompleted,
	}
	scanJobs.paths[7] = "/movies"

	status := scanJobs.status(7)

	if !strings.Contains(status.Error, "No media files were found under /movies") {
		t.Fatalf("unexpected warning: %q", status.Error)
	}
}

func TestListLibraryMedia_OmitsEmbeddedSubtitlesFromBrowsePayload(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(
		`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`,
		"test@test.com",
		"hash",
		now,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(
		`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		userID,
		"TV",
		db.LibraryTypeTV,
		"/tv",
		now,
	).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	var episodeID int
	if err := dbConn.QueryRow(
		`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		libraryID,
		"Test Show - S01E01",
		"/tv/Test Show/Season 1/Test Show - S01E01.mkv",
		0,
		db.MatchStatusLocal,
		1,
		1,
	).Scan(&episodeID); err != nil {
		t.Fatalf("insert episode: %v", err)
	}
	var mediaID int
	if err := dbConn.QueryRow(`INSERT INTO media_global (kind, ref_id) VALUES (?, ?) RETURNING id`, db.LibraryTypeTV, episodeID).
		Scan(&mediaID); err != nil {
		t.Fatalf("insert media global row: %v", err)
	}
	if _, err := dbConn.Exec(
		`INSERT INTO embedded_subtitles (media_id, stream_index, language, title) VALUES (?, ?, ?, ?)`,
		mediaID,
		3,
		"eng",
		"English",
	); err != nil {
		t.Fatalf("insert embedded subtitle: %v", err)
	}

	handler := &LibraryHandler{DB: dbConn}
	req := httptest.NewRequest(http.MethodGet, "/api/libraries/"+strconv.Itoa(libraryID)+"/media", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.ListLibraryMedia(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("expected 1 media item, got %d", len(payload.Items))
	}
	if _, exists := payload.Items[0]["embeddedSubtitles"]; exists {
		t.Fatalf("expected embeddedSubtitles to be omitted from browse payload: %#v", payload.Items[0]["embeddedSubtitles"])
	}
}

func TestListLibraryMedia_IncludesIdentifyStateOverlay(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(
		`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`,
		"identify@test.com",
		"hash",
		now,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(
		`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		userID,
		"Movies",
		db.LibraryTypeMovie,
		"/movies",
		now,
	).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	var movieID int
	moviePath := "/movies/Queued Movie (2025)/Queued Movie.mkv"
	if err := dbConn.QueryRow(
		`INSERT INTO movies (library_id, title, path, duration, match_status) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		libraryID,
		"Queued Movie",
		moviePath,
		0,
		db.MatchStatusLocal,
	).Scan(&movieID); err != nil {
		t.Fatalf("insert movie: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO media_global (kind, ref_id) VALUES (?, ?)`, db.LibraryTypeMovie, movieID); err != nil {
		t.Fatalf("insert media global row: %v", err)
	}

	tracker := newIdentifyRunTracker()
	tracker.startLibrary(libraryID, []db.IdentificationRow{{
		RefID: movieID,
		Kind:  db.LibraryTypeMovie,
		Path:  moviePath,
	}})
	tracker.setState(libraryID, db.LibraryTypeMovie, moviePath, "identifying")

	handler := &LibraryHandler{DB: dbConn, identifyRun: tracker}
	req := httptest.NewRequest(http.MethodGet, "/api/libraries/"+strconv.Itoa(libraryID)+"/media", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.ListLibraryMedia(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("expected 1 media item, got %d", len(payload.Items))
	}
	if got := payload.Items[0]["identify_state"]; got != "identifying" {
		t.Fatalf("identify_state = %#v", got)
	}
}

func TestListLibraryMedia_OmitsEmbeddedAudioTracksFromBrowsePayload(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer dbConn.Close()

	var userID int
	if err := dbConn.QueryRow(
		`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP) RETURNING id`,
		"audio@test.local",
		"hash",
		true,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	var libraryID int
	if err := dbConn.QueryRow(
		`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP) RETURNING id`,
		userID,
		"Movies",
		db.LibraryTypeMovie,
		"/movies",
	).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	var movieID int
	if err := dbConn.QueryRow(
		`INSERT INTO movies (library_id, title, path, duration, match_status) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		libraryID,
		"Track Test",
		"/movies/Track Test (2025)/Track Test.mkv",
		0,
		db.MatchStatusLocal,
	).Scan(&movieID); err != nil {
		t.Fatalf("insert movie: %v", err)
	}

	var mediaID int
	if err := dbConn.QueryRow(`INSERT INTO media_global (kind, ref_id) VALUES (?, ?) RETURNING id`, db.LibraryTypeMovie, movieID).
		Scan(&mediaID); err != nil {
		t.Fatalf("insert media global row: %v", err)
	}
	if _, err := dbConn.Exec(
		`INSERT INTO embedded_audio_tracks (media_id, stream_index, language, title) VALUES (?, ?, ?, ?)`,
		mediaID,
		2,
		"jpn",
		"Japanese Stereo",
	); err != nil {
		t.Fatalf("insert embedded audio track: %v", err)
	}

	handler := &LibraryHandler{DB: dbConn}
	req := httptest.NewRequest(http.MethodGet, "/api/libraries/"+strconv.Itoa(libraryID)+"/media", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.ListLibraryMedia(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("expected 1 media item, got %d", len(payload.Items))
	}
	if _, exists := payload.Items[0]["embeddedAudioTracks"]; exists {
		t.Fatalf("expected embeddedAudioTracks to be omitted from browse payload: %#v", payload.Items[0]["embeddedAudioTracks"])
	}
}

func TestListLibraryMedia_OmitsSubtitlesFromBrowsePayload(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer dbConn.Close()

	var userID int
	if err := dbConn.QueryRow(
		`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP) RETURNING id`,
		"subtitle@test.local",
		"hash",
		true,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	var libraryID int
	if err := dbConn.QueryRow(
		`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP) RETURNING id`,
		userID,
		"Movies",
		db.LibraryTypeMovie,
		"/movies",
	).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	var movieID int
	if err := dbConn.QueryRow(
		`INSERT INTO movies (library_id, title, path, duration, match_status) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		libraryID,
		"Subtitle Test",
		"/movies/Subtitle Test (2025)/Subtitle Test.mkv",
		0,
		db.MatchStatusLocal,
	).Scan(&movieID); err != nil {
		t.Fatalf("insert movie: %v", err)
	}

	var mediaID int
	if err := dbConn.QueryRow(`INSERT INTO media_global (kind, ref_id) VALUES (?, ?) RETURNING id`, db.LibraryTypeMovie, movieID).
		Scan(&mediaID); err != nil {
		t.Fatalf("insert media global row: %v", err)
	}
	if _, err := dbConn.Exec(
		`INSERT INTO subtitles (media_id, title, language, format, path) VALUES (?, ?, ?, ?, ?)`,
		mediaID,
		"English",
		"eng",
		"srt",
		"/movies/Subtitle Test (2025)/Subtitle Test.eng.srt",
	); err != nil {
		t.Fatalf("insert subtitle: %v", err)
	}

	handler := &LibraryHandler{DB: dbConn}
	req := httptest.NewRequest(http.MethodGet, "/api/libraries/"+strconv.Itoa(libraryID)+"/media", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.ListLibraryMedia(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("expected 1 media item, got %d", len(payload.Items))
	}
	if _, exists := payload.Items[0]["subtitles"]; exists {
		t.Fatalf("expected subtitles to be omitted from browse payload: %#v", payload.Items[0]["subtitles"])
	}
}

func TestListLibraryMedia_EmptyLibraryReturnsJSONArray(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer dbConn.Close()

	var userID int
	if err := dbConn.QueryRow(
		`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP) RETURNING id`,
		"empty@test.local",
		"hash",
		true,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	var libraryID int
	if err := dbConn.QueryRow(
		`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP) RETURNING id`,
		userID,
		"Movies",
		db.LibraryTypeMovie,
		"/movies",
	).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	handler := &LibraryHandler{DB: dbConn}
	req := httptest.NewRequest(http.MethodGet, "/api/libraries/"+strconv.Itoa(libraryID)+"/media", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.ListLibraryMedia(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Items   []map[string]any `json:"items"`
		HasMore bool             `json:"has_more"`
		Total   int              `json:"total"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Items) != 0 || payload.HasMore || payload.Total != 0 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestListLibraryMedia_PaginatesWithOffsetAndLimit(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer dbConn.Close()

	var userID int
	if err := dbConn.QueryRow(
		`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP) RETURNING id`,
		"paged@test.local",
		"hash",
		true,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	var libraryID int
	if err := dbConn.QueryRow(
		`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP) RETURNING id`,
		userID,
		"Movies",
		db.LibraryTypeMovie,
		"/movies",
	).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	titles := []string{"Movie A", "Movie B", "Movie C"}
	for _, title := range titles {
		var movieID int
		if err := dbConn.QueryRow(
			`INSERT INTO movies (library_id, title, path, duration, match_status) VALUES (?, ?, ?, ?, ?) RETURNING id`,
			libraryID,
			title,
			"/movies/"+title+".mkv",
			120,
			db.MatchStatusLocal,
		).Scan(&movieID); err != nil {
			t.Fatalf("insert movie %q: %v", title, err)
		}
		if _, err := dbConn.Exec(`INSERT INTO media_global (kind, ref_id) VALUES (?, ?)`, db.LibraryTypeMovie, movieID); err != nil {
			t.Fatalf("insert media global row: %v", err)
		}
	}

	handler := &LibraryHandler{DB: dbConn}
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/libraries/%d/media?limit=2&offset=0", libraryID), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.ListLibraryMedia(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Items      []map[string]any `json:"items"`
		NextOffset *int             `json:"next_offset"`
		HasMore    bool             `json:"has_more"`
		Total      int              `json:"total"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(payload.Items))
	}
	if payload.Total != 3 {
		t.Fatalf("total = %d", payload.Total)
	}
	if !payload.HasMore {
		t.Fatal("expected has_more to be true")
	}
	if payload.NextOffset == nil || *payload.NextOffset != 2 {
		t.Fatalf("next_offset = %#v", payload.NextOffset)
	}
	if got := payload.Items[0]["title"]; got != "Movie A" {
		t.Fatalf("first title = %#v", got)
	}
	if got := payload.Items[1]["title"]; got != "Movie B" {
		t.Fatalf("second title = %#v", got)
	}
}

func TestGetDiscover_AttachesMovieTVAndAnimeLibraryMatches(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(
		`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`,
		"discover@test.com",
		"hash",
		now,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	var movieLibraryID int
	if err := dbConn.QueryRow(
		`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		userID,
		"Movies",
		db.LibraryTypeMovie,
		"/movies",
		now,
	).Scan(&movieLibraryID); err != nil {
		t.Fatalf("insert movie library: %v", err)
	}
	var tvLibraryID int
	if err := dbConn.QueryRow(
		`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		userID,
		"TV",
		db.LibraryTypeTV,
		"/tv",
		now,
	).Scan(&tvLibraryID); err != nil {
		t.Fatalf("insert tv library: %v", err)
	}
	var animeLibraryID int
	if err := dbConn.QueryRow(
		`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		userID,
		"Anime",
		db.LibraryTypeAnime,
		"/anime",
		now,
	).Scan(&animeLibraryID); err != nil {
		t.Fatalf("insert anime library: %v", err)
	}

	if _, err := dbConn.Exec(
		`INSERT INTO movies (library_id, title, path, duration, match_status, tmdb_id) VALUES (?, ?, ?, ?, ?, ?)`,
		movieLibraryID,
		"Movie Match",
		"/movies/movie-match.mkv",
		0,
		db.MatchStatusIdentified,
		101,
	); err != nil {
		t.Fatalf("insert movie: %v", err)
	}
	if _, err := dbConn.Exec(
		`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, tmdb_id, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		tvLibraryID,
		"TV Match - S01E01 - Pilot",
		"/tv/tv-match-s01e01.mkv",
		0,
		db.MatchStatusIdentified,
		202,
		1,
		1,
	); err != nil {
		t.Fatalf("insert tv episode: %v", err)
	}
	if _, err := dbConn.Exec(
		`INSERT INTO anime_episodes (library_id, title, path, duration, match_status, tmdb_id, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		animeLibraryID,
		"Anime Match - S01E01 - Start",
		"/anime/anime-match-s01e01.mkv",
		0,
		db.MatchStatusIdentified,
		303,
		1,
		1,
	); err != nil {
		t.Fatalf("insert anime episode: %v", err)
	}

	handler := &LibraryHandler{
		DB: dbConn,
		Discover: &discoverStub{
			getDiscover: func(context.Context, string) (*metadata.DiscoverResponse, error) {
				return &metadata.DiscoverResponse{
					Shelves: []metadata.DiscoverShelf{
						{
							ID:    "trending",
							Title: "Trending Now",
							Items: []metadata.DiscoverItem{
								{MediaType: metadata.DiscoverMediaTypeMovie, TMDBID: 101, Title: "Movie Match"},
								{MediaType: metadata.DiscoverMediaTypeTV, TMDBID: 202, Title: "TV Match"},
								{MediaType: metadata.DiscoverMediaTypeTV, TMDBID: 303, Title: "Anime Match"},
							},
						},
					},
				}, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/discover", nil)
	req = req.WithContext(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}))
	rec := httptest.NewRecorder()

	handler.GetDiscover(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var payload metadata.DiscoverResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	items := payload.Shelves[0].Items
	if len(items) != 3 {
		t.Fatalf("items = %+v", items)
	}
	if got := items[0].LibraryMatches[0].Kind; got != "movie" {
		t.Fatalf("movie kind = %q", got)
	}
	if got := items[1].LibraryMatches[0].ShowKey; got != "tmdb-202" {
		t.Fatalf("tv show key = %q", got)
	}
	if got := items[2].LibraryMatches[0].LibraryType; got != db.LibraryTypeAnime {
		t.Fatalf("anime library type = %q", got)
	}
}

func TestGetDiscover_ReturnsServiceUnavailableWhenTMDBMissing(t *testing.T) {
	handler := &LibraryHandler{
		Discover: &discoverStub{
			getDiscover: func(context.Context, string) (*metadata.DiscoverResponse, error) {
				return nil, metadata.ErrTMDBNotConfigured
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/discover", nil)
	req = req.WithContext(withUser(req.Context(), &db.User{ID: 1, IsAdmin: true}))
	rec := httptest.NewRecorder()

	handler.GetDiscover(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "TMDB_API_KEY") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestGetDiscoverTitleDetails_AttachesLibraryMatch(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(
		`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`,
		"detail@test.com",
		"hash",
		now,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(
		`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		userID,
		"TV",
		db.LibraryTypeTV,
		"/tv",
		now,
	).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	if _, err := dbConn.Exec(
		`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, tmdb_id, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		libraryID,
		"Detail Match - S01E01 - Start",
		"/tv/detail-match-s01e01.mkv",
		0,
		db.MatchStatusIdentified,
		404,
		1,
		1,
	); err != nil {
		t.Fatalf("insert tv episode: %v", err)
	}

	handler := &LibraryHandler{
		DB: dbConn,
		Discover: &discoverStub{
			getDiscoverTitleDetail: func(context.Context, metadata.DiscoverMediaType, int) (*metadata.DiscoverTitleDetails, error) {
				return &metadata.DiscoverTitleDetails{
					MediaType:    metadata.DiscoverMediaTypeTV,
					TMDBID:       404,
					Title:        "Detail Match",
					Overview:     "Overview",
					Genres:       []string{"Drama"},
					Videos:       []metadata.DiscoverTitleVideo{},
					PosterPath:   "/poster.jpg",
					BackdropPath: "/backdrop.jpg",
				}, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/discover/tv/404", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("mediaType", "tv")
	rctx.URLParams.Add("tmdbId", "404")
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.GetDiscoverTitleDetails(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var payload metadata.DiscoverTitleDetails
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if len(payload.LibraryMatches) != 1 {
		t.Fatalf("library matches = %+v", payload.LibraryMatches)
	}
	if payload.LibraryMatches[0].ShowKey != "tmdb-404" {
		t.Fatalf("show key = %q", payload.LibraryMatches[0].ShowKey)
	}
}

func TestGetDiscoverGenres_ReturnsPayload(t *testing.T) {
	handler := &LibraryHandler{
		Discover: &discoverStub{
			getDiscoverGenres: func(context.Context) (*metadata.DiscoverGenresResponse, error) {
				return &metadata.DiscoverGenresResponse{
					MovieGenres: []metadata.DiscoverGenre{{ID: 28, Name: "Action"}},
					TVGenres:    []metadata.DiscoverGenre{{ID: 10765, Name: "Sci-Fi & Fantasy"}},
				}, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/discover/genres", nil)
	req = req.WithContext(withUser(req.Context(), &db.User{ID: 1, IsAdmin: true}))
	rec := httptest.NewRecorder()

	handler.GetDiscoverGenres(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var payload metadata.DiscoverGenresResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if len(payload.MovieGenres) != 1 || payload.MovieGenres[0].Name != "Action" {
		t.Fatalf("movie genres = %+v", payload.MovieGenres)
	}
	if len(payload.TVGenres) != 1 || payload.TVGenres[0].Name != "Sci-Fi & Fantasy" {
		t.Fatalf("tv genres = %+v", payload.TVGenres)
	}
}

func TestBrowseDiscover_AttachesLibraryMatches(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(
		`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`,
		"browse@test.com",
		"hash",
		now,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(
		`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		userID,
		"Movies",
		db.LibraryTypeMovie,
		"/movies",
		now,
	).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	if _, err := dbConn.Exec(
		`INSERT INTO movies (library_id, title, path, duration, match_status, tmdb_id) VALUES (?, ?, ?, ?, ?, ?)`,
		libraryID,
		"Browse Match",
		"/movies/browse-match.mkv",
		0,
		db.MatchStatusIdentified,
		515,
	); err != nil {
		t.Fatalf("insert movie: %v", err)
	}

	handler := &LibraryHandler{
		DB: dbConn,
		Discover: &discoverStub{
			browseDiscover: func(context.Context, metadata.DiscoverBrowseCategory, metadata.DiscoverMediaType, int, int, string) (*metadata.DiscoverBrowseResponse, error) {
				return &metadata.DiscoverBrowseResponse{
					Items:        []metadata.DiscoverItem{{MediaType: metadata.DiscoverMediaTypeMovie, TMDBID: 515, Title: "Browse Match"}},
					Page:         1,
					TotalPages:   3,
					TotalResults: 60,
					MediaType:    metadata.DiscoverMediaTypeMovie,
					Category:     metadata.DiscoverBrowseCategoryPopularMovies,
				}, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/discover/browse?category=popular-movies&media_type=movie&page=1", nil)
	req = req.WithContext(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}))
	rec := httptest.NewRecorder()

	handler.BrowseDiscover(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var payload metadata.DiscoverBrowseResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.TotalResults != 60 || len(payload.Items) != 1 {
		t.Fatalf("payload = %+v", payload)
	}
	if len(payload.Items[0].LibraryMatches) != 1 || payload.Items[0].LibraryMatches[0].Kind != "movie" {
		t.Fatalf("library matches = %+v", payload.Items[0].LibraryMatches)
	}
}

func TestBrowseDiscover_RejectsInvalidParams(t *testing.T) {
	handler := &LibraryHandler{
		Discover: &discoverStub{},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/discover/browse?category=bad&page=0", nil)
	req = req.WithContext(withUser(req.Context(), &db.User{ID: 1, IsAdmin: true}))
	rec := httptest.NewRecorder()

	handler.BrowseDiscover(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetDiscover_DoesNotMatchTVShowsWithoutActiveEpisodes(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(
		`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`,
		"stale-show@test.com",
		"hash",
		now,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(
		`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		userID,
		"TV",
		db.LibraryTypeTV,
		"/tv",
		now,
	).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	var showID int
	if err := dbConn.QueryRow(
		`INSERT INTO shows (library_id, kind, tmdb_id, title, title_key, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		libraryID,
		db.LibraryTypeTV,
		777,
		"Gone Show",
		"goneshow",
		now.Format(time.RFC3339),
		now.Format(time.RFC3339),
	).Scan(&showID); err != nil {
		t.Fatalf("insert show: %v", err)
	}
	if _, err := dbConn.Exec(
		`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, tmdb_id, show_id, season, episode, missing_since) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		libraryID,
		"Gone Show - S01E01 - Pilot",
		"/tv/gone-show-s01e01.mkv",
		0,
		db.MatchStatusIdentified,
		777,
		showID,
		1,
		1,
		now.Format(time.RFC3339),
	); err != nil {
		t.Fatalf("insert missing tv episode: %v", err)
	}

	handler := &LibraryHandler{
		DB: dbConn,
		Discover: &discoverStub{
			getDiscover: func(context.Context, string) (*metadata.DiscoverResponse, error) {
				return &metadata.DiscoverResponse{
					Shelves: []metadata.DiscoverShelf{
						{
							ID:    "trending",
							Title: "Trending",
							Items: []metadata.DiscoverItem{
								{MediaType: metadata.DiscoverMediaTypeTV, TMDBID: 777, Title: "Gone Show"},
							},
						},
					},
				}, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/discover", nil)
	req = req.WithContext(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}))
	rec := httptest.NewRecorder()

	handler.GetDiscover(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var payload metadata.DiscoverResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if len(payload.Shelves) != 1 || len(payload.Shelves[0].Items) != 1 {
		t.Fatalf("payload = %+v", payload)
	}
	if len(payload.Shelves[0].Items[0].LibraryMatches) != 0 {
		t.Fatalf("expected no library matches, got %+v", payload.Shelves[0].Items[0].LibraryMatches)
	}
}

func TestRefreshLibraryPlaybackTracks_EmptyMovieLibrary(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(
		`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`,
		"test@test.com",
		"hash",
		now,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(
		`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		userID,
		"Movies",
		db.LibraryTypeMovie,
		"/movies",
		now,
	).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	handler := &LibraryHandler{DB: dbConn}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/libraries/"+strconv.Itoa(libraryID)+"/playback-tracks/refresh",
		nil,
	)
	req = req.WithContext(
		context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx),
	)
	rec := httptest.NewRecorder()

	handler.RefreshLibraryPlaybackTracks(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Accepted  bool `json:"accepted"`
		LibraryID int  `json:"libraryId"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !payload.Accepted || payload.LibraryID != libraryID {
		t.Fatalf("payload = %+v", payload)
	}
	// Background goroutine uses the same DB; wait so it finishes before test cleanup closes the handle.
	time.Sleep(150 * time.Millisecond)
}

func TestRefreshLibraryPlaybackTracks_ForbiddenWrongUser(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var ownerID, otherID int
	if err := dbConn.QueryRow(
		`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`,
		"owner@test.com",
		"hash",
		now,
	).Scan(&ownerID); err != nil {
		t.Fatalf("insert owner: %v", err)
	}
	if err := dbConn.QueryRow(
		`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 0, ?) RETURNING id`,
		"other@test.com",
		"hash2",
		now,
	).Scan(&otherID); err != nil {
		t.Fatalf("insert other user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(
		`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		ownerID,
		"Movies",
		db.LibraryTypeMovie,
		"/movies",
		now,
	).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	handler := &LibraryHandler{DB: dbConn}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/libraries/"+strconv.Itoa(libraryID)+"/playback-tracks/refresh",
		nil,
	)
	req = req.WithContext(
		context.WithValue(withUser(req.Context(), &db.User{ID: otherID, IsAdmin: false}), chi.RouteCtxKey, rctx),
	)
	rec := httptest.NewRecorder()

	handler.RefreshLibraryPlaybackTracks(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSetContinueWatchingVisibilityUpdatesProgressRow(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(
		`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`,
		"cw@test.com",
		"hash",
		now,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(
		`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		userID,
		"Movies",
		db.LibraryTypeMovie,
		"/movies",
		now,
	).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	var movieID int
	if err := dbConn.QueryRow(
		`INSERT INTO movies (library_id, title, path, duration, match_status) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		libraryID,
		"Visibility Movie",
		"/movies/visibility.mkv",
		3600,
		db.MatchStatusLocal,
	).Scan(&movieID); err != nil {
		t.Fatalf("insert movie: %v", err)
	}
	var mediaID int
	if err := dbConn.QueryRow(
		`INSERT INTO media_global (kind, ref_id) VALUES (?, ?) RETURNING id`,
		db.LibraryTypeMovie,
		movieID,
	).Scan(&mediaID); err != nil {
		t.Fatalf("insert media_global: %v", err)
	}
	if err := db.UpsertPlaybackProgress(dbConn, userID, mediaID, 600, 3600, false); err != nil {
		t.Fatalf("seed progress: %v", err)
	}

	handler := &LibraryHandler{DB: dbConn}
	req := httptest.NewRequest(
		http.MethodPut,
		"/api/media/"+strconv.Itoa(mediaID)+"/continue-watching",
		strings.NewReader(`{"hidden":true}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(mediaID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.SetContinueWatchingVisibility(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var hidden int
	if err := dbConn.QueryRow(
		`SELECT COALESCE(hide_from_continue_watching, 0) FROM playback_progress WHERE user_id = ? AND media_id = ?`,
		userID,
		mediaID,
	).Scan(&hidden); err != nil {
		t.Fatalf("query progress visibility: %v", err)
	}
	if hidden != 1 {
		t.Fatalf("expected hidden=1, got %d", hidden)
	}
}

func TestClearMediaProgressResetsAndHidesContinueWatching(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(
		`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`,
		"clear-media@test.com",
		"hash",
		now,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(
		`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		userID,
		"Movies",
		db.LibraryTypeMovie,
		"/movies",
		now,
	).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	var movieID int
	if err := dbConn.QueryRow(
		`INSERT INTO movies (library_id, title, path, duration, match_status) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		libraryID,
		"Clear Progress Movie",
		"/movies/clear-progress.mkv",
		5400,
		db.MatchStatusLocal,
	).Scan(&movieID); err != nil {
		t.Fatalf("insert movie: %v", err)
	}
	var mediaID int
	if err := dbConn.QueryRow(
		`INSERT INTO media_global (kind, ref_id) VALUES (?, ?) RETURNING id`,
		db.LibraryTypeMovie,
		movieID,
	).Scan(&mediaID); err != nil {
		t.Fatalf("insert media_global: %v", err)
	}
	if err := db.UpsertPlaybackProgress(dbConn, userID, mediaID, 1200, 5400, false); err != nil {
		t.Fatalf("seed progress: %v", err)
	}

	handler := &LibraryHandler{DB: dbConn}
	req := httptest.NewRequest(http.MethodDelete, "/api/media/"+strconv.Itoa(mediaID)+"/progress", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(mediaID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.ClearMediaProgress(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var position, percent float64
	var completed, hidden int
	if err := dbConn.QueryRow(
		`SELECT position_seconds, progress_percent, completed, COALESCE(hide_from_continue_watching, 0) FROM playback_progress WHERE user_id = ? AND media_id = ?`,
		userID,
		mediaID,
	).Scan(&position, &percent, &completed, &hidden); err != nil {
		t.Fatalf("query reset progress: %v", err)
	}
	if position != 0 || percent != 0 || completed != 0 || hidden != 1 {
		t.Fatalf("unexpected reset row: position=%.2f percent=%.2f completed=%d hidden=%d", position, percent, completed, hidden)
	}
}

func TestClearShowProgressDeletesProgressForShowEpisodes(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(
		`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`,
		"clear-show@test.com",
		"hash",
		now,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(
		`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		userID,
		"TV",
		db.LibraryTypeTV,
		"/tv",
		now,
	).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	insertEpisode := func(title string, episodeNum int) int {
		t.Helper()
		var episodeID int
		if err := dbConn.QueryRow(
			`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, tmdb_id, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			libraryID,
			title,
			"/tv/"+title,
			1800,
			db.MatchStatusLocal,
			777,
			1,
			episodeNum,
		).Scan(&episodeID); err != nil {
			t.Fatalf("insert episode: %v", err)
		}
		var mediaID int
		if err := dbConn.QueryRow(
			`INSERT INTO media_global (kind, ref_id) VALUES (?, ?) RETURNING id`,
			db.LibraryTypeTV,
			episodeID,
		).Scan(&mediaID); err != nil {
			t.Fatalf("insert media_global: %v", err)
		}
		return mediaID
	}

	mediaA := insertEpisode("Clear Show - S01E01", 1)
	mediaB := insertEpisode("Clear Show - S01E02", 2)
	if err := db.UpsertPlaybackProgress(dbConn, userID, mediaA, 300, 1800, false); err != nil {
		t.Fatalf("seed progress A: %v", err)
	}
	if err := db.UpsertPlaybackProgress(dbConn, userID, mediaB, 450, 1800, false); err != nil {
		t.Fatalf("seed progress B: %v", err)
	}

	handler := &LibraryHandler{DB: dbConn}
	req := httptest.NewRequest(
		http.MethodDelete,
		"/api/libraries/"+strconv.Itoa(libraryID)+"/shows/tmdb-777/progress",
		nil,
	)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	rctx.URLParams.Add("showKey", "tmdb-777")
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.ClearShowProgress(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var remaining int
	if err := dbConn.QueryRow(
		`SELECT COUNT(*) FROM playback_progress WHERE user_id = ? AND media_id IN (?, ?)`,
		userID,
		mediaA,
		mediaB,
	).Scan(&remaining); err != nil {
		t.Fatalf("count remaining progress: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("expected no remaining show progress rows, got %d", remaining)
	}
}

func TestParseDiscoverOriginCountry(t *testing.T) {
	cases := []struct {
		raw  string
		want string
		ok   bool
	}{
		{"", "", true},
		{"US", "US", true},
		{"gb", "GB", true},
		{"  fr ", "FR", true},
		{"USA", "", false},
		{"u1", "", false},
		{"U-S", "", false},
		{"1U", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := parseDiscoverOriginCountry(tc.raw)
			if !tc.ok {
				if err == nil {
					t.Fatalf("want error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
