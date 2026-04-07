package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "modernc.org/sqlite"

	"plum/internal/arr"
	"plum/internal/db"
	"plum/internal/dotenv"
	httpapi "plum/internal/http"
	"plum/internal/metadata"
	"plum/internal/transcoder"
	"plum/internal/ws"
)

func main() {
	logWriters := []io.Writer{os.Stderr}
	logFilePath := strings.TrimSpace(getEnv("PLUM_LOG_FILE", ""))
	if logFilePath != "" {
		if err := os.MkdirAll(filepath.Dir(logFilePath), 0o755); err != nil {
			log.Fatalf("prepare log file dir: %v", err)
		}
		f, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			log.Fatalf("open log file: %v", err)
		}
		defer f.Close()
		logWriters = append(logWriters, f)
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(io.MultiWriter(logWriters...), &slog.HandlerOptions{Level: slog.LevelInfo})))

	envLoaded := dotenv.LoadIntoOSEnv()

	addr := getEnv("PLUM_ADDR", ":8080")
	conn := getEnv("PLUM_DATABASE_URL", getEnv("PLUM_DB_PATH", "./data/plum.db"))
	tmdbKey := getEnv("TMDB_API_KEY", "")
	tvdbKey := getEnv("TVDB_API_KEY", "")
	omdbKey := getEnv("OMDB_API_KEY", "")
	fanartKey := getEnv("FANART_API_KEY", "")
	musicBrainzContact := getEnv("MUSICBRAINZ_CONTACT_URL", "")

	if err := ensureDatabaseDir(conn); err != nil {
		log.Fatalf("prepare db dir: %v", err)
	}

	sqlDB, err := db.InitDB(conn)
	if err != nil {
		log.Fatalf("init db: %v", err)
	}
	defer sqlDB.Close()

	startup := startupConfig{
		Component: "server",
		Event:     "startup",
		EnvLoaded: envLoaded,
		Addr:      addr,
		DB:        conn,
		Metadata: metadataConfig{
			TMDB:               tmdbKey != "",
			TVDB:               tvdbKey != "",
			OMDB:               omdbKey != "",
			Fanart:             fanartKey != "",
			MusicBrainzContact: musicBrainzContact != "",
		},
	}

	if mediaStackSettings, err := db.GetEffectiveMediaStackSettings(sqlDB); err != nil {
		startup.MediaStack = mediaStackConfig{
			Available: false,
			Error:     err.Error(),
		}
	} else {
		mediaStackAny := arr.IsConfigured(mediaStackSettings.Radarr) || arr.IsConfigured(mediaStackSettings.SonarrTV)
		startup.MediaStack = mediaStackConfig{
			Available: true,
			Radarr:    arr.IsConfigured(mediaStackSettings.Radarr),
			SonarrTV:  arr.IsConfigured(mediaStackSettings.SonarrTV),
			Any:       mediaStackAny,
		}
	}

	if raw, err := json.Marshal(startup); err != nil {
		slog.Error("startup log marshal error", "error", err)
	} else {
		slog.Info(string(raw))
	}

	pipeline := metadata.NewPipeline(tmdbKey, tvdbKey, omdbKey, fanartKey, musicBrainzContact)
	pipeline.SetIMDbRatingProvider(&db.IMDbRatingStore{DB: sqlDB})
	pipeline.SetProviderCache(db.NewMetadataProviderCacheStore(sqlDB))

	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	hub := ws.NewHub()
	go hub.Run()
	playbackRoot := filepath.Join(os.TempDir(), "plum_playback")
	playbackSessions := transcoder.NewPlaybackSessionManager(appCtx, playbackRoot, hub)
	logDirEnv := strings.TrimSpace(getEnv("PLUM_LOG_DIR", ""))
	if err := transcoder.CleanupLegacyTranscodes(os.TempDir()); err != nil {
		slog.Warn("cleanup legacy transcodes", "error", err)
	}
	// Remove any session temp dirs left over from a previous (crashed) run.
	if err := transcoder.CleanupOrphanedSessionDirs(playbackRoot, transcoder.SessionDirCleanupMinAgeAny); err != nil {
		slog.Warn("cleanup orphaned session dirs", "error", err)
	}
	db.StartIMDbRatingsSync(appCtx, sqlDB, func(msg string, args ...any) { slog.Info(fmt.Sprintf(msg, args...)) })
	db.StartSessionCleanup(appCtx, sqlDB, func(msg string, args ...any) { slog.Info(fmt.Sprintf(msg, args...)) })
	db.StartMetadataProviderCacheCleanup(appCtx, sqlDB, func(msg string, args ...any) { slog.Info(fmt.Sprintf(msg, args...)) })

	thumbDir := getEnv("PLUM_THUMBNAILS_DIR", "")
	if thumbDir == "" {
		thumbDir = filepath.Join(filepath.Dir(conn), "thumbnails")
	}
	artDir := getEnv("PLUM_ARTWORK_DIR", "")
	if artDir == "" {
		artDir = filepath.Join(filepath.Dir(conn), "artwork")
	}

	srv := newHTTPServer(
		addr,
		buildRouter(appCtx, sqlDB, hub, playbackSessions, pipeline, thumbDir, artDir, playbackRoot, logFilePath, logDirEnv),
	)

	go func() {
		slog.Info("plum backend listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	// graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	appCancel()
	playbackSessions.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Warn("server shutdown", "error", err)
	}

	hub.Close()
}

func ensureDatabaseDir(conn string) error {
	if conn == "" || conn == ":memory:" || strings.HasPrefix(conn, "file:") {
		return nil
	}
	path := strings.SplitN(conn, "?", 2)[0]
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

type startupConfig struct {
	Component  string           `json:"component"`
	Event      string           `json:"event"`
	EnvLoaded  bool             `json:"env_loaded"`
	Addr       string           `json:"addr"`
	DB         string           `json:"db"`
	Metadata   metadataConfig   `json:"metadata"`
	MediaStack mediaStackConfig `json:"media_stack"`
}

type metadataConfig struct {
	TMDB               bool `json:"tmdb"`
	TVDB               bool `json:"tvdb"`
	OMDB               bool `json:"omdb"`
	Fanart             bool `json:"fanart"`
	MusicBrainzContact bool `json:"musicbrainz_contact"`
}

type mediaStackConfig struct {
	Available bool   `json:"available"`
	Radarr    bool   `json:"radarr,omitempty"`
	SonarrTV  bool   `json:"sonarr_tv,omitempty"`
	Any       bool   `json:"any,omitempty"`
	Error     string `json:"error,omitempty"`
}

func buildRouter(
	shutdownCtx context.Context,
	sqlDB *sql.DB,
	hub *ws.Hub,
	playbackSessions *transcoder.PlaybackSessionManager,
	pipeline *metadata.Pipeline,
	thumbDir string,
	artDir string,
	playbackRoot string,
	logFilePath string,
	logDirEnv string,
) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(httpapi.RequestLoggingMiddleware())
	allowedOrigins := httpapi.AllowedOriginsFromEnv(os.Getenv("PLUM_ALLOWED_ORIGINS"))
	r.Use(httpapi.CORSMiddleware(allowedOrigins))
	r.Use(httpapi.RequestBodyLimitMiddleware(httpapi.RequestBodyLimitBytes()))

	r.Use(httpapi.AuthMiddleware(sqlDB))

	authHandler := &httpapi.AuthHandler{DB: sqlDB}
	scanJobs := httpapi.NewLibraryScanManager(shutdownCtx, sqlDB, pipeline, hub, thumbDir)
	playbackHandler := &httpapi.PlaybackHandler{
		DB:       sqlDB,
		Sessions: playbackSessions,
		ThumbDir: thumbDir,
		ArtDir:   artDir,
	}
	searchIndex := httpapi.NewSearchIndexManager(shutdownCtx, sqlDB, pipeline, pipeline)
	mediaStack := arr.NewService()
	libHandler := &httpapi.LibraryHandler{
		DB:          sqlDB,
		Meta:        pipeline,
		Artwork:     pipeline,
		Movies:      pipeline,
		MovieQuery:  pipeline,
		MovieLookup: pipeline,
		Series:      pipeline,
		SeriesQuery: pipeline,
		Discover:    pipeline,
		Arr:         mediaStack,
		ScanJobs:    scanJobs,
		SearchIndex: searchIndex,
	}
	scanJobs.AttachHandler(libHandler)
	if err := scanJobs.Recover(); err != nil {
		slog.Warn("recover scan jobs", "error", err)
	}
	searchIndex.QueueAllLibraries(true)
	adminHandler := &httpapi.AdminHandler{
		ShutdownCtx:  shutdownCtx,
		DB:           sqlDB,
		Lib:          libHandler,
		ScanJobs:     scanJobs,
		Sessions:     playbackSessions,
		PlaybackRoot: playbackRoot,
		LogFile:      logFilePath,
		LogDir:       logDirEnv,
	}
	httpapi.StartAdminMaintenanceScheduler(shutdownCtx, adminHandler)
	transcodingSettingsHandler := &httpapi.TranscodingSettingsHandler{DB: sqlDB}
	metadataArtworkSettingsHandler := &httpapi.MetadataArtworkSettingsHandler{DB: sqlDB, Artwork: pipeline}
	mediaStackSettingsHandler := &httpapi.MediaStackSettingsHandler{DB: sqlDB, Arr: mediaStack}
	serverEnvSettingsHandler := &httpapi.ServerEnvSettingsHandler{Pipeline: pipeline}

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	r.Get("/api/setup/status", authHandler.SetupStatus)
	r.Post("/api/auth/admin-setup", authHandler.AdminSetup)
	r.Post("/api/auth/login", authHandler.Login)
	r.Post("/api/auth/device-login", authHandler.DeviceLogin)
	r.Post("/api/auth/quick-connect/redeem", authHandler.RedeemQuickConnect)
	r.Post("/api/auth/logout", authHandler.Logout)
	r.Get("/api/auth/me", authHandler.Me)

	r.Group(func(protected chi.Router) {
		protected.Use(httpapi.RequireAuth)

		protected.Group(func(admin chi.Router) {
			admin.Use(httpapi.RequireAdmin)
			admin.Get("/api/settings/transcoding", transcodingSettingsHandler.Get)
			admin.Put("/api/settings/transcoding", transcodingSettingsHandler.Put)
			admin.Get("/api/settings/metadata-artwork", metadataArtworkSettingsHandler.Get)
			admin.Put("/api/settings/metadata-artwork", metadataArtworkSettingsHandler.Put)
			admin.Get("/api/settings/media-stack", mediaStackSettingsHandler.Get)
			admin.Put("/api/settings/media-stack", mediaStackSettingsHandler.Put)
			admin.Post("/api/settings/media-stack/validate", mediaStackSettingsHandler.Validate)
			admin.Get("/api/settings/server-env", serverEnvSettingsHandler.Get)
			admin.Put("/api/settings/server-env", serverEnvSettingsHandler.Put)
			httpapi.MountAdminRoutes(admin, adminHandler)
		})

		protected.Post("/api/auth/quick-connect", authHandler.CreateQuickConnectCode)
		protected.Post("/api/libraries", libHandler.CreateLibrary)
		protected.Get("/api/libraries", libHandler.ListLibraries)
		protected.Get("/api/libraries/unidentified-summary", libHandler.ListUnidentifiedLibrarySummaries)
		protected.Get("/api/libraries/intro-summary", libHandler.GetIntroScanSummary)
		protected.Get("/api/libraries/intro-summary/refresh-status", libHandler.GetIntroRefreshStatus)
		protected.Put("/api/libraries/{id}/playback-preferences", libHandler.UpdateLibraryPlaybackPreferences)
		protected.Get("/api/libraries/{id}/intro-summary/shows", libHandler.GetIntroScanShowSummary)
		protected.Get("/api/home", libHandler.GetHomeDashboard)
		protected.Get("/api/downloads", libHandler.GetDownloads)
		protected.Post("/api/downloads/remove", libHandler.RemoveDownload)
		protected.Get("/api/discover", libHandler.GetDiscover)
		protected.Get("/api/discover/genres", libHandler.GetDiscoverGenres)
		protected.Get("/api/discover/browse", libHandler.BrowseDiscover)
		protected.Get("/api/discover/search", libHandler.SearchDiscover)
		protected.Get("/api/discover/{mediaType}/{tmdbId}", libHandler.GetDiscoverTitleDetails)
		protected.Post("/api/discover/{mediaType}/{tmdbId}/add", libHandler.AddDiscoverTitle)
		protected.Get("/api/search", libHandler.SearchLibraryMedia)
		protected.Get("/api/libraries/{id}/scan", libHandler.GetLibraryScanStatus)
		protected.Post("/api/libraries/{id}/scan", libHandler.ScanLibrary)
		protected.Post("/api/libraries/{id}/scan/start", libHandler.StartLibraryScan)
		protected.Post("/api/libraries/{id}/identify", libHandler.IdentifyLibrary)
		protected.Get("/api/libraries/{id}/media", libHandler.ListLibraryMedia)
		protected.Post("/api/libraries/{id}/playback-tracks/refresh", libHandler.RefreshLibraryPlaybackTracks)
		protected.Post("/api/libraries/{id}/intro/refresh", libHandler.RefreshLibraryIntroOnly)
		protected.Post("/api/libraries/{id}/intro/chromaprint-scan", libHandler.PostLibraryIntroChromaprintScan)
		protected.Get("/api/libraries/{id}/movies/{mediaId}", libHandler.GetLibraryMovieDetails)
		protected.Get("/api/libraries/{id}/movies/{mediaId}/artwork/poster/candidates", libHandler.GetMoviePosterCandidates)
		protected.Put("/api/libraries/{id}/movies/{mediaId}/artwork/poster", libHandler.SetMoviePosterSelection)
		protected.Delete("/api/libraries/{id}/movies/{mediaId}/artwork/poster", libHandler.ResetMoviePosterSelection)
		protected.Post("/api/libraries/{id}/movies/identify", libHandler.IdentifyMovie)
		protected.Get("/api/libraries/{id}/shows/{showKey}/details", libHandler.GetLibraryShowDetails)
		protected.Get("/api/libraries/{id}/shows/{showKey}/episodes", libHandler.GetLibraryShowEpisodes)
		protected.Delete("/api/libraries/{id}/shows/{showKey}/progress", libHandler.ClearShowProgress)
		protected.Put("/api/libraries/{id}/shows/{showKey}/watched", libHandler.MarkShowWatched)
		protected.Get("/api/libraries/{id}/shows/{showKey}/artwork/poster/candidates", libHandler.GetShowPosterCandidates)
		protected.Put("/api/libraries/{id}/shows/{showKey}/artwork/poster", libHandler.SetShowPosterSelection)
		protected.Delete("/api/libraries/{id}/shows/{showKey}/artwork/poster", libHandler.ResetShowPosterSelection)
		protected.Post("/api/libraries/{id}/shows/refresh", libHandler.RefreshShow)
		protected.Post("/api/libraries/{id}/shows/identify", libHandler.IdentifyShow)
		protected.Post("/api/libraries/{id}/shows/confirm", libHandler.ConfirmShow)

		protected.Get("/api/movies/search", libHandler.GetMovieSearch)
		protected.Get("/api/series/search", libHandler.GetSeriesSearch)
		protected.Get("/api/series/{tmdbId}", libHandler.GetSeriesDetails)

		protected.Get("/api/media", playbackHandler.ListMedia)
		protected.Put("/api/media/{id}/progress", libHandler.UpdateMediaProgress)
		protected.Delete("/api/media/{id}/progress", libHandler.ClearMediaProgress)
		protected.Put("/api/media/{id}/continue-watching", libHandler.SetContinueWatchingVisibility)
		protected.Post("/api/media/{id}/playback-tracks/refresh", playbackHandler.RefreshPlaybackTracks)
		protected.Patch("/api/media/{id}/intro", playbackHandler.PatchMediaIntro)
		protected.Post("/api/media/{id}/embedded-subtitles/warm-cache", playbackHandler.WarmEmbeddedSubtitleCaches)
		protected.Post("/api/playback/sessions/{id}", playbackHandler.CreateSession)
		protected.Patch("/api/playback/sessions/{sessionId}/audio", playbackHandler.UpdateSessionAudio)
		protected.Delete("/api/playback/sessions/{sessionId}", playbackHandler.CloseSession)
		protected.Get("/api/playback/sessions/{sessionId}/revisions/{revision}/*", playbackHandler.ServeSessionRevision)
		protected.Head("/api/playback/sessions/{sessionId}/revisions/{revision}/*", playbackHandler.ServeSessionRevision)
		protected.Get("/api/stream/{id}", playbackHandler.StreamMedia)
		protected.Head("/api/stream/{id}", playbackHandler.StreamMedia)
		protected.Get("/api/media/{id}/subtitles/embedded/{index}/sup", playbackHandler.StreamEmbeddedSubtitleSup)
		protected.Head("/api/media/{id}/subtitles/embedded/{index}/sup", playbackHandler.StreamEmbeddedSubtitleSup)
		protected.Get("/api/media/{id}/subtitles/embedded/{index}/ass", playbackHandler.StreamEmbeddedSubtitleAss)
		protected.Head("/api/media/{id}/subtitles/embedded/{index}/ass", playbackHandler.StreamEmbeddedSubtitleAss)
		protected.Get("/api/media/{id}/subtitles/embedded/{index}", playbackHandler.StreamEmbeddedSubtitle)
		protected.Head("/api/media/{id}/subtitles/embedded/{index}", playbackHandler.StreamEmbeddedSubtitle)
		protected.Get("/api/subtitles/{id}/ass", playbackHandler.StreamSubtitleAss)
		protected.Head("/api/subtitles/{id}/ass", playbackHandler.StreamSubtitleAss)
		protected.Get("/api/subtitles/{id}", playbackHandler.StreamSubtitle)
		protected.Head("/api/subtitles/{id}", playbackHandler.StreamSubtitle)
		protected.Get("/api/media/{id}/thumbnail", playbackHandler.ServeThumbnail)
		protected.Head("/api/media/{id}/thumbnail", playbackHandler.ServeThumbnail)
		protected.Get("/api/media/{id}/artwork/{kind}", playbackHandler.ServeArtwork)
		protected.Head("/api/media/{id}/artwork/{kind}", playbackHandler.ServeArtwork)
		protected.Get("/api/libraries/{id}/shows/{showKey}/artwork/poster", playbackHandler.ServeShowArtwork)
		protected.Head("/api/libraries/{id}/shows/{showKey}/artwork/poster", playbackHandler.ServeShowArtwork)
	})

	r.Get("/ws", httpapi.ServeWebSocket(hub, playbackSessions, allowedOrigins))

	return r
}

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		// Streaming handlers clear the write deadline via httputil.ClearStreamWriteDeadline.
		WriteTimeout: 30 * time.Second,
	}
}
