package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"plum/internal/ffopts"
	"plum/internal/metadata"
)

// SkipFFprobeInScan is set by tests to skip ffprobe during scan (avoids blocking on fake files).
var SkipFFprobeInScan bool

var (
	showKeyNonAlnumRegexp = regexp.MustCompile(`[^a-z0-9]+`)
	showNameFromTitleRegexp = regexp.MustCompile(`^(.+?)\s*-\s*S\d+`)
)

const (
	MatchStatusIdentified = "identified"
	MatchStatusLocal      = "local"
	MatchStatusUnmatched  = "unmatched"
)

type ScanResult struct {
	Added     int `json:"added"`
	Updated   int `json:"updated"`
	Removed   int `json:"removed"`
	Unmatched int `json:"unmatched"`
	Skipped   int `json:"skipped"`
}

type ScanProgress struct {
	Processed int
	Result    ScanResult
}

type ScanActivity struct {
	Phase  string
	Target string
	Path   string
}

type ScanHashMode string

const (
	ScanHashModeInline ScanHashMode = "inline"
	ScanHashModeDefer  ScanHashMode = "defer"
)

type ScanOptions struct {
	Identifier             metadata.Identifier
	MusicIdentifier        metadata.MusicIdentifier
	ProbeMedia             bool
	ProbeEmbeddedSubtitles bool
	ScanSidecarSubtitles   bool
	Subpaths               []string
	Progress               func(ScanProgress)
	Activity               func(ScanActivity)
	HashMode               ScanHashMode
}

type EnrichmentTask struct {
	LibraryID int
	Kind      string
	RefID     int
	GlobalID  int
	Path      string
}

type ScanDelta struct {
	Result       ScanResult
	TouchedFiles []EnrichmentTask
}

var (
	videoExtensions = map[string]struct{}{
		".mp4": {}, ".mkv": {}, ".mov": {}, ".avi": {}, ".webm": {}, ".ts": {}, ".m4v": {},
	}
	audioExtensions = map[string]struct{}{
		".mp3": {}, ".flac": {}, ".m4a": {}, ".aac": {}, ".ogg": {}, ".opus": {}, ".wav": {}, ".alac": {},
	}
	readAudioMetadata = metadata.ReadAudioMetadata
	readVideoMetadata = probeVideoMetadata
	computeMediaHash  = computeFileHash
)

type Subtitle struct {
	ID       int    `json:"id"`
	MediaID  int    `json:"-"`
	Title    string `json:"title"`
	Language string `json:"language"`
	Format   string `json:"format"`
	Path     string `json:"-"`
}

type EmbeddedSubtitle struct {
	MediaID     int    `json:"-"`
	StreamIndex int    `json:"streamIndex"`
	Language    string `json:"language"`
	Title       string `json:"title"`
	Codec       string `json:"codec,omitempty"`
	Supported   *bool  `json:"supported,omitempty"`
}

type EmbeddedAudioTrack struct {
	MediaID     int    `json:"-"`
	StreamIndex int    `json:"streamIndex"`
	Language    string `json:"language"`
	Title       string `json:"title"`
}

type MediaItem struct {
	ID                        int                  `json:"id"`
	LibraryID                 int                  `json:"library_id"`
	Title                     string               `json:"title"`
	Path                      string               `json:"path"`
	Duration                  int                  `json:"duration"`
	Type                      string               `json:"type"`
	MatchStatus               string               `json:"match_status,omitempty"`
	IdentifyState             string               `json:"identify_state,omitempty"`
	Subtitles                 []Subtitle           `json:"subtitles"`
	EmbeddedSubtitles         []EmbeddedSubtitle   `json:"embeddedSubtitles"`
	EmbeddedAudioTracks       []EmbeddedAudioTrack `json:"embeddedAudioTracks"`
	TMDBID                    int                  `json:"tmdb_id"`
	TVDBID                    string               `json:"tvdb_id,omitempty"`
	Overview                  string               `json:"overview"`
	PosterPath                string               `json:"poster_path"`
	BackdropPath              string               `json:"backdrop_path"`
	PosterURL                 string               `json:"poster_url,omitempty"`
	BackdropURL               string               `json:"backdrop_url,omitempty"`
	ShowPosterPath            string               `json:"show_poster_path,omitempty"`
	ShowPosterURL             string               `json:"show_poster_url,omitempty"`
	ReleaseDate               string               `json:"release_date"`
	ShowVoteAverage           float64              `json:"show_vote_average,omitempty"`
	ShowIMDbRating            float64              `json:"show_imdb_rating,omitempty"`
	VoteAverage               float64              `json:"vote_average"`
	IMDbID                    string               `json:"imdb_id,omitempty"`
	IMDbRating                float64              `json:"imdb_rating,omitempty"`
	Artist                    string               `json:"artist,omitempty"`
	Album                     string               `json:"album,omitempty"`
	AlbumArtist               string               `json:"album_artist,omitempty"`
	DiscNumber                int                  `json:"disc_number,omitempty"`
	TrackNumber               int                  `json:"track_number,omitempty"`
	ReleaseYear               int                  `json:"release_year,omitempty"`
	MusicBrainzArtistID       string               `json:"-"`
	MusicBrainzReleaseGroupID string               `json:"-"`
	MusicBrainzReleaseID      string               `json:"-"`
	MusicBrainzRecordingID    string               `json:"-"`
	ProgressSeconds           float64              `json:"progress_seconds,omitempty"`
	ProgressPercent           float64              `json:"progress_percent,omitempty"`
	RemainingSeconds          float64              `json:"remaining_seconds,omitempty"`
	Completed                 bool                 `json:"completed,omitempty"`
	LastWatchedAt             string               `json:"last_watched_at,omitempty"`
	// Season and Episode are set for tv/anime episodes; 0 when not applicable.
	Season  int `json:"season,omitempty"`
	Episode int `json:"episode,omitempty"`
	// ShowID is the internal shows.id for tv/anime episodes when linked; 0 when unset.
	ShowID int `json:"show_id,omitempty"`
	// MetadataReviewNeeded marks an auto-picked episodic match that still needs user confirmation.
	MetadataReviewNeeded bool `json:"metadata_review_needed,omitempty"`
	// MetadataConfirmed marks episodic metadata that the user explicitly accepted.
	MetadataConfirmed bool `json:"metadata_confirmed,omitempty"`
	// ThumbnailPath is set for video items when a frame thumbnail has been generated (e.g. episode still).
	ThumbnailPath  string `json:"thumbnail_path,omitempty"`
	ThumbnailURL   string `json:"thumbnail_url,omitempty"`
	Missing        bool   `json:"missing,omitempty"`
	MissingSince   string `json:"missing_since,omitempty"`
	Duplicate      bool   `json:"duplicate,omitempty"`
	DuplicateCount int    `json:"duplicate_count,omitempty"`
	// IntroStartSeconds/IntroEndSeconds come from the primary media file's chapter metadata (ffprobe).
	IntroStartSeconds *float64 `json:"intro_start_seconds,omitempty"`
	IntroEndSeconds   *float64 `json:"intro_end_seconds,omitempty"`

	FileSizeBytes int64  `json:"-"`
	FileModTime   string `json:"-"`
	FileHash      string `json:"-"`
	FileHashKind  string `json:"-"`
}

type LibraryMediaPage struct {
	Items      []MediaItem `json:"items"`
	NextOffset *int        `json:"next_offset,omitempty"`
	HasMore    bool        `json:"has_more"`
	Total      int         `json:"total,omitempty"`
}

type PlaybackTrackMetadata struct {
	Subtitles           []Subtitle           `json:"subtitles"`
	EmbeddedSubtitles   []EmbeddedSubtitle   `json:"embeddedSubtitles"`
	EmbeddedAudioTracks []EmbeddedAudioTrack `json:"embeddedAudioTracks"`
}

type User struct {
	ID           int       `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	IsAdmin      bool      `json:"is_admin"`
	CreatedAt    time.Time `json:"created_at"`
}

type Session struct {
	ID        string    `json:"id"`
	UserID    int       `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Library holds a user-defined media library. Type must be one of: "tv", "movie", "music".
// TV and movie libraries use TMDB for metadata; music libraries do not.
type Library struct {
	ID                        int       `json:"id"`
	UserID                    int       `json:"user_id"`
	Name                      string    `json:"name"`
	Type                      string    `json:"type"`
	Path                      string    `json:"path"`
	PreferredAudioLanguage    string    `json:"preferred_audio_language,omitempty"`
	PreferredSubtitleLanguage string    `json:"preferred_subtitle_language,omitempty"`
	SubtitlesEnabledByDefault bool      `json:"subtitles_enabled_by_default,omitempty"`
	WatcherEnabled            bool      `json:"watcher_enabled,omitempty"`
	WatcherMode               string    `json:"watcher_mode,omitempty"`
	ScanIntervalMinutes       int       `json:"scan_interval_minutes,omitempty"`
	IntroSkipMode             string    `json:"intro_skip_mode,omitempty"`
	CreatedAt                 time.Time `json:"created_at"`
}

// ValidLibraryTypes are the allowed Library.Type values used for identification and scanning.
// Each type maps to a separate table (movies, tv_episodes, anime_episodes, music_tracks).
const (
	LibraryTypeTV    = "tv"
	LibraryTypeMovie = "movie"
	LibraryTypeMusic = "music"
	LibraryTypeAnime = "anime"

	LibraryWatcherModeAuto = "auto"
	LibraryWatcherModePoll = "poll"
)

// sqlitePragmas are applied to every new connection via the DSN so pool connections
// all have foreign_keys and busy_timeout set (connection-specific in SQLite).
//
// cache_size: negative value is a limit in KiB (here ~64 MiB page cache).
// mmap_size: bytes of DB file mapped read-only; improves cold reads on local disks (avoid huge values on network FS).
// After very large imports, running ANALYZE once can help the planner; Plum does not run it automatically.
const sqlitePragmas = "_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)" +
	"&_pragma=cache_size(-65536)&_pragma=mmap_size(67108864)"

func InitDB(conn string) (*sql.DB, error) {
	if conn == "" {
		conn = "./data/plum.db"
	}
	dsn := conn
	if strings.Contains(dsn, "?") {
		dsn += "&" + sqlitePragmas
	} else {
		dsn += "?" + sqlitePragmas
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	if err := createSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := ensureMetadataRefreshPolicyDefaults(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := ensureMetadataArtworkSettingsDefaults(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func createSchema(db *sql.DB) error {
	const schema = `
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  is_admin INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
  id TEXT PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at DATETIME NOT NULL,
  expires_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);

CREATE TABLE IF NOT EXISTS libraries (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  type TEXT NOT NULL CHECK (type IN ('tv','movie','music','anime')),
  path TEXT NOT NULL,
  preferred_audio_language TEXT,
  preferred_subtitle_language TEXT,
  subtitles_enabled_by_default INTEGER,
  watcher_enabled INTEGER NOT NULL DEFAULT 0,
  watcher_mode TEXT NOT NULL DEFAULT 'auto',
  scan_interval_minutes INTEGER NOT NULL DEFAULT 0,
  intro_skip_mode TEXT NOT NULL DEFAULT 'manual',
  created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_libraries_user_id ON libraries(user_id);

-- media_global maps API global id -> (kind, ref_id) for the category table.
CREATE TABLE IF NOT EXISTS media_global (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  kind TEXT NOT NULL CHECK (kind IN ('movie','tv','anime','music')),
  ref_id INTEGER NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_media_global_kind_ref ON media_global(kind, ref_id);

-- Category tables: each library type writes only to its own table.
CREATE TABLE IF NOT EXISTS movies (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  library_id INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  path TEXT NOT NULL,
  duration INTEGER NOT NULL DEFAULT 0,
  file_size_bytes INTEGER NOT NULL DEFAULT 0,
  file_mod_time TEXT,
  file_hash TEXT,
  file_hash_kind TEXT,
  last_seen_at TEXT,
  missing_since TEXT,
  match_status TEXT NOT NULL DEFAULT 'local',
  tmdb_id INTEGER,
  tvdb_id TEXT,
  overview TEXT,
  poster_path TEXT,
  poster_locked INTEGER NOT NULL DEFAULT 0,
  backdrop_path TEXT,
  release_date TEXT,
  vote_average REAL DEFAULT 0,
  imdb_id TEXT,
  imdb_rating REAL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_movies_library_id ON movies(library_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_movies_library_path ON movies(library_id, path);

CREATE TABLE IF NOT EXISTS shows (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  library_id INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
  kind TEXT NOT NULL CHECK (kind IN ('tv','anime')),
  tmdb_id INTEGER,
  tvdb_id TEXT,
  title TEXT NOT NULL,
  title_key TEXT NOT NULL,
  overview TEXT,
  poster_path TEXT,
  poster_locked INTEGER NOT NULL DEFAULT 0,
  backdrop_path TEXT,
  first_air_date TEXT,
  vote_average REAL DEFAULT 0,
  imdb_id TEXT,
  imdb_rating REAL DEFAULT 0,
  metadata_version INTEGER NOT NULL DEFAULT 1,
  metadata_hash TEXT,
  last_refreshed_at TEXT,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_shows_library_kind_tmdb_id ON shows(library_id, kind, tmdb_id) WHERE tmdb_id IS NOT NULL AND tmdb_id > 0;
CREATE UNIQUE INDEX IF NOT EXISTS idx_shows_library_kind_title_key ON shows(library_id, kind, title_key) WHERE tmdb_id IS NULL OR tmdb_id <= 0;
CREATE INDEX IF NOT EXISTS idx_shows_library_kind ON shows(library_id, kind);

CREATE TABLE IF NOT EXISTS seasons (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  show_id INTEGER NOT NULL REFERENCES shows(id) ON DELETE CASCADE,
  season_number INTEGER NOT NULL,
  title TEXT,
  overview TEXT,
  poster_path TEXT,
  air_date TEXT,
  metadata_version INTEGER NOT NULL DEFAULT 1,
  metadata_hash TEXT,
  last_refreshed_at TEXT,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_seasons_show_number ON seasons(show_id, season_number);
CREATE INDEX IF NOT EXISTS idx_seasons_show_id ON seasons(show_id);

CREATE TABLE IF NOT EXISTS tv_episodes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  library_id INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  path TEXT NOT NULL,
  duration INTEGER NOT NULL DEFAULT 0,
  file_size_bytes INTEGER NOT NULL DEFAULT 0,
  file_mod_time TEXT,
  file_hash TEXT,
  file_hash_kind TEXT,
  last_seen_at TEXT,
  missing_since TEXT,
  match_status TEXT NOT NULL DEFAULT 'local',
  tmdb_id INTEGER,
  tvdb_id TEXT,
  overview TEXT,
  poster_path TEXT,
  backdrop_path TEXT,
  release_date TEXT,
  vote_average REAL DEFAULT 0,
  imdb_id TEXT,
  imdb_rating REAL DEFAULT 0,
  metadata_review_needed INTEGER NOT NULL DEFAULT 0,
  metadata_confirmed INTEGER NOT NULL DEFAULT 0,
  show_id INTEGER REFERENCES shows(id) ON DELETE SET NULL,
  season_id INTEGER REFERENCES seasons(id) ON DELETE SET NULL,
  metadata_version INTEGER NOT NULL DEFAULT 1,
  metadata_content_hash TEXT,
  last_metadata_refresh_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_tv_episodes_library_id ON tv_episodes(library_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_tv_episodes_library_path ON tv_episodes(library_id, path);

CREATE TABLE IF NOT EXISTS anime_episodes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  library_id INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  path TEXT NOT NULL,
  duration INTEGER NOT NULL DEFAULT 0,
  file_size_bytes INTEGER NOT NULL DEFAULT 0,
  file_mod_time TEXT,
  file_hash TEXT,
  file_hash_kind TEXT,
  last_seen_at TEXT,
  missing_since TEXT,
  match_status TEXT NOT NULL DEFAULT 'local',
  tmdb_id INTEGER,
  tvdb_id TEXT,
  overview TEXT,
  poster_path TEXT,
  backdrop_path TEXT,
  release_date TEXT,
  vote_average REAL DEFAULT 0,
  imdb_id TEXT,
  imdb_rating REAL DEFAULT 0,
  metadata_review_needed INTEGER NOT NULL DEFAULT 0,
  metadata_confirmed INTEGER NOT NULL DEFAULT 0,
  show_id INTEGER REFERENCES shows(id) ON DELETE SET NULL,
  season_id INTEGER REFERENCES seasons(id) ON DELETE SET NULL,
  metadata_version INTEGER NOT NULL DEFAULT 1,
  metadata_content_hash TEXT,
  last_metadata_refresh_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_anime_episodes_library_id ON anime_episodes(library_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_anime_episodes_library_path ON anime_episodes(library_id, path);

CREATE TABLE IF NOT EXISTS music_tracks (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  library_id INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  path TEXT NOT NULL,
  duration INTEGER NOT NULL DEFAULT 0,
  file_size_bytes INTEGER NOT NULL DEFAULT 0,
  file_mod_time TEXT,
  file_hash TEXT,
  file_hash_kind TEXT,
  last_seen_at TEXT,
  missing_since TEXT,
  match_status TEXT NOT NULL DEFAULT 'local',
  artist TEXT,
  album TEXT,
  album_artist TEXT,
  poster_path TEXT,
  musicbrainz_artist_id TEXT,
  musicbrainz_release_group_id TEXT,
  musicbrainz_release_id TEXT,
  musicbrainz_recording_id TEXT,
  disc_number INTEGER NOT NULL DEFAULT 0,
  track_number INTEGER NOT NULL DEFAULT 0,
  release_year INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_music_tracks_library_id ON music_tracks(library_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_music_tracks_library_path ON music_tracks(library_id, path);

CREATE TABLE IF NOT EXISTS subtitles (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  media_id INTEGER NOT NULL,
  title TEXT NOT NULL,
  language TEXT NOT NULL,
  format TEXT NOT NULL,
  path TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_subtitles_media_id ON subtitles(media_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_subtitles_path ON subtitles(path);

CREATE TABLE IF NOT EXISTS embedded_subtitles (
  media_id INTEGER NOT NULL,
  stream_index INTEGER NOT NULL,
  language TEXT NOT NULL,
  title TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_embedded_subtitles_media_stream ON embedded_subtitles(media_id, stream_index);

CREATE TABLE IF NOT EXISTS embedded_audio_tracks (
  media_id INTEGER NOT NULL,
  stream_index INTEGER NOT NULL,
  language TEXT NOT NULL,
  title TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_embedded_audio_tracks_media_id ON embedded_audio_tracks(media_id);

CREATE TABLE IF NOT EXISTS media_files (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  media_id INTEGER NOT NULL REFERENCES media_global(id) ON DELETE CASCADE,
  path TEXT NOT NULL UNIQUE,
  file_size_bytes INTEGER NOT NULL DEFAULT 0,
  file_mod_time TEXT,
  file_hash TEXT,
  file_hash_kind TEXT,
  duration INTEGER NOT NULL DEFAULT 0,
  missing_since TEXT,
  last_seen_at TEXT,
  is_primary INTEGER NOT NULL DEFAULT 1,
  intro_start_sec REAL,
  intro_end_sec REAL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_media_files_media_id ON media_files(media_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_media_files_primary_media_id ON media_files(media_id) WHERE is_primary = 1;
CREATE INDEX IF NOT EXISTS idx_media_files_hash ON media_files(file_hash) WHERE file_hash IS NOT NULL AND file_hash != '';

CREATE TABLE IF NOT EXISTS artwork_assets (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  source_url TEXT NOT NULL,
  artwork_kind TEXT NOT NULL CHECK (artwork_kind IN ('poster','backdrop')),
  source_etag TEXT,
  content_hash TEXT,
  mime_type TEXT,
  width INTEGER NOT NULL DEFAULT 0,
  height INTEGER NOT NULL DEFAULT 0,
  original_rel_path TEXT NOT NULL,
  last_fetched_at TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_artwork_assets_source ON artwork_assets(source_url, artwork_kind);
CREATE INDEX IF NOT EXISTS idx_artwork_assets_hash ON artwork_assets(content_hash) WHERE content_hash IS NOT NULL AND content_hash != '';

CREATE TABLE IF NOT EXISTS artwork_variants (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  asset_id INTEGER NOT NULL REFERENCES artwork_assets(id) ON DELETE CASCADE,
  profile TEXT NOT NULL,
  rel_path TEXT NOT NULL,
  width INTEGER NOT NULL DEFAULT 0,
  height INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_artwork_variants_asset_profile ON artwork_variants(asset_id, profile);

CREATE TABLE IF NOT EXISTS artwork_links (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  entity_kind TEXT NOT NULL,
  entity_id INTEGER NOT NULL,
  artwork_kind TEXT NOT NULL CHECK (artwork_kind IN ('poster','backdrop')),
  asset_id INTEGER NOT NULL REFERENCES artwork_assets(id) ON DELETE CASCADE,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_artwork_links_entity_kind ON artwork_links(entity_kind, entity_id, artwork_kind);

CREATE TABLE IF NOT EXISTS app_settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS imdb_ratings (
  imdb_id TEXT PRIMARY KEY,
  rating REAL NOT NULL DEFAULT 0,
  votes INTEGER NOT NULL DEFAULT 0,
  updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_imdb_ratings_votes ON imdb_ratings(votes DESC);

CREATE TABLE IF NOT EXISTS library_job_status (
  library_id INTEGER PRIMARY KEY REFERENCES libraries(id) ON DELETE CASCADE,
  phase TEXT NOT NULL,
  enrichment_phase TEXT NOT NULL DEFAULT 'idle',
  enriching INTEGER NOT NULL DEFAULT 0,
  identify_phase TEXT NOT NULL DEFAULT 'idle',
  identified INTEGER NOT NULL DEFAULT 0,
  identify_failed INTEGER NOT NULL DEFAULT 0,
  processed INTEGER NOT NULL DEFAULT 0,
  added INTEGER NOT NULL DEFAULT 0,
  updated INTEGER NOT NULL DEFAULT 0,
  removed INTEGER NOT NULL DEFAULT 0,
  unmatched INTEGER NOT NULL DEFAULT 0,
  skipped INTEGER NOT NULL DEFAULT 0,
  identify_requested INTEGER NOT NULL DEFAULT 0,
  queued_at DATETIME,
  estimated_items INTEGER NOT NULL DEFAULT 0,
  error TEXT,
  retry_count INTEGER NOT NULL DEFAULT 0,
  max_retries INTEGER NOT NULL DEFAULT 3,
  next_retry_at DATETIME,
  last_error TEXT,
  next_scheduled_at DATETIME,
  started_at DATETIME,
  finished_at DATETIME,
  updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS playback_progress (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  media_id INTEGER NOT NULL REFERENCES media_global(id) ON DELETE CASCADE,
  position_seconds REAL NOT NULL DEFAULT 0,
  duration_seconds REAL NOT NULL DEFAULT 0,
  progress_percent REAL NOT NULL DEFAULT 0,
  completed INTEGER NOT NULL DEFAULT 0,
  last_watched_at DATETIME,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  PRIMARY KEY (user_id, media_id)
);
CREATE INDEX IF NOT EXISTS idx_playback_progress_user_last_watched ON playback_progress(user_id, last_watched_at DESC);

CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  applied_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS metadata_provider_cache (
  provider TEXT NOT NULL,
  method TEXT NOT NULL,
  url_path TEXT NOT NULL,
  query_hash TEXT NOT NULL,
  body_hash TEXT NOT NULL,
  response_json BLOB NOT NULL,
  fetched_at TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  schema_version INTEGER NOT NULL DEFAULT 1,
  content_hash TEXT NOT NULL,
  status_code INTEGER NOT NULL,
  last_accessed_at TEXT NOT NULL,
  hit_count INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (provider, method, url_path, query_hash, body_hash)
);
CREATE INDEX IF NOT EXISTS idx_metadata_provider_cache_expires_at ON metadata_provider_cache(expires_at);
`
	if _, err := db.Exec(schema); err != nil {
		return err
	}
	return applySchemaMigrations(context.Background(), db)
}

type schemaMigration struct {
	version int
	name    string
	apply   func(context.Context, *sql.Tx) error
}

var schemaMigrations = []schemaMigration{
	{
		version: 1,
		name:    "category_tvdb_id",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			for _, table := range []string{"movies", "tv_episodes", "anime_episodes"} {
				if err := addColumnIfMissingTx(ctx, tx, table, "tvdb_id", "TEXT"); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		version: 2,
		name:    "category_imdb_fields",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			for _, table := range []string{"movies", "tv_episodes", "anime_episodes"} {
				if err := addColumnIfMissingTx(ctx, tx, table, "imdb_id", "TEXT"); err != nil {
					return err
				}
				if err := addColumnIfMissingTx(ctx, tx, table, "imdb_rating", "REAL DEFAULT 0"); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		version: 3,
		name:    "match_status",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			for _, table := range []string{"movies", "tv_episodes", "anime_episodes", "music_tracks"} {
				if err := addColumnIfMissingTx(ctx, tx, table, "match_status", "TEXT NOT NULL DEFAULT 'local'"); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		version: 4,
		name:    "episode_numbers",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			for _, table := range []string{"tv_episodes", "anime_episodes"} {
				if err := addColumnIfMissingTx(ctx, tx, table, "season", "INTEGER"); err != nil {
					return err
				}
				if err := addColumnIfMissingTx(ctx, tx, table, "episode", "INTEGER"); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		version: 5,
		name:    "thumbnail_path",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			for _, table := range []string{"tv_episodes", "anime_episodes"} {
				if err := addColumnIfMissingTx(ctx, tx, table, "thumbnail_path", "TEXT"); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		version: 6,
		name:    "library_playback_preferences",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			for _, column := range []struct {
				name string
				def  string
			}{
				{name: "preferred_audio_language", def: "TEXT"},
				{name: "preferred_subtitle_language", def: "TEXT"},
				{name: "subtitles_enabled_by_default", def: "INTEGER"},
			} {
				if err := addColumnIfMissingTx(ctx, tx, "libraries", column.name, column.def); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		version: 7,
		name:    "music_metadata_columns",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			for _, column := range []struct {
				name string
				def  string
			}{
				{name: "artist", def: "TEXT"},
				{name: "album", def: "TEXT"},
				{name: "album_artist", def: "TEXT"},
				{name: "disc_number", def: "INTEGER NOT NULL DEFAULT 0"},
				{name: "track_number", def: "INTEGER NOT NULL DEFAULT 0"},
				{name: "release_year", def: "INTEGER NOT NULL DEFAULT 0"},
			} {
				if err := addColumnIfMissingTx(ctx, tx, "music_tracks", column.name, column.def); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		version: 8,
		name:    "library_job_status_columns",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			for _, column := range []struct {
				name string
				def  string
			}{
				{name: "enriching", def: "INTEGER NOT NULL DEFAULT 0"},
				{name: "enrichment_phase", def: "TEXT NOT NULL DEFAULT 'idle'"},
				{name: "identify_phase", def: "TEXT NOT NULL DEFAULT 'idle'"},
				{name: "identified", def: "INTEGER NOT NULL DEFAULT 0"},
				{name: "identify_failed", def: "INTEGER NOT NULL DEFAULT 0"},
				{name: "processed", def: "INTEGER NOT NULL DEFAULT 0"},
				{name: "added", def: "INTEGER NOT NULL DEFAULT 0"},
				{name: "updated", def: "INTEGER NOT NULL DEFAULT 0"},
				{name: "removed", def: "INTEGER NOT NULL DEFAULT 0"},
				{name: "unmatched", def: "INTEGER NOT NULL DEFAULT 0"},
				{name: "skipped", def: "INTEGER NOT NULL DEFAULT 0"},
				{name: "identify_requested", def: "INTEGER NOT NULL DEFAULT 0"},
				{name: "error", def: "TEXT"},
				{name: "started_at", def: "DATETIME"},
				{name: "finished_at", def: "DATETIME"},
				{name: "updated_at", def: "DATETIME"},
			} {
				if err := addColumnIfMissingTx(ctx, tx, "library_job_status", column.name, column.def); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		version: 9,
		name:    "episode_metadata_review_needed",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			for _, table := range []string{"tv_episodes", "anime_episodes"} {
				if err := addColumnIfMissingTx(ctx, tx, table, "metadata_review_needed", "INTEGER NOT NULL DEFAULT 0"); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		version: 10,
		name:    "scan_queue_indexes",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			for _, column := range []struct {
				name string
				def  string
			}{
				{name: "queued_at", def: "DATETIME"},
				{name: "estimated_items", def: "INTEGER NOT NULL DEFAULT 0"},
			} {
				if err := addColumnIfMissingTx(ctx, tx, "library_job_status", column.name, column.def); err != nil {
					return err
				}
			}
			for _, stmt := range []string{
				`CREATE UNIQUE INDEX IF NOT EXISTS idx_subtitles_path ON subtitles(path)`,
				`CREATE INDEX IF NOT EXISTS idx_library_job_status_phase_updated_at ON library_job_status(phase, updated_at DESC)`,
				`CREATE INDEX IF NOT EXISTS idx_movies_library_match_status ON movies(library_id, match_status)`,
				`CREATE INDEX IF NOT EXISTS idx_tv_episodes_library_match_status ON tv_episodes(library_id, match_status)`,
				`CREATE INDEX IF NOT EXISTS idx_anime_episodes_library_match_status ON anime_episodes(library_id, match_status)`,
			} {
				if _, err := tx.ExecContext(ctx, stmt); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		version: 11,
		name:    "episode_metadata_confirmed",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			for _, table := range []string{"tv_episodes", "anime_episodes"} {
				if err := addColumnIfMissingTx(ctx, tx, table, "metadata_confirmed", "INTEGER NOT NULL DEFAULT 0"); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		version: 12,
		name:    "music_provider_metadata",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			for _, column := range []struct {
				name string
				def  string
			}{
				{name: "poster_path", def: "TEXT"},
				{name: "musicbrainz_artist_id", def: "TEXT"},
				{name: "musicbrainz_release_group_id", def: "TEXT"},
				{name: "musicbrainz_release_id", def: "TEXT"},
				{name: "musicbrainz_recording_id", def: "TEXT"},
			} {
				if err := addColumnIfMissingTx(ctx, tx, "music_tracks", column.name, column.def); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		version: 13,
		name:    "media_file_state",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			tables := []string{"movies", "tv_episodes", "anime_episodes", "music_tracks"}
			for _, table := range tables {
				for _, column := range []struct {
					name string
					def  string
				}{
					{name: "file_size_bytes", def: "INTEGER NOT NULL DEFAULT 0"},
					{name: "file_mod_time", def: "TEXT"},
					{name: "file_hash", def: "TEXT"},
					{name: "file_hash_kind", def: "TEXT"},
					{name: "last_seen_at", def: "TEXT"},
					{name: "missing_since", def: "TEXT"},
				} {
					if err := addColumnIfMissingTx(ctx, tx, table, column.name, column.def); err != nil {
						return err
					}
				}
			}
			for _, stmt := range []string{
				`CREATE INDEX IF NOT EXISTS idx_movies_library_missing_since ON movies(library_id, missing_since)`,
				`CREATE INDEX IF NOT EXISTS idx_tv_episodes_library_missing_since ON tv_episodes(library_id, missing_since)`,
				`CREATE INDEX IF NOT EXISTS idx_anime_episodes_library_missing_since ON anime_episodes(library_id, missing_since)`,
				`CREATE INDEX IF NOT EXISTS idx_music_tracks_library_missing_since ON music_tracks(library_id, missing_since)`,
				`CREATE INDEX IF NOT EXISTS idx_movies_library_file_hash ON movies(library_id, file_hash) WHERE file_hash IS NOT NULL AND file_hash != ''`,
				`CREATE INDEX IF NOT EXISTS idx_tv_episodes_library_file_hash ON tv_episodes(library_id, file_hash) WHERE file_hash IS NOT NULL AND file_hash != ''`,
				`CREATE INDEX IF NOT EXISTS idx_anime_episodes_library_file_hash ON anime_episodes(library_id, file_hash) WHERE file_hash IS NOT NULL AND file_hash != ''`,
				`CREATE INDEX IF NOT EXISTS idx_music_tracks_library_file_hash ON music_tracks(library_id, file_hash) WHERE file_hash IS NOT NULL AND file_hash != ''`,
			} {
				if _, err := tx.ExecContext(ctx, stmt); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		version: 14,
		name:    "metadata_storage_entities_and_cache",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			for _, stmt := range []string{
				`CREATE TABLE IF NOT EXISTS shows (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  library_id INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
  kind TEXT NOT NULL CHECK (kind IN ('tv','anime')),
  tmdb_id INTEGER,
  tvdb_id TEXT,
  title TEXT NOT NULL,
  title_key TEXT NOT NULL,
  overview TEXT,
  poster_path TEXT,
  backdrop_path TEXT,
  first_air_date TEXT,
  vote_average REAL DEFAULT 0,
  imdb_id TEXT,
  imdb_rating REAL DEFAULT 0,
  metadata_version INTEGER NOT NULL DEFAULT 1,
  metadata_hash TEXT,
  last_refreshed_at TEXT,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
)`,
				`CREATE TABLE IF NOT EXISTS seasons (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  show_id INTEGER NOT NULL REFERENCES shows(id) ON DELETE CASCADE,
  season_number INTEGER NOT NULL,
  title TEXT,
  overview TEXT,
  poster_path TEXT,
  air_date TEXT,
  metadata_version INTEGER NOT NULL DEFAULT 1,
  metadata_hash TEXT,
  last_refreshed_at TEXT,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
)`,
				`CREATE TABLE IF NOT EXISTS metadata_provider_cache (
  provider TEXT NOT NULL,
  method TEXT NOT NULL,
  url_path TEXT NOT NULL,
  query_hash TEXT NOT NULL,
  body_hash TEXT NOT NULL,
  response_json BLOB NOT NULL,
  fetched_at TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  schema_version INTEGER NOT NULL DEFAULT 1,
  content_hash TEXT NOT NULL,
  status_code INTEGER NOT NULL,
  last_accessed_at TEXT NOT NULL,
  hit_count INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (provider, method, url_path, query_hash, body_hash)
)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS idx_shows_library_kind_tmdb_id ON shows(library_id, kind, tmdb_id) WHERE tmdb_id IS NOT NULL AND tmdb_id > 0`,
				`CREATE UNIQUE INDEX IF NOT EXISTS idx_shows_library_kind_title_key ON shows(library_id, kind, title_key) WHERE tmdb_id IS NULL OR tmdb_id <= 0`,
				`CREATE INDEX IF NOT EXISTS idx_shows_library_kind ON shows(library_id, kind)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS idx_seasons_show_number ON seasons(show_id, season_number)`,
				`CREATE INDEX IF NOT EXISTS idx_seasons_show_id ON seasons(show_id)`,
				`CREATE INDEX IF NOT EXISTS idx_metadata_provider_cache_expires_at ON metadata_provider_cache(expires_at)`,
			} {
				if _, err := tx.ExecContext(ctx, stmt); err != nil {
					return err
				}
			}
			for _, table := range []string{"tv_episodes", "anime_episodes"} {
				for _, column := range []struct {
					name string
					def  string
				}{
					{name: "show_id", def: "INTEGER REFERENCES shows(id) ON DELETE SET NULL"},
					{name: "season_id", def: "INTEGER REFERENCES seasons(id) ON DELETE SET NULL"},
					{name: "metadata_version", def: "INTEGER NOT NULL DEFAULT 1"},
					{name: "metadata_content_hash", def: "TEXT"},
					{name: "last_metadata_refresh_at", def: "TEXT"},
				} {
					if err := addColumnIfMissingTx(ctx, tx, table, column.name, column.def); err != nil {
						return err
					}
				}
			}
			for _, stmt := range []string{
				`CREATE INDEX IF NOT EXISTS idx_tv_episodes_show_id ON tv_episodes(show_id)`,
				`CREATE INDEX IF NOT EXISTS idx_tv_episodes_season_id ON tv_episodes(season_id)`,
				`CREATE INDEX IF NOT EXISTS idx_anime_episodes_show_id ON anime_episodes(show_id)`,
				`CREATE INDEX IF NOT EXISTS idx_anime_episodes_season_id ON anime_episodes(season_id)`,
			} {
				if _, err := tx.ExecContext(ctx, stmt); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		version: 15,
		name:    "metadata_storage_backfill",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			return backfillShowsAndSeasonsTx(ctx, tx)
		},
	},
	{
		version: 16,
		name:    "show_title_key_scoped_to_unmatched_rows",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, `DROP INDEX IF EXISTS idx_shows_library_kind_title_key`); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS idx_shows_library_kind_title_key ON shows(library_id, kind, title_key) WHERE tmdb_id IS NULL OR tmdb_id <= 0`)
			return err
		},
	},
	{
		version: 17,
		name:    "library_automation_and_job_retry_fields",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			for _, column := range []struct {
				name string
				def  string
			}{
				{name: "watcher_enabled", def: "INTEGER NOT NULL DEFAULT 0"},
				{name: "watcher_mode", def: "TEXT NOT NULL DEFAULT 'auto'"},
				{name: "scan_interval_minutes", def: "INTEGER NOT NULL DEFAULT 0"},
			} {
				if err := addColumnIfMissingTx(ctx, tx, "libraries", column.name, column.def); err != nil {
					return err
				}
			}
			for _, column := range []struct {
				name string
				def  string
			}{
				{name: "retry_count", def: "INTEGER NOT NULL DEFAULT 0"},
				{name: "max_retries", def: "INTEGER NOT NULL DEFAULT 3"},
				{name: "next_retry_at", def: "DATETIME"},
				{name: "last_error", def: "TEXT"},
				{name: "next_scheduled_at", def: "DATETIME"},
			} {
				if err := addColumnIfMissingTx(ctx, tx, "library_job_status", column.name, column.def); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		version: 18,
		name:    "media_files_and_artwork_tables",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			for _, stmt := range []string{
				`CREATE TABLE IF NOT EXISTS media_files (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  media_id INTEGER NOT NULL REFERENCES media_global(id) ON DELETE CASCADE,
  path TEXT NOT NULL UNIQUE,
  file_size_bytes INTEGER NOT NULL DEFAULT 0,
  file_mod_time TEXT,
  file_hash TEXT,
  file_hash_kind TEXT,
  duration INTEGER NOT NULL DEFAULT 0,
  missing_since TEXT,
  last_seen_at TEXT,
  is_primary INTEGER NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
)`,
				`CREATE INDEX IF NOT EXISTS idx_media_files_media_id ON media_files(media_id)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS idx_media_files_primary_media_id ON media_files(media_id) WHERE is_primary = 1`,
				`CREATE INDEX IF NOT EXISTS idx_media_files_hash ON media_files(file_hash) WHERE file_hash IS NOT NULL AND file_hash != ''`,
				`CREATE TABLE IF NOT EXISTS artwork_assets (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  source_url TEXT NOT NULL,
  artwork_kind TEXT NOT NULL CHECK (artwork_kind IN ('poster','backdrop')),
  source_etag TEXT,
  content_hash TEXT,
  mime_type TEXT,
  width INTEGER NOT NULL DEFAULT 0,
  height INTEGER NOT NULL DEFAULT 0,
  original_rel_path TEXT NOT NULL,
  last_fetched_at TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS idx_artwork_assets_source ON artwork_assets(source_url, artwork_kind)`,
				`CREATE INDEX IF NOT EXISTS idx_artwork_assets_hash ON artwork_assets(content_hash) WHERE content_hash IS NOT NULL AND content_hash != ''`,
				`CREATE TABLE IF NOT EXISTS artwork_variants (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  asset_id INTEGER NOT NULL REFERENCES artwork_assets(id) ON DELETE CASCADE,
  profile TEXT NOT NULL,
  rel_path TEXT NOT NULL,
  width INTEGER NOT NULL DEFAULT 0,
  height INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS idx_artwork_variants_asset_profile ON artwork_variants(asset_id, profile)`,
				`CREATE TABLE IF NOT EXISTS artwork_links (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  entity_kind TEXT NOT NULL,
  entity_id INTEGER NOT NULL,
  artwork_kind TEXT NOT NULL CHECK (artwork_kind IN ('poster','backdrop')),
  asset_id INTEGER NOT NULL REFERENCES artwork_assets(id) ON DELETE CASCADE,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS idx_artwork_links_entity_kind ON artwork_links(entity_kind, entity_id, artwork_kind)`,
			} {
				if _, err := tx.ExecContext(ctx, stmt); err != nil {
					return err
				}
			}
			return backfillMediaFilesTx(ctx, tx)
		},
	},
	{
		version: 19,
		name:    "title_metadata_and_search_index",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			for _, stmt := range []string{
				`CREATE TABLE IF NOT EXISTS metadata_genres (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  slug TEXT NOT NULL UNIQUE,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
)`,
				`CREATE TABLE IF NOT EXISTS metadata_people (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  name_key TEXT NOT NULL UNIQUE,
  provider TEXT NOT NULL DEFAULT '',
  provider_id TEXT NOT NULL DEFAULT '',
  profile_path TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
)`,
				`CREATE TABLE IF NOT EXISTS title_genres (
  title_kind TEXT NOT NULL CHECK (title_kind IN ('movie','show')),
  title_id INTEGER NOT NULL,
  genre_slug TEXT NOT NULL REFERENCES metadata_genres(slug) ON DELETE CASCADE,
  PRIMARY KEY (title_kind, title_id, genre_slug)
)`,
				`CREATE TABLE IF NOT EXISTS title_cast (
  title_kind TEXT NOT NULL CHECK (title_kind IN ('movie','show')),
  title_id INTEGER NOT NULL,
  person_name_key TEXT NOT NULL REFERENCES metadata_people(name_key) ON DELETE CASCADE,
  character_name TEXT,
  billing_order INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (title_kind, title_id, person_name_key, character_name)
)`,
				`CREATE INDEX IF NOT EXISTS idx_title_genres_kind_id ON title_genres(title_kind, title_id)`,
				`CREATE INDEX IF NOT EXISTS idx_title_cast_kind_id ON title_cast(title_kind, title_id)`,
				`CREATE INDEX IF NOT EXISTS idx_title_cast_person ON title_cast(person_name_key)`,
				`CREATE TABLE IF NOT EXISTS search_documents (
  doc_key TEXT PRIMARY KEY,
  kind TEXT NOT NULL CHECK (kind IN ('movie','show')),
  library_id INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
  library_name TEXT NOT NULL,
  library_type TEXT NOT NULL,
  title TEXT NOT NULL,
  normalized_title TEXT NOT NULL,
  subtitle TEXT NOT NULL DEFAULT '',
  poster_path TEXT NOT NULL DEFAULT '',
  poster_url TEXT NOT NULL DEFAULT '',
  imdb_rating REAL NOT NULL DEFAULT 0,
  href TEXT NOT NULL,
  show_key TEXT NOT NULL DEFAULT '',
  media_id INTEGER NOT NULL DEFAULT 0,
  title_ref_id INTEGER NOT NULL DEFAULT 0,
  updated_at DATETIME NOT NULL
)`,
				`CREATE INDEX IF NOT EXISTS idx_search_documents_library ON search_documents(library_id, kind)`,
				`CREATE TABLE IF NOT EXISTS search_document_genres (
  doc_key TEXT NOT NULL REFERENCES search_documents(doc_key) ON DELETE CASCADE,
  genre_slug TEXT NOT NULL,
  genre_name TEXT NOT NULL,
  PRIMARY KEY (doc_key, genre_slug)
)`,
				`CREATE INDEX IF NOT EXISTS idx_search_document_genres_slug ON search_document_genres(genre_slug)`,
				`CREATE TABLE IF NOT EXISTS search_document_cast (
  doc_key TEXT NOT NULL REFERENCES search_documents(doc_key) ON DELETE CASCADE,
  person_name TEXT NOT NULL,
  person_name_key TEXT NOT NULL,
  billing_order INTEGER NOT NULL DEFAULT 0,
  character_name TEXT,
  PRIMARY KEY (doc_key, person_name_key, character_name)
)`,
				`CREATE INDEX IF NOT EXISTS idx_search_document_cast_person ON search_document_cast(person_name_key)`,
				`CREATE VIRTUAL TABLE IF NOT EXISTS search_documents_fts USING fts5(
  doc_key UNINDEXED,
  title,
  normalized_title
)`,
				`CREATE VIRTUAL TABLE IF NOT EXISTS search_people_fts USING fts5(
  doc_key UNINDEXED,
  person_name,
  person_name_key
)`,
			} {
				if _, err := tx.ExecContext(ctx, stmt); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		version: 20,
		name:    "metadata_artwork_settings_and_poster_locks",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			if err := addColumnIfMissingTx(ctx, tx, "movies", "poster_locked", "INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}
			if err := addColumnIfMissingTx(ctx, tx, "shows", "poster_locked", "INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}
			return nil
		},
	},
	{
		version: 21,
		name:    "library_job_enrichment_phase",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			if err := addColumnIfMissingTx(ctx, tx, "library_job_status", "enrichment_phase", "TEXT NOT NULL DEFAULT 'idle'"); err != nil {
				return err
			}
			if _, err := tx.ExecContext(
				ctx,
				`UPDATE library_job_status
				SET enrichment_phase = CASE
					WHEN COALESCE(enriching, 0) != 0 THEN 'running'
					ELSE 'idle'
				END`,
			); err != nil {
				return err
			}
			return nil
		},
	},
	{
		version: 22,
		name:    "enable_watcher_for_existing_libraries",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `UPDATE libraries SET watcher_enabled = 1 WHERE watcher_enabled = 0`)
			return err
		},
	},
	{
		version: 23,
		name:    "show_vote_average",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			// Series vote_average must come from provider show metadata, not MAX(episode),
			// or the UI would show misleading scores until a full show refresh.
			return addColumnIfMissingTx(ctx, tx, "shows", "vote_average", "REAL DEFAULT 0")
		},
	},
	{
		version: 24,
		name:    "intro_skip_and_chapter_probe",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			if err := addColumnIfMissingTx(ctx, tx, "media_files", "intro_start_sec", "REAL"); err != nil {
				return err
			}
			if err := addColumnIfMissingTx(ctx, tx, "media_files", "intro_end_sec", "REAL"); err != nil {
				return err
			}
			if err := addColumnIfMissingTx(ctx, tx, "libraries", "intro_skip_mode", "TEXT NOT NULL DEFAULT 'manual'"); err != nil {
				return err
			}
			return nil
		},
	},
	{
		version: 25,
		name:    "embedded_subtitle_codec_and_supported",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			if err := addColumnIfMissingTx(ctx, tx, "embedded_subtitles", "codec", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			return addColumnIfMissingTx(ctx, tx, "embedded_subtitles", "supported", "INTEGER")
		},
	},
	{
		version: 26,
		name:    "quick_connect_codes",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS quick_connect_codes (
  code TEXT NOT NULL PRIMARY KEY,
  user_id INTEGER NOT NULL,
  expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_quick_connect_codes_user_id ON quick_connect_codes(user_id);
CREATE INDEX IF NOT EXISTS idx_quick_connect_codes_expires_at ON quick_connect_codes(expires_at);
`)
			return err
		},
	},
	{
		version: 27,
		name:    "sessions_expires_at_index",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
`)
			return err
		},
	},
	{
		version: 28,
		name:    "users_single_admin_unique",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_single_admin ON users(is_admin) WHERE is_admin = 1;
`)
			return err
		},
	},
	{
		version: 29,
		name:    "quick_connect_codes_unix_timestamps",
		apply:   migrateQuickConnectCodesToUnixTx,
	},
	{
		version: 30,
		name:    "embedded_subtitles_composite_index",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, `DROP INDEX IF EXISTS idx_embedded_subtitles_media_id`); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_embedded_subtitles_media_stream ON embedded_subtitles(media_id, stream_index)`)
			return err
		},
	},
}

func migrateQuickConnectCodesToUnixTx(ctx context.Context, tx *sql.Tx) error {
	var tableName string
	err := tx.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name='quick_connect_codes'`).Scan(&tableName)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `CREATE TABLE quick_connect_codes_unix (
  code TEXT NOT NULL PRIMARY KEY,
  user_id INTEGER NOT NULL,
  expires_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
)`); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT code, user_id, expires_at, created_at FROM quick_connect_codes`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var code, expStr, creStr string
		var userID int
		if err := rows.Scan(&code, &userID, &expStr, &creStr); err != nil {
			return err
		}
		expT, e1 := time.Parse(time.RFC3339, expStr)
		creT, e2 := time.Parse(time.RFC3339, creStr)
		if e1 != nil || e2 != nil {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO quick_connect_codes_unix (code, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`,
			code, userID, expT.Unix(), creT.Unix(),
		); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE quick_connect_codes`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `ALTER TABLE quick_connect_codes_unix RENAME TO quick_connect_codes`); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
CREATE INDEX IF NOT EXISTS idx_quick_connect_codes_user_id ON quick_connect_codes(user_id);
CREATE INDEX IF NOT EXISTS idx_quick_connect_codes_expires_at ON quick_connect_codes(expires_at);
`)
	return err
}

func applySchemaMigrations(ctx context.Context, db *sql.DB) error {
	applied, err := listAppliedSchemaMigrations(db)
	if err != nil {
		return err
	}

	for _, migration := range schemaMigrations {
		if _, ok := applied[migration.version]; ok {
			continue
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if err := migration.apply(ctx, tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply schema migration %d (%s): %w", migration.version, migration.name, err)
		}
		if err := recordSchemaMigrationTx(ctx, tx, migration); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

func backfillMediaFilesTx(ctx context.Context, tx *sql.Tx) error {
	now := time.Now().UTC().Format(time.RFC3339)
	for _, table := range []string{"movies", "tv_episodes", "anime_episodes", "music_tracks"} {
		kind := tableToKind(table)
		if kind == "" {
			continue
		}
		stmt := `INSERT OR IGNORE INTO media_files (
media_id, path, file_size_bytes, file_mod_time, file_hash, file_hash_kind, duration, missing_since, last_seen_at, is_primary, created_at, updated_at
)
SELECT
  g.id,
  m.path,
  COALESCE(m.file_size_bytes, 0),
  COALESCE(m.file_mod_time, ''),
  COALESCE(m.file_hash, ''),
  COALESCE(m.file_hash_kind, ''),
  COALESCE(m.duration, 0),
  COALESCE(m.missing_since, ''),
  COALESCE(m.last_seen_at, ''),
  1,
  ?,
  ?
FROM ` + table + ` m
JOIN media_global g ON g.kind = ? AND g.ref_id = m.id
WHERE COALESCE(m.path, '') != ''`
		if _, err := tx.ExecContext(ctx, stmt, now, now, kind); err != nil {
			return err
		}
	}
	return nil
}

func tableToKind(table string) string {
	switch table {
	case "movies":
		return LibraryTypeMovie
	case "tv_episodes":
		return LibraryTypeTV
	case "anime_episodes":
		return LibraryTypeAnime
	case "music_tracks":
		return LibraryTypeMusic
	default:
		return ""
	}
}

func listAppliedSchemaMigrations(db *sql.DB) (map[int]struct{}, error) {
	rows, err := db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[int]struct{})
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = struct{}{}
	}
	return applied, rows.Err()
}

func recordSchemaMigrationTx(ctx context.Context, tx *sql.Tx, migration schemaMigration) error {
	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
		migration.version,
		migration.name,
		time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

func addColumnIfMissingTx(ctx context.Context, tx *sql.Tx, table, column, definition string) error {
	exists, err := columnExistsTx(ctx, tx, table, column)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = tx.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition))
	return err
}

func columnExistsTx(ctx context.Context, tx *sql.Tx, table, column string) (bool, error) {
	rows, err := tx.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal sql.NullString
			pk         int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

func GetAllMediaForUser(db *sql.DB, userID int) ([]MediaItem, error) {
	items, err := queryAllMediaByKind(db, userID, "")
	if err != nil {
		return nil, err
	}
	items, err = attachMediaFilesBatch(db, items)
	if err != nil {
		return nil, err
	}
	return attachSubtitlesBatch(db, items)
}

func UserHasLibraryAccess(db *sql.DB, userID, libraryID int) (bool, error) {
	var ownerID int
	err := db.QueryRow(`SELECT user_id FROM libraries WHERE id = ?`, libraryID).Scan(&ownerID)
	if err != nil {
		return false, err
	}
	return ownerID == userID, nil
}

// queryAllMediaByKind returns media from category tables joined with media_global.
// If kind is "", queries all four categories and merges; otherwise only that kind.
// If userID > 0, filters media to only those in libraries owned by that user.
func queryAllMediaByKind(db *sql.DB, userID int, kind string) ([]MediaItem, error) {
	kinds := []string{"movie", "tv", "anime", "music"}
	if kind != "" {
		kinds = []string{kind}
	}
	var items []MediaItem
	for _, k := range kinds {
		table := mediaTableForKind(k)
		var q string
		var args []interface{}
		args = append(args, k)

		switch table {
		case "music_tracks":
			q = `SELECT g.id, m.library_id, m.title, m.path, m.duration, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.missing_since, ''), m.match_status, m.artist, m.album, m.album_artist, m.poster_path, COALESCE(m.disc_number, 0), COALESCE(m.track_number, 0), COALESCE(m.release_year, 0) FROM music_tracks m JOIN media_global g ON g.kind = ? AND g.ref_id = m.id `
		case "tv_episodes", "anime_episodes":
			q = `SELECT g.id, m.library_id, m.title, m.path, m.duration, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.missing_since, ''), m.match_status, m.tmdb_id, m.tvdb_id, m.overview, m.poster_path, m.backdrop_path, m.release_date, m.vote_average, m.imdb_id, m.imdb_rating, COALESCE(m.season, 0), COALESCE(m.episode, 0), COALESCE(m.metadata_review_needed, 0), COALESCE(m.metadata_confirmed, 0), m.thumbnail_path FROM ` + table + ` m JOIN media_global g ON g.kind = ? AND g.ref_id = m.id `
		default:
			q = `SELECT g.id, m.library_id, m.title, m.path, m.duration, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.missing_since, ''), m.match_status, m.tmdb_id, m.tvdb_id, m.overview, m.poster_path, m.backdrop_path, m.release_date, m.vote_average, m.imdb_id, m.imdb_rating FROM ` + table + ` m JOIN media_global g ON g.kind = ? AND g.ref_id = m.id `
		}

		if userID > 0 {
			q += ` JOIN libraries l ON l.id = m.library_id AND l.user_id = ? `
			args = append(args, userID)
		}

		q += ` ORDER BY g.id`

		rows, err := db.Query(q, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var m MediaItem
			m.Type = k
			var overview, posterPath, backdropPath, releaseDate, thumbnailPath sql.NullString
			var matchStatus, imdbID sql.NullString
			var voteAvg, imdbRating sql.NullFloat64
			var tmdbID sql.NullInt64
			var tvdbID sql.NullString
			var metadataReviewNeeded sql.NullBool
			var metadataConfirmed sql.NullBool
			var artist, album, albumArtist sql.NullString
			var musicPosterPath sql.NullString
			switch table {
			case "music_tracks":
				err = rows.Scan(&m.ID, &m.LibraryID, &m.Title, &m.Path, &m.Duration, &m.FileSizeBytes, &m.FileModTime, &m.FileHash, &m.FileHashKind, &m.MissingSince, &matchStatus, &artist, &album, &albumArtist, &musicPosterPath, &m.DiscNumber, &m.TrackNumber, &m.ReleaseYear)
				if artist.Valid {
					m.Artist = artist.String
				}
				if album.Valid {
					m.Album = album.String
				}
				if albumArtist.Valid {
					m.AlbumArtist = albumArtist.String
				}
				if musicPosterPath.Valid {
					m.PosterPath = musicPosterPath.String
				}
			case "tv_episodes", "anime_episodes":
				err = rows.Scan(&m.ID, &m.LibraryID, &m.Title, &m.Path, &m.Duration, &m.FileSizeBytes, &m.FileModTime, &m.FileHash, &m.FileHashKind, &m.MissingSince, &matchStatus, &tmdbID, &tvdbID, &overview, &posterPath, &backdropPath, &releaseDate, &voteAvg, &imdbID, &imdbRating, &m.Season, &m.Episode, &metadataReviewNeeded, &metadataConfirmed, &thumbnailPath)
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
			default:
				err = rows.Scan(&m.ID, &m.LibraryID, &m.Title, &m.Path, &m.Duration, &m.FileSizeBytes, &m.FileModTime, &m.FileHash, &m.FileHashKind, &m.MissingSince, &matchStatus, &tmdbID, &tvdbID, &overview, &posterPath, &backdropPath, &releaseDate, &voteAvg, &imdbID, &imdbRating)
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
			}
			if matchStatus.Valid {
				m.MatchStatus = matchStatus.String
			}
			m.Missing = m.MissingSince != ""
			if err != nil {
				rows.Close()
				return nil, err
			}
			items = append(items, m)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return attachDuplicateState(db, items)
}

func mediaTableForKind(kind string) string {
	return MediaTableForKind(kind)
}

// MediaTableForKind returns the category table name for a library kind (exported for use by http handlers).
func MediaTableForKind(kind string) string {
	switch kind {
	case "movie":
		return "movies"
	case "tv":
		return "tv_episodes"
	case "anime":
		return "anime_episodes"
	case "music":
		return "music_tracks"
	default:
		return "movies"
	}
}

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
		`SELECT id, name, type FROM libraries WHERE user_id = ? AND type != ? ORDER BY id`,
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

// UpdateMediaMetadata updates a single category row with identified metadata (title, overview, poster, tmdb_id, etc.).
func UpdateMediaMetadata(db *sql.DB, table string, refID int, title string, overview, posterPath, backdropPath, releaseDate string, voteAvg float64, imdbID string, imdbRating float64, tmdbID int, tvdbID string, season, episode int) error {
	return UpdateMediaMetadataWithState(db, table, refID, title, overview, posterPath, backdropPath, releaseDate, voteAvg, imdbID, imdbRating, tmdbID, tvdbID, season, episode, false, false)
}

// UpdateMediaMetadataWithReview updates a single category row with identified metadata and review state.
func UpdateMediaMetadataWithReview(db *sql.DB, table string, refID int, title string, overview, posterPath, backdropPath, releaseDate string, voteAvg float64, imdbID string, imdbRating float64, tmdbID int, tvdbID string, season, episode int, metadataReviewNeeded bool) error {
	return UpdateMediaMetadataWithState(db, table, refID, title, overview, posterPath, backdropPath, releaseDate, voteAvg, imdbID, imdbRating, tmdbID, tvdbID, season, episode, metadataReviewNeeded, false)
}

// UpdateMediaMetadataWithState updates a single category row with identified metadata and episodic metadata state.
func UpdateMediaMetadataWithState(db *sql.DB, table string, refID int, title string, overview, posterPath, backdropPath, releaseDate string, voteAvg float64, imdbID string, imdbRating float64, tmdbID int, tvdbID string, season, episode int, metadataReviewNeeded bool, metadataConfirmed bool) error {
	return UpdateMediaMetadataWithCanonicalState(db, table, refID, title, overview, posterPath, backdropPath, releaseDate, voteAvg, imdbID, imdbRating, tmdbID, tvdbID, season, episode, CanonicalMetadata{
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
func UpdateMediaMetadataWithCanonicalState(db *sql.DB, table string, refID int, title string, overview, posterPath, backdropPath, releaseDate string, voteAvg float64, imdbID string, imdbRating float64, tmdbID int, tvdbID string, season, episode int, canonical CanonicalMetadata, metadataReviewNeeded bool, metadataConfirmed bool, updateShowVoteAverage bool) error {
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
		ctx := context.Background()
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
	ctx := context.Background()
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

// attachSubtitlesBatch loads subtitle and embedded stream metadata for all items in batch queries.
func attachSubtitlesBatch(db *sql.DB, items []MediaItem) ([]MediaItem, error) {
	if len(items) == 0 {
		return items, nil
	}
	ids := make([]int, len(items))
	for i := range items {
		ids[i] = items[i].ID
	}
	subsByID, err := getSubtitlesByMediaIDs(db, ids)
	if err != nil {
		return nil, err
	}
	embByID, err := getEmbeddedSubtitlesByMediaIDs(db, ids)
	if err != nil {
		return nil, err
	}
	audioByID, err := getEmbeddedAudioTracksByMediaIDs(db, ids)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if subs := subsByID[items[i].ID]; subs != nil {
			items[i].Subtitles = subs
		} else {
			items[i].Subtitles = []Subtitle{}
		}
		if embedded := embByID[items[i].ID]; embedded != nil {
			items[i].EmbeddedSubtitles = embedded
		} else {
			items[i].EmbeddedSubtitles = []EmbeddedSubtitle{}
		}
		if audioTracks := audioByID[items[i].ID]; audioTracks != nil {
			items[i].EmbeddedAudioTracks = audioTracks
		} else {
			items[i].EmbeddedAudioTracks = []EmbeddedAudioTrack{}
		}
	}
	return items, nil
}

func EnsurePlaybackTrackMetadata(ctx context.Context, db *sql.DB, item *MediaItem) error {
	_, err := RefreshPlaybackTrackMetadata(ctx, db, item)
	return err
}

func RefreshPlaybackTrackMetadata(ctx context.Context, db *sql.DB, item *MediaItem) (PlaybackTrackMetadata, error) {
	metadata := PlaybackTrackMetadata{
		Subtitles:           []Subtitle{},
		EmbeddedSubtitles:   []EmbeddedSubtitle{},
		EmbeddedAudioTracks: []EmbeddedAudioTrack{},
	}
	if item == nil || item.ID <= 0 {
		return metadata, nil
	}

	sourcePath, err := ResolveMediaSourcePath(db, *item)
	if err != nil {
		return metadata, err
	}
	item.Path = sourcePath

	if item.Type == LibraryTypeMusic {
		return metadata, nil
	}

	if err := scanForSubtitles(ctx, db, item.ID, sourcePath); err != nil {
		slog.Warn("refresh playback sidecar subtitles", "media_id", item.ID, "path", sourcePath, "error", err)
	}

	subtitles, err := getSubtitlesForMedia(db, item.ID)
	if err != nil {
		return metadata, err
	}
	if subtitles != nil {
		metadata.Subtitles = subtitles
	}

	probed, err := readVideoMetadata(ctx, sourcePath)
	if err != nil {
		slog.Warn("refresh playback embedded tracks", "media_id", item.ID, "path", sourcePath, "error", err)
		embeddedSubtitles, embeddedAudioTracks, getErr := getPersistedPlaybackTracks(db, item.ID)
		if getErr != nil {
			return metadata, getErr
		}
		metadata.EmbeddedSubtitles = embeddedSubtitles
		metadata.EmbeddedAudioTracks = embeddedAudioTracks
	} else {
		persistEmbeddedStreams(ctx, db, item.ID, probed.EmbeddedSubtitles, probed.EmbeddedAudioTracks)
		metadata.EmbeddedSubtitles = append(metadata.EmbeddedSubtitles, probed.EmbeddedSubtitles...)
		metadata.EmbeddedAudioTracks = append(metadata.EmbeddedAudioTracks, probed.EmbeddedAudioTracks...)
		if err := UpdateMediaFileIntroFromProbe(ctx, db, item.ID, sourcePath, probed); err != nil {
			slog.Warn("persist intro chapters", "media_id", item.ID, "path", sourcePath, "error", err)
		}
		if probed.IntroStartSeconds != nil {
			v := *probed.IntroStartSeconds
			item.IntroStartSeconds = &v
		} else {
			item.IntroStartSeconds = nil
		}
		if probed.IntroEndSeconds != nil {
			v := *probed.IntroEndSeconds
			item.IntroEndSeconds = &v
		} else {
			item.IntroEndSeconds = nil
		}
	}

	item.Subtitles = append([]Subtitle{}, metadata.Subtitles...)
	item.EmbeddedSubtitles = append([]EmbeddedSubtitle{}, metadata.EmbeddedSubtitles...)
	item.EmbeddedAudioTracks = append([]EmbeddedAudioTrack{}, metadata.EmbeddedAudioTracks...)
	return metadata, nil
}

// RefreshPlaybackTrackMetadataForLibrary runs RefreshPlaybackTrackMetadata for every present
// (non-missing) item in the library. Music libraries are no-ops. Per-item errors are counted in
// failed and do not abort the rest.
func RefreshPlaybackTrackMetadataForLibrary(ctx context.Context, dbConn *sql.DB, libraryID int) (refreshed int, failed int, err error) {
	var typ string
	if err := dbConn.QueryRow(`SELECT type FROM libraries WHERE id = ?`, libraryID).Scan(&typ); err != nil {
		return 0, 0, err
	}
	if typ == LibraryTypeMusic {
		return 0, 0, nil
	}
	items, err := GetMediaByLibraryID(dbConn, libraryID)
	if err != nil {
		return 0, 0, err
	}
	for i := range items {
		it := items[i]
		_, rerr := RefreshPlaybackTrackMetadata(ctx, dbConn, &it)
		if rerr != nil {
			slog.Warn("refresh playback tracks", "library_id", libraryID, "media_id", it.ID, "error", rerr)
			failed++
		} else {
			refreshed++
		}
	}
	return refreshed, failed, nil
}

func getPersistedPlaybackTracks(db *sql.DB, mediaID int) ([]EmbeddedSubtitle, []EmbeddedAudioTrack, error) {
	embeddedSubtitles, err := getEmbeddedSubtitlesForMedia(db, mediaID)
	if err != nil {
		return nil, nil, err
	}
	embeddedAudioTracks, err := getEmbeddedAudioTracksForMedia(db, mediaID)
	if err != nil {
		return nil, nil, err
	}
	if embeddedSubtitles == nil {
		embeddedSubtitles = []EmbeddedSubtitle{}
	}
	if embeddedAudioTracks == nil {
		embeddedAudioTracks = []EmbeddedAudioTrack{}
	}
	return embeddedSubtitles, embeddedAudioTracks, nil
}

func getSubtitlesByMediaIDs(db *sql.DB, mediaIDs []int) (map[int][]Subtitle, error) {
	if len(mediaIDs) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(mediaIDs))
	args := make([]interface{}, len(mediaIDs))
	for i := range mediaIDs {
		placeholders[i] = "?"
		args[i] = mediaIDs[i]
	}
	query := `SELECT id, media_id, title, language, format, path FROM subtitles WHERE media_id IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int][]Subtitle)
	for rows.Next() {
		var s Subtitle
		if err := rows.Scan(&s.ID, &s.MediaID, &s.Title, &s.Language, &s.Format, &s.Path); err != nil {
			return nil, err
		}
		out[s.MediaID] = append(out[s.MediaID], s)
	}
	return out, rows.Err()
}

func getEmbeddedSubtitlesByMediaIDs(db *sql.DB, mediaIDs []int) (map[int][]EmbeddedSubtitle, error) {
	if len(mediaIDs) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(mediaIDs))
	args := make([]interface{}, len(mediaIDs))
	for i := range mediaIDs {
		placeholders[i] = "?"
		args[i] = mediaIDs[i]
	}
	query := `SELECT media_id, stream_index, language, title, COALESCE(codec, ''), supported FROM embedded_subtitles WHERE media_id IN (` + strings.Join(placeholders, ",") + `) ORDER BY media_id, stream_index`
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int][]EmbeddedSubtitle)
	for rows.Next() {
		var s EmbeddedSubtitle
		var supportedInt sql.NullInt64
		if err := rows.Scan(&s.MediaID, &s.StreamIndex, &s.Language, &s.Title, &s.Codec, &supportedInt); err != nil {
			return nil, err
		}
		if supportedInt.Valid {
			v := supportedInt.Int64 != 0
			s.Supported = &v
		}
		out[s.MediaID] = append(out[s.MediaID], s)
	}
	return out, rows.Err()
}

func getEmbeddedAudioTracksByMediaIDs(db *sql.DB, mediaIDs []int) (map[int][]EmbeddedAudioTrack, error) {
	if len(mediaIDs) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(mediaIDs))
	args := make([]interface{}, len(mediaIDs))
	for i := range mediaIDs {
		placeholders[i] = "?"
		args[i] = mediaIDs[i]
	}
	query := `SELECT media_id, stream_index, language, title FROM embedded_audio_tracks WHERE media_id IN (` + strings.Join(placeholders, ",") + `) ORDER BY media_id, stream_index`
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int][]EmbeddedAudioTrack)
	for rows.Next() {
		var track EmbeddedAudioTrack
		if err := rows.Scan(&track.MediaID, &track.StreamIndex, &track.Language, &track.Title); err != nil {
			return nil, err
		}
		out[track.MediaID] = append(out[track.MediaID], track)
	}
	return out, rows.Err()
}

func GetMediaByID(db *sql.DB, id int) (*MediaItem, error) {
	var kind string
	var refID int
	err := db.QueryRow(`SELECT kind, ref_id FROM media_global WHERE id = ?`, id).Scan(&kind, &refID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	table := mediaTableForKind(kind)
	var libID int
	var title, path string
	var duration int
	var season, episode int
	var metadataReviewNeeded sql.NullBool
	var metadataConfirmed sql.NullBool
	var overview, posterPath, backdropPath, releaseDate, thumbnailPath, matchStatus, imdbID sql.NullString
	var voteAvg, imdbRating sql.NullFloat64
	var tmdbID sql.NullInt64
	var tvdbID sql.NullString
	var artist, album, albumArtist sql.NullString
	var musicPosterPath sql.NullString
	var discNumber, trackNumber, releaseYear int
	var fileSizeBytes int64
	var fileModTime, fileHash, fileHashKind, missingSince string
	switch table {
	case "music_tracks":
		err = db.QueryRow(`SELECT m.id, m.library_id, m.title, m.path, m.duration, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.missing_since, ''), m.match_status, m.artist, m.album, m.album_artist, m.poster_path, COALESCE(m.disc_number, 0), COALESCE(m.track_number, 0), COALESCE(m.release_year, 0) FROM music_tracks m WHERE m.id = ?`, refID).
			Scan(&refID, &libID, &title, &path, &duration, &fileSizeBytes, &fileModTime, &fileHash, &fileHashKind, &missingSince, &matchStatus, &artist, &album, &albumArtist, &musicPosterPath, &discNumber, &trackNumber, &releaseYear)
	case "tv_episodes", "anime_episodes":
		err = db.QueryRow(`SELECT m.id, m.library_id, m.title, m.path, m.duration, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.missing_since, ''), m.match_status, m.tmdb_id, m.tvdb_id, m.overview, m.poster_path, m.backdrop_path, m.release_date, m.vote_average, m.imdb_id, m.imdb_rating, COALESCE(m.season, 0), COALESCE(m.episode, 0), COALESCE(m.metadata_review_needed, 0), COALESCE(m.metadata_confirmed, 0), m.thumbnail_path FROM `+table+` m WHERE m.id = ?`, refID).
			Scan(&refID, &libID, &title, &path, &duration, &fileSizeBytes, &fileModTime, &fileHash, &fileHashKind, &missingSince, &matchStatus, &tmdbID, &tvdbID, &overview, &posterPath, &backdropPath, &releaseDate, &voteAvg, &imdbID, &imdbRating, &season, &episode, &metadataReviewNeeded, &metadataConfirmed, &thumbnailPath)
	default:
		err = db.QueryRow(`SELECT m.id, m.library_id, m.title, m.path, m.duration, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.missing_since, ''), m.match_status, m.tmdb_id, m.tvdb_id, m.overview, m.poster_path, m.backdrop_path, m.release_date, m.vote_average, m.imdb_id, m.imdb_rating FROM `+table+` m WHERE m.id = ?`, refID).
			Scan(&refID, &libID, &title, &path, &duration, &fileSizeBytes, &fileModTime, &fileHash, &fileHashKind, &missingSince, &matchStatus, &tmdbID, &tvdbID, &overview, &posterPath, &backdropPath, &releaseDate, &voteAvg, &imdbID, &imdbRating)
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	m := MediaItem{
		ID:            id,
		LibraryID:     libID,
		Title:         title,
		Path:          path,
		Duration:      duration,
		Type:          kind,
		FileSizeBytes: fileSizeBytes,
		FileModTime:   fileModTime,
		FileHash:      fileHash,
		FileHashKind:  fileHashKind,
		MissingSince:  missingSince,
	}
	if matchStatus.Valid {
		m.MatchStatus = matchStatus.String
	}
	m.Missing = m.MissingSince != ""
	switch table {
	case "tv_episodes", "anime_episodes":
		m.Season = season
		m.Episode = episode
		if metadataReviewNeeded.Valid {
			m.MetadataReviewNeeded = metadataReviewNeeded.Bool
		}
		if metadataConfirmed.Valid {
			m.MetadataConfirmed = metadataConfirmed.Bool
		}
		if thumbnailPath.Valid {
			m.ThumbnailPath = thumbnailPath.String
		}
	case "music_tracks":
		if artist.Valid {
			m.Artist = artist.String
		}
		if album.Valid {
			m.Album = album.String
		}
		if albumArtist.Valid {
			m.AlbumArtist = albumArtist.String
		}
		if musicPosterPath.Valid {
			m.PosterPath = musicPosterPath.String
		}
		m.DiscNumber = discNumber
		m.TrackNumber = trackNumber
		m.ReleaseYear = releaseYear
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
	if tmdbID.Valid {
		m.TMDBID = int(tmdbID.Int64)
	}
	if tvdbID.Valid {
		m.TVDBID = tvdbID.String
	}
	if file, err := lookupPrimaryMediaFile(db, id); err == nil {
		m.Path = file.Path
		if file.Duration > 0 {
			m.Duration = file.Duration
		}
		m.FileSizeBytes = file.FileSizeBytes
		m.FileModTime = file.FileModTime
		m.FileHash = file.FileHash
		m.FileHashKind = file.FileHashKind
		m.MissingSince = file.MissingSince
		m.Missing = file.MissingSince != ""
		if file.IntroStartSec.Valid {
			v := file.IntroStartSec.Float64
			m.IntroStartSeconds = &v
		}
		if file.IntroEndSec.Valid {
			v := file.IntroEndSec.Float64
			m.IntroEndSeconds = &v
		}
	}
	decorateMediaItemURLs(&m)
	subs, err := getSubtitlesForMedia(db, id)
	if err != nil {
		return nil, err
	}
	if subs != nil {
		m.Subtitles = subs
	} else {
		m.Subtitles = []Subtitle{}
	}
	emb, err := getEmbeddedSubtitlesForMedia(db, id)
	if err != nil {
		return nil, err
	}
	if emb != nil {
		m.EmbeddedSubtitles = emb
	} else {
		m.EmbeddedSubtitles = []EmbeddedSubtitle{}
	}
	audioTracks, err := getEmbeddedAudioTracksForMedia(db, id)
	if err != nil {
		return nil, err
	}
	if audioTracks != nil {
		m.EmbeddedAudioTracks = audioTracks
	} else {
		m.EmbeddedAudioTracks = []EmbeddedAudioTrack{}
	}
	dupSlice := []MediaItem{m}
	dupSlice, err = attachDuplicateState(db, dupSlice)
	if err != nil {
		return nil, err
	}
	m = dupSlice[0]
	return &m, nil
}

// GetMediaByLibraryID returns all media for a library (one category table only), no N+1.
func GetMediaByLibraryID(db *sql.DB, libraryID int) ([]MediaItem, error) {
	var typ string
	err := db.QueryRow(`SELECT type FROM libraries WHERE id = ?`, libraryID).Scan(&typ)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []MediaItem{}, nil
		}
		return nil, err
	}
	if typ == LibraryTypeTV || typ == LibraryTypeAnime {
		if err := ensureLibraryShowsAndSeasons(db, libraryID, typ); err != nil {
			return nil, err
		}
	}
	items, _, err := queryMediaByLibraryID(db, libraryID, typ, 0, 0)
	if err != nil {
		return nil, err
	}
	items, err = attachMediaFilesBatch(db, items)
	if err != nil {
		return nil, err
	}
	items, err = attachSubtitlesBatch(db, items)
	if err != nil {
		return nil, err
	}
	return attachDuplicateState(db, items)
}

func GetMediaPageByLibraryID(db *sql.DB, libraryID int, offset int, limit int) (LibraryMediaPage, error) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 60
	}
	var typ string
	err := db.QueryRow(`SELECT type FROM libraries WHERE id = ?`, libraryID).Scan(&typ)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LibraryMediaPage{Items: []MediaItem{}, HasMore: false, Total: 0}, nil
		}
		return LibraryMediaPage{}, err
	}
	if typ == LibraryTypeTV || typ == LibraryTypeAnime {
		if err := ensureLibraryShowsAndSeasons(db, libraryID, typ); err != nil {
			return LibraryMediaPage{}, err
		}
	}
	items, total, err := queryMediaByLibraryID(db, libraryID, typ, offset, limit)
	if err != nil {
		return LibraryMediaPage{}, err
	}
	items, err = attachMediaFilesBatch(db, items)
	if err != nil {
		return LibraryMediaPage{}, err
	}
	hasMore := offset+len(items) < total
	var nextOffset *int
	if hasMore {
		value := offset + len(items)
		nextOffset = &value
	}
	return LibraryMediaPage{
		Items:      items,
		NextOffset: nextOffset,
		HasMore:    hasMore,
		Total:      total,
	}, nil
}

// queryMediaByLibraryID queries the single category table for this library.
func queryMediaByLibraryID(db *sql.DB, libraryID int, kind string, offset int, limit int) ([]MediaItem, int, error) {
	table := mediaTableForKind(kind)
	countQuery := `SELECT COUNT(1) FROM ` + table + ` WHERE library_id = ? AND COALESCE(missing_since, '') = ''`
	var total int
	if err := db.QueryRow(countQuery, libraryID).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := `SELECT g.id, m.library_id, m.title, m.path, m.duration, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.missing_since, ''), m.match_status, m.tmdb_id, m.tvdb_id, m.overview, m.poster_path, m.backdrop_path, m.release_date, m.vote_average, m.imdb_id, m.imdb_rating
FROM ` + table + ` m
JOIN media_global g ON g.kind = ? AND g.ref_id = m.id
WHERE m.library_id = ? AND COALESCE(m.missing_since, '') = ''
ORDER BY g.id`
	switch table {
	case "music_tracks":
		q = `SELECT g.id, m.library_id, m.title, m.path, m.duration, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.missing_since, ''), m.match_status, m.artist, m.album, m.album_artist, m.poster_path, COALESCE(m.disc_number, 0), COALESCE(m.track_number, 0), COALESCE(m.release_year, 0)
FROM music_tracks m
JOIN media_global g ON g.kind = 'music' AND g.ref_id = m.id
WHERE m.library_id = ? AND COALESCE(m.missing_since, '') = ''
ORDER BY g.id`
	case "tv_episodes", "anime_episodes":
		q = `SELECT g.id, m.library_id, m.title, m.path, m.duration, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.missing_since, ''), m.match_status, m.tmdb_id, m.tvdb_id, m.overview, m.poster_path, m.backdrop_path, m.release_date, m.vote_average, m.imdb_id, m.imdb_rating, COALESCE(m.season, 0), COALESCE(m.episode, 0), COALESCE(m.metadata_review_needed, 0), COALESCE(m.metadata_confirmed, 0), m.thumbnail_path, COALESCE(s.poster_path, ''), COALESCE(s.vote_average, 0), COALESCE(s.imdb_rating, 0)
FROM ` + table + ` m
JOIN media_global g ON g.kind = ? AND g.ref_id = m.id
LEFT JOIN shows s ON s.id = m.show_id
WHERE m.library_id = ? AND COALESCE(m.missing_since, '') = ''
ORDER BY g.id`
	default:
		q = `SELECT g.id, m.library_id, m.title, m.path, m.duration, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.missing_since, ''), m.match_status, m.tmdb_id, m.tvdb_id, m.overview, m.poster_path, m.backdrop_path, m.release_date, m.vote_average, m.imdb_id, m.imdb_rating
FROM ` + table + ` m
JOIN media_global g ON g.kind = ? AND g.ref_id = m.id
WHERE m.library_id = ? AND COALESCE(m.missing_since, '') = ''
ORDER BY g.id`
	}
	if limit > 0 {
		q += ` LIMIT ? OFFSET ?`
	}
	var rows *sql.Rows
	var err error
	if table == "music_tracks" {
		if limit > 0 {
			rows, err = db.Query(q, libraryID, limit, offset)
		} else {
			rows, err = db.Query(q, libraryID)
		}
	} else {
		if limit > 0 {
			rows, err = db.Query(q, kind, libraryID, limit, offset)
		} else {
			rows, err = db.Query(q, kind, libraryID)
		}
	}
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]MediaItem, 0)
	for rows.Next() {
		var m MediaItem
		m.Type = kind
		m.LibraryID = libraryID
		var overview, posterPath, backdropPath, releaseDate, thumbnailPath, matchStatus, imdbID sql.NullString
		var showPosterPath sql.NullString
		var voteAvg, showVoteAvg, showImdbAvg, imdbRating sql.NullFloat64
		var tmdbID sql.NullInt64
		var tvdbID sql.NullString
		var metadataReviewNeeded sql.NullBool
		var metadataConfirmed sql.NullBool
		var artist, album, albumArtist sql.NullString
		var musicPosterPath sql.NullString
		switch table {
		case "music_tracks":
			err = rows.Scan(&m.ID, &m.LibraryID, &m.Title, &m.Path, &m.Duration, &m.FileSizeBytes, &m.FileModTime, &m.FileHash, &m.FileHashKind, &m.MissingSince, &matchStatus, &artist, &album, &albumArtist, &musicPosterPath, &m.DiscNumber, &m.TrackNumber, &m.ReleaseYear)
			if artist.Valid {
				m.Artist = artist.String
			}
			if album.Valid {
				m.Album = album.String
			}
			if albumArtist.Valid {
				m.AlbumArtist = albumArtist.String
			}
			if musicPosterPath.Valid {
				m.PosterPath = musicPosterPath.String
			}
		case "tv_episodes", "anime_episodes":
			err = rows.Scan(&m.ID, &m.LibraryID, &m.Title, &m.Path, &m.Duration, &m.FileSizeBytes, &m.FileModTime, &m.FileHash, &m.FileHashKind, &m.MissingSince, &matchStatus, &tmdbID, &tvdbID, &overview, &posterPath, &backdropPath, &releaseDate, &voteAvg, &imdbID, &imdbRating, &m.Season, &m.Episode, &metadataReviewNeeded, &metadataConfirmed, &thumbnailPath, &showPosterPath, &showVoteAvg, &showImdbAvg)
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
		default:
			err = rows.Scan(&m.ID, &m.LibraryID, &m.Title, &m.Path, &m.Duration, &m.FileSizeBytes, &m.FileModTime, &m.FileHash, &m.FileHashKind, &m.MissingSince, &matchStatus, &tmdbID, &tvdbID, &overview, &posterPath, &backdropPath, &releaseDate, &voteAvg, &imdbID, &imdbRating)
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
		}
		if matchStatus.Valid {
			m.MatchStatus = matchStatus.String
		}
		m.Missing = m.MissingSince != ""
		if err != nil {
			return nil, 0, err
		}
		items = append(items, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if table == "tv_episodes" || table == "anime_episodes" {
		if err := hydrateEpisodeShowPosters(db, libraryID, kind, items); err != nil {
			return nil, 0, err
		}
	}
	return items, total, nil
}

// queryMediaByShowID returns episode rows for a single show (indexed by shows.id), ordered by season/episode.
func queryMediaByShowID(db *sql.DB, libraryID int, kind string, showID int) ([]MediaItem, error) {
	if showID <= 0 {
		return nil, nil
	}
	return queryMediaByShowIDs(db, libraryID, kind, []int{showID})
}

// queryMediaByShowIDs returns episode rows for multiple shows in one query (same columns/order semantics as single-show).
func queryMediaByShowIDs(db *sql.DB, libraryID int, kind string, showIDs []int) ([]MediaItem, error) {
	uniq := make([]int, 0, len(showIDs))
	seenID := make(map[int]struct{}, len(showIDs))
	for _, id := range showIDs {
		if id <= 0 {
			continue
		}
		if _, ok := seenID[id]; ok {
			continue
		}
		seenID[id] = struct{}{}
		uniq = append(uniq, id)
	}
	if len(uniq) == 0 {
		return nil, nil
	}
	showIDs = uniq
	table := mediaTableForKind(kind)
	if table != "tv_episodes" && table != "anime_episodes" {
		return nil, nil
	}
	placeholders := make([]string, len(showIDs))
	args := make([]interface{}, 0, 2+len(showIDs))
	args = append(args, kind, libraryID)
	for i, id := range showIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	q := `SELECT g.id, m.library_id, m.title, m.path, m.duration, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.missing_since, ''), m.match_status, m.tmdb_id, m.tvdb_id, m.overview, m.poster_path, m.backdrop_path, m.release_date, m.vote_average, m.imdb_id, m.imdb_rating, COALESCE(m.season, 0), COALESCE(m.episode, 0), COALESCE(m.metadata_review_needed, 0), COALESCE(m.metadata_confirmed, 0), m.thumbnail_path, COALESCE(m.show_id, 0), COALESCE(s.poster_path, ''), COALESCE(s.vote_average, 0), COALESCE(s.imdb_rating, 0)
FROM ` + table + ` m
JOIN media_global g ON g.kind = ? AND g.ref_id = m.id
LEFT JOIN shows s ON s.id = m.show_id
WHERE m.library_id = ? AND m.show_id IN (` + strings.Join(placeholders, ",") + `) AND COALESCE(m.missing_since, '') = ''
ORDER BY m.show_id, COALESCE(m.season, 0), COALESCE(m.episode, 0), COALESCE(m.title, ''), g.id`
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]MediaItem, 0)
	for rows.Next() {
		var m MediaItem
		m.Type = kind
		m.LibraryID = libraryID
		var overview, posterPath, backdropPath, releaseDate, thumbnailPath, matchStatus, imdbID sql.NullString
		var showPosterPath sql.NullString
		var voteAvg, showVoteAvg, showImdbAvg, imdbRating sql.NullFloat64
		var tmdbID sql.NullInt64
		var tvdbID sql.NullString
		var metadataReviewNeeded sql.NullBool
		var metadataConfirmed sql.NullBool
		err = rows.Scan(&m.ID, &m.LibraryID, &m.Title, &m.Path, &m.Duration, &m.FileSizeBytes, &m.FileModTime, &m.FileHash, &m.FileHashKind, &m.MissingSince, &matchStatus, &tmdbID, &tvdbID, &overview, &posterPath, &backdropPath, &releaseDate, &voteAvg, &imdbID, &imdbRating, &m.Season, &m.Episode, &metadataReviewNeeded, &metadataConfirmed, &thumbnailPath, &m.ShowID, &showPosterPath, &showVoteAvg, &showImdbAvg)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := hydrateEpisodeShowPosters(db, libraryID, kind, items); err != nil {
		return nil, err
	}
	return items, nil
}

func hydrateEpisodeShowPosters(db *sql.DB, libraryID int, kind string, items []MediaItem) error {
	if len(items) == 0 || (kind != LibraryTypeTV && kind != LibraryTypeAnime) {
		return nil
	}
	rows, err := db.Query(`SELECT COALESCE(tmdb_id, 0), COALESCE(title_key, ''), COALESCE(poster_path, '')
FROM shows
WHERE library_id = ? AND kind = ?`, libraryID, kind)
	if err != nil {
		return err
	}
	defer rows.Close()

	postersByTMDBID := make(map[int]string)
	postersByTitleKey := make(map[string]string)
	for rows.Next() {
		var tmdbID int
		var titleKey string
		var posterPath string
		if err := rows.Scan(&tmdbID, &titleKey, &posterPath); err != nil {
			return err
		}
		if posterPath == "" {
			continue
		}
		if tmdbID > 0 {
			postersByTMDBID[tmdbID] = posterPath
		}
		if titleKey != "" {
			postersByTitleKey[titleKey] = posterPath
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range items {
		if items[i].ShowPosterPath != "" {
			continue
		}
		if items[i].TMDBID > 0 {
			if posterPath := postersByTMDBID[items[i].TMDBID]; posterPath != "" {
				items[i].ShowPosterPath = posterPath
				continue
			}
		}
		titleKey := normalizeShowKeyTitle(items[i].Title)
		if titleKey == "" {
			continue
		}
		if posterPath := postersByTitleKey[titleKey]; posterPath != "" {
			items[i].ShowPosterPath = posterPath
		}
	}
	return nil
}

// duplicateHashQueryChunk limits bound variables per query (below SQLite's default SQLITE_MAX_VARIABLE_NUMBER).
const duplicateHashQueryChunk = 400

type duplicateStateGroupKey struct {
	libraryID int
	kind      string
}

func mediaFilesTableExists(db *sql.DB) (bool, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(1) FROM sqlite_master WHERE type = 'table' AND name = 'media_files'`).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func queryDuplicateCountsMediaFiles(db *sql.DB, kind string, libraryID int, hashes []string) (map[string]int, error) {
	if len(hashes) == 0 {
		return map[string]int{}, nil
	}
	table := mediaTableForKind(kind)
	ph := make([]string, len(hashes))
	args := make([]any, 0, 2+len(hashes))
	args = append(args, kind, libraryID)
	for i, h := range hashes {
		ph[i] = "?"
		args = append(args, h)
	}
	q := `SELECT COALESCE(mf.file_hash, ''), COUNT(1)
FROM media_files mf
JOIN media_global g ON g.id = mf.media_id
JOIN ` + table + ` m ON m.id = g.ref_id
WHERE g.kind = ?
AND m.library_id = ?
AND COALESCE(mf.file_hash, '') IN (` + strings.Join(ph, ",") + `)
AND COALESCE(mf.missing_since, '') = ''
GROUP BY COALESCE(mf.file_hash, '')`
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int, len(hashes))
	for rows.Next() {
		var hash string
		var cnt int
		if err := rows.Scan(&hash, &cnt); err != nil {
			return nil, err
		}
		out[hash] = cnt
	}
	return out, rows.Err()
}

func queryDuplicateCountsLegacy(db *sql.DB, table string, libraryID int, hashes []string) (map[string]int, error) {
	if len(hashes) == 0 {
		return map[string]int{}, nil
	}
	ph := make([]string, len(hashes))
	args := make([]any, 0, 1+len(hashes))
	args = append(args, libraryID)
	for i, h := range hashes {
		ph[i] = "?"
		args = append(args, h)
	}
	q := `SELECT COALESCE(file_hash, ''), COUNT(1) FROM ` + table + `
WHERE library_id = ?
AND COALESCE(file_hash, '') IN (` + strings.Join(ph, ",") + `)
AND COALESCE(missing_since, '') = ''
GROUP BY COALESCE(file_hash, '')`
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int, len(hashes))
	for rows.Next() {
		var hash string
		var cnt int
		if err := rows.Scan(&hash, &cnt); err != nil {
			return nil, err
		}
		out[hash] = cnt
	}
	return out, rows.Err()
}

func duplicateCountsForHashes(db *sql.DB, useMediaFiles bool, kind string, libraryID int, hashes []string) (map[string]int, error) {
	out := make(map[string]int, len(hashes))
	table := mediaTableForKind(kind)
	for start := 0; start < len(hashes); start += duplicateHashQueryChunk {
		end := start + duplicateHashQueryChunk
		if end > len(hashes) {
			end = len(hashes)
		}
		chunk := hashes[start:end]
		var part map[string]int
		var err error
		if useMediaFiles {
			part, err = queryDuplicateCountsMediaFiles(db, kind, libraryID, chunk)
		} else {
			part, err = queryDuplicateCountsLegacy(db, table, libraryID, chunk)
		}
		if err != nil {
			return nil, err
		}
		for h, c := range part {
			out[h] = c
		}
	}
	return out, nil
}

func attachDuplicateState(db *sql.DB, items []MediaItem) ([]MediaItem, error) {
	if len(items) == 0 {
		return items, nil
	}
	useMediaFiles, err := mediaFilesTableExists(db)
	if err != nil {
		return nil, err
	}
	groups := make(map[duplicateStateGroupKey]map[string]struct{})
	for i := range items {
		it := &items[i]
		if it.LibraryID <= 0 || it.Missing || it.FileHash == "" {
			it.Duplicate = false
			it.DuplicateCount = 0
			continue
		}
		gk := duplicateStateGroupKey{libraryID: it.LibraryID, kind: it.Type}
		if groups[gk] == nil {
			groups[gk] = make(map[string]struct{})
		}
		groups[gk][it.FileHash] = struct{}{}
	}
	countsByGroup := make(map[duplicateStateGroupKey]map[string]int, len(groups))
	for gk, hashSet := range groups {
		hashes := make([]string, 0, len(hashSet))
		for h := range hashSet {
			hashes = append(hashes, h)
		}
		sort.Strings(hashes)
		m, err := duplicateCountsForHashes(db, useMediaFiles, gk.kind, gk.libraryID, hashes)
		if err != nil {
			return nil, err
		}
		countsByGroup[gk] = m
	}
	for i := range items {
		it := &items[i]
		if it.LibraryID <= 0 || it.Missing || it.FileHash == "" {
			continue
		}
		gk := duplicateStateGroupKey{libraryID: it.LibraryID, kind: it.Type}
		c := countsByGroup[gk][it.FileHash]
		if c > 1 {
			it.Duplicate = true
			it.DuplicateCount = c
		} else {
			it.Duplicate = false
			it.DuplicateCount = 0
		}
	}
	return items, nil
}

func getSubtitlesForMedia(db *sql.DB, mediaID int) ([]Subtitle, error) {
	rows, err := db.Query(`SELECT id, media_id, title, language, format, path FROM subtitles WHERE media_id = ?`, mediaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []Subtitle
	for rows.Next() {
		var s Subtitle
		if err := rows.Scan(&s.ID, &s.MediaID, &s.Title, &s.Language, &s.Format, &s.Path); err != nil {
			return nil, err
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}

func getEmbeddedSubtitlesForMedia(db *sql.DB, mediaID int) ([]EmbeddedSubtitle, error) {
	rows, err := db.Query(`SELECT media_id, stream_index, language, title, COALESCE(codec, ''), supported FROM embedded_subtitles WHERE media_id = ? ORDER BY stream_index`, mediaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []EmbeddedSubtitle
	for rows.Next() {
		var s EmbeddedSubtitle
		var supportedInt sql.NullInt64
		if err := rows.Scan(&s.MediaID, &s.StreamIndex, &s.Language, &s.Title, &s.Codec, &supportedInt); err != nil {
			return nil, err
		}
		if supportedInt.Valid {
			v := supportedInt.Int64 != 0
			s.Supported = &v
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}

func getEmbeddedAudioTracksForMedia(db *sql.DB, mediaID int) ([]EmbeddedAudioTrack, error) {
	rows, err := db.Query(`SELECT media_id, stream_index, language, title FROM embedded_audio_tracks WHERE media_id = ? ORDER BY stream_index`, mediaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []EmbeddedAudioTrack
	for rows.Next() {
		var track EmbeddedAudioTrack
		if err := rows.Scan(&track.MediaID, &track.StreamIndex, &track.Language, &track.Title); err != nil {
			return nil, err
		}
		tracks = append(tracks, track)
	}
	return tracks, rows.Err()
}

func GetSubtitleByID(db *sql.DB, id int) (*Subtitle, error) {
	var s Subtitle
	err := db.QueryRow(`SELECT id, media_id, title, language, format, path FROM subtitles WHERE id = ?`, id).
		Scan(&s.ID, &s.MediaID, &s.Title, &s.Language, &s.Format, &s.Path)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

func getMediaDuration(ctx context.Context, path string) (int, error) {
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, "ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	var d float64
	if _, err := fmt.Sscanf(string(out), "%f", &d); err != nil {
		return 0, err
	}
	return int(d), nil
}

type VideoProbeResult struct {
	Duration            int
	EmbeddedSubtitles   []EmbeddedSubtitle
	EmbeddedAudioTracks []EmbeddedAudioTrack
	IntroStartSeconds   *float64
	IntroEndSeconds     *float64
}

func probeVideoMetadata(ctx context.Context, path string) (VideoProbeResult, error) {
	// Large UHD remuxes: ffprobe may need to read InputProbeBeforeI (128 MiB) from slow disks/NAS;
	// a short timeout yields empty embedded subtitle metadata so clients only see in-band CEA-608.
	probeCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	args := []string{"-v", "error"}
	args = append(args, ffopts.InputProbeBeforeI...)
	args = append(args,
		"-show_entries", "format=duration:stream=index,codec_type,codec_name:stream_tags=language,title",
		"-show_entries", "chapter=start_time,end_time:chapter_tags=title",
		"-of", "json",
		path,
	)
	cmd := exec.CommandContext(probeCtx, "ffprobe", args...)
	out, err := cmd.Output()
	if err != nil {
		return VideoProbeResult{}, err
	}

	var parsed struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			Index     int    `json:"index"`
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
			Tags      struct {
				Language string `json:"language"`
				Title    string `json:"title"`
			} `json:"tags"`
		} `json:"streams"`
		Chapters []struct {
			StartTime string `json:"start_time"`
			EndTime   string `json:"end_time"`
			Tags      struct {
				Title string `json:"title"`
			} `json:"tags"`
		} `json:"chapters"`
	}

	if err := json.Unmarshal(out, &parsed); err != nil {
		return VideoProbeResult{}, err
	}

	result := VideoProbeResult{}
	if parsed.Format.Duration != "" {
		if f, err := strconv.ParseFloat(parsed.Format.Duration, 64); err == nil {
			result.Duration = int(f)
		}
	}
	for _, stream := range parsed.Streams {
		lang := stream.Tags.Language
		if lang == "" {
			lang = "und"
		}
		title := stream.Tags.Title
		if title == "" {
			title = lang
		}
		switch stream.CodecType {
		case "subtitle":
			codec := strings.TrimSpace(stream.CodecName)
			var supportedPtr *bool
			if codec != "" {
				supported := isSupportedEmbeddedSubtitleCodec(codec)
				supportedPtr = &supported
			}
			result.EmbeddedSubtitles = append(result.EmbeddedSubtitles, EmbeddedSubtitle{
				StreamIndex: stream.Index,
				Language:    lang,
				Title:       title,
				Codec:       codec,
				Supported:   supportedPtr,
			})
		case "audio":
			result.EmbeddedAudioTracks = append(result.EmbeddedAudioTracks, EmbeddedAudioTrack{
				StreamIndex: stream.Index,
				Language:    lang,
				Title:       title,
			})
		}
	}
	chProbes := make([]chapterProbe, 0, len(parsed.Chapters))
	for _, ch := range parsed.Chapters {
		st, errSt := strconv.ParseFloat(ch.StartTime, 64)
		et, errEt := strconv.ParseFloat(ch.EndTime, 64)
		if errSt != nil || errEt != nil {
			continue
		}
		chProbes = append(chProbes, chapterProbe{
			startSec: st,
			endSec:   et,
			title:    ch.Tags.Title,
		})
	}
	if start, end, ok := IntroChapterRangeFromProbes(chProbes); ok {
		s, e := start, end
		// Discard intro window that extends beyond the probed duration.
		if result.Duration > 0 && e > float64(result.Duration) {
			e = float64(result.Duration)
		}
		if s >= 0 && e > s {
			result.IntroStartSeconds = &s
			result.IntroEndSeconds = &e
		}
	}
	return result, nil
}

func isSupportedEmbeddedSubtitleCodec(codec string) bool {
	switch strings.ToLower(strings.TrimSpace(codec)) {
	case "ass", "ssa", "subrip", "srt", "webvtt", "text", "mov_text", "ttml", "tx3g",
		"hdmv_text_subtitle", // Blu-ray TextST; ffmpeg can mux to WebVTT
		"eia_608", "eia_708", // ATSC closed captions
		"hdmv_pgs_subtitle", "pgssub", "pgs": // bitmap; WebVTT ineligible but Exo/Media3 can render raw PGS
		return true
	default:
		return false
	}
}

func probeEmbeddedSubtitles(ctx context.Context, path string) ([]EmbeddedSubtitle, error) {
	result, err := probeVideoMetadata(ctx, path)
	if err != nil {
		return nil, err
	}
	return result.EmbeddedSubtitles, nil
}

func probeEmbeddedAudioTracks(ctx context.Context, path string) ([]EmbeddedAudioTrack, error) {
	result, err := probeVideoMetadata(ctx, path)
	if err != nil {
		return nil, err
	}
	return result.EmbeddedAudioTracks, nil
}

func scanForSubtitles(ctx context.Context, dbConn *sql.DB, mediaID int, videoPath string) error {
	dir := filepath.Dir(videoPath)
	base := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, base) {
			ext := strings.ToLower(filepath.Ext(name))
			if ext == ".srt" || ext == ".vtt" || ext == ".ass" || ext == ".ssa" {
				path := filepath.Join(dir, name)
				lang := "und"
				parts := strings.Split(strings.TrimSuffix(name, ext), ".")
				if len(parts) > 1 {
					lastPart := parts[len(parts)-1]
					if len(lastPart) == 2 || len(lastPart) == 3 {
						lang = lastPart
					}
				}

				_, err := dbConn.ExecContext(ctx,
					`INSERT OR IGNORE INTO subtitles (media_id, title, language, format, path) VALUES (?, ?, ?, ?, ?)`,
					mediaID, name, lang, ext[1:], path,
				)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func persistEmbeddedStreams(ctx context.Context, dbConn *sql.DB, mediaID int, subtitles []EmbeddedSubtitle, audioTracks []EmbeddedAudioTrack) {
	if mediaID <= 0 {
		return
	}
	if _, err := dbConn.ExecContext(ctx, `DELETE FROM embedded_subtitles WHERE media_id = ?`, mediaID); err != nil {
		slog.Warn("clear embedded_subtitles", "media_id", mediaID, "error", err)
	} else {
		for _, s := range subtitles {
			var supportedVal interface{}
			if s.Supported != nil {
				if *s.Supported {
					supportedVal = 1
				} else {
					supportedVal = 0
				}
			}
			if _, err := dbConn.ExecContext(ctx,
				`INSERT INTO embedded_subtitles (media_id, stream_index, language, title, codec, supported) VALUES (?, ?, ?, ?, ?, ?)`,
				mediaID, s.StreamIndex, s.Language, s.Title, s.Codec, supportedVal,
			); err != nil {
				slog.Warn("insert embedded_subtitles", "media_id", mediaID, "error", err)
			}
		}
	}

	if _, err := dbConn.ExecContext(ctx, `DELETE FROM embedded_audio_tracks WHERE media_id = ?`, mediaID); err != nil {
		slog.Warn("clear embedded_audio_tracks", "media_id", mediaID, "error", err)
	} else {
		for _, track := range audioTracks {
			if _, err := dbConn.ExecContext(ctx, `INSERT INTO embedded_audio_tracks (media_id, stream_index, language, title) VALUES (?, ?, ?, ?)`, mediaID, track.StreamIndex, track.Language, track.Title); err != nil {
				slog.Warn("insert embedded_audio_tracks", "media_id", mediaID, "error", err)
			}
		}
	}
}

// HandleScanLibrary walks the given filesystem path and inserts supported media files
// into the category table for this library type only (movies, tv_episodes, anime_episodes, or music_tracks).
// libraryID must be > 0. mediaType must be tv, movie, music, or anime.
// id may be nil; then no metadata lookup is performed.
func HandleScanLibrary(ctx context.Context, dbConn *sql.DB, root, mediaType string, libraryID int, id metadata.Identifier) (ScanResult, error) {
	var musicIdentifier metadata.MusicIdentifier
	if detected, ok := id.(metadata.MusicIdentifier); ok {
		musicIdentifier = detected
	}
	return HandleScanLibraryWithOptions(ctx, dbConn, root, mediaType, libraryID, ScanOptions{
		Identifier:             id,
		MusicIdentifier:        musicIdentifier,
		ProbeMedia:             true,
		ProbeEmbeddedSubtitles: true,
		ScanSidecarSubtitles:   true,
	})
}

type scanCandidate struct {
	Path    string
	RelPath string
	Name    string
	Size    int64
	ModTime string
}

func EstimateLibraryFiles(ctx context.Context, root, mediaType string) (int, error) {
	count := 0
	err := iterateLibraryFiles(ctx, root, mediaType, nil, nil, func(scanCandidate) error {
		count++
		return nil
	})
	return count, err
}

func iterateLibraryFiles(
	ctx context.Context,
	root, mediaType string,
	onDirectory func(string),
	onSkip func(),
	visit func(scanCandidate) error,
) error {
	if root == "" {
		return fmt.Errorf("path is required")
	}
	if mediaType == "" {
		mediaType = LibraryTypeMovie
	}

	info, err := os.Stat(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf(
				"path not found: %q — use a path visible to the backend process. In Docker that usually means the container mount path (for example /tv, /movies, /anime, /music); in local dev it may be the host path from PLUM_MEDIA_*_PATH in .env",
				root,
			)
		}
		return fmt.Errorf("stat path: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory")
	}

	exts := allowedExtensions(mediaType)
	return filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if onDirectory != nil && path != root {
				onDirectory(path)
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		ext := strings.ToLower(filepath.Ext(d.Name()))
		if _, ok := exts[ext]; !ok {
			return nil
		}
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if shouldSkipScanPath(mediaType, relPath, d.Name()) {
			if onSkip != nil {
				onSkip()
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return visit(scanCandidate{
			Path:    path,
			RelPath: relPath,
			Name:    d.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime().UTC().Format(time.RFC3339Nano),
		})
	})
}

const (
	scanInsertChunkSize   = 25
	EnrichmentWorkerCount = 2
	enrichmentWorkerCount = EnrichmentWorkerCount
)

type pendingDiscoveredInsert struct {
	Item MediaItem
}

func buildScannedMediaItem(root, kind string, candidate scanCandidate) (MediaItem, metadata.MediaInfo, metadata.MusicInfo, error) {
	title := strings.TrimSuffix(candidate.Name, filepath.Ext(candidate.Name))
	if title == "" {
		title = candidate.Name
	}

	item := MediaItem{
		Title:         title,
		Path:          candidate.Path,
		Type:          kind,
		MatchStatus:   MatchStatusLocal,
		FileSizeBytes: candidate.Size,
		FileModTime:   candidate.ModTime,
	}

	var fileInfo metadata.MediaInfo
	var musicInfo metadata.MusicInfo
	switch kind {
	case LibraryTypeMusic:
		pathInfo := metadata.ParsePathForMusic(candidate.RelPath, candidate.Name)
		merged := metadata.MergeMusicMetadata(pathInfo, metadata.MusicMetadata{}, title)
		item.Title = merged.Title
		item.Artist = merged.Artist
		item.Album = merged.Album
		item.AlbumArtist = merged.AlbumArtist
		item.DiscNumber = merged.DiscNumber
		item.TrackNumber = merged.TrackNumber
		item.ReleaseYear = merged.ReleaseYear
		musicInfo = metadata.MusicInfo{
			Title:       merged.Title,
			Artist:      merged.Artist,
			Album:       merged.Album,
			AlbumArtist: merged.AlbumArtist,
			DiscNumber:  merged.DiscNumber,
			TrackNumber: merged.TrackNumber,
			ReleaseYear: merged.ReleaseYear,
		}
	case LibraryTypeMovie:
		movieInfo := metadata.ParseMovie(candidate.RelPath, candidate.Name)
		item.Title = metadata.MovieDisplayTitle(movieInfo, title)
		fileInfo = metadata.MovieMediaInfo(movieInfo)
	case LibraryTypeTV, LibraryTypeAnime:
		fileInfo = metadata.ParseFilename(candidate.Name)
		pathInfo := metadata.ParsePathForTV(candidate.RelPath, candidate.Name)
		merged := metadata.MergePathInfo(pathInfo, fileInfo)
		showRoot := metadata.ShowRootPath(root, candidate.Path)
		metadata.ApplyShowNFO(&merged, showRoot)
		if kind == LibraryTypeAnime && merged.IsSpecial && merged.Episode > 0 {
			merged.Season = 0
		}
		item.Season = merged.Season
		item.Episode = merged.Episode
		item.Title = buildEpisodeDisplayTitle(pathInfo.ShowName, merged, title, fileInfo.Title)
		fileInfo = merged
	default:
		return MediaItem{}, metadata.MediaInfo{}, metadata.MusicInfo{}, fmt.Errorf("unsupported media type %q", kind)
	}

	return item, fileInfo, musicInfo, nil
}

func flushPendingDiscoveredInserts(
	ctx context.Context,
	dbConn *sql.DB,
	table, kind string,
	libraryID int,
	pending []pendingDiscoveredInsert,
	seenAt string,
) ([]EnrichmentTask, error) {
	if len(pending) == 0 {
		return nil, nil
	}

	tx, err := dbConn.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	rollback := true
	defer func() {
		if rollback {
			_ = tx.Rollback()
		}
	}()

	insertSQL := `INSERT INTO ` + table + ` (library_id, title, path, duration, file_size_bytes, file_mod_time, file_hash, file_hash_kind, last_seen_at, missing_since, match_status, tmdb_id, tvdb_id, overview, poster_path, backdrop_path, release_date, vote_average, imdb_id, imdb_rating) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`
	switch table {
	case "music_tracks":
		insertSQL = `INSERT INTO music_tracks (library_id, title, path, duration, file_size_bytes, file_mod_time, file_hash, file_hash_kind, last_seen_at, missing_since, match_status, artist, album, album_artist, poster_path, musicbrainz_artist_id, musicbrainz_release_group_id, musicbrainz_release_id, musicbrainz_recording_id, disc_number, track_number, release_year) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`
	case "tv_episodes", "anime_episodes":
		insertSQL = `INSERT INTO ` + table + ` (library_id, title, path, duration, file_size_bytes, file_mod_time, file_hash, file_hash_kind, last_seen_at, missing_since, match_status, tmdb_id, tvdb_id, overview, poster_path, backdrop_path, release_date, vote_average, imdb_id, imdb_rating, season, episode, metadata_review_needed, metadata_confirmed) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`
	}
	insertStmt, err := tx.PrepareContext(ctx, insertSQL)
	if err != nil {
		return nil, err
	}
	defer insertStmt.Close()

	globalStmt, err := tx.PrepareContext(ctx, `INSERT INTO media_global (kind, ref_id) VALUES (?, ?) RETURNING id`)
	if err != nil {
		return nil, err
	}
	defer globalStmt.Close()

	mediaFileStmt, err := tx.PrepareContext(ctx, `INSERT INTO media_files (
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
updated_at = excluded.updated_at`)
	if err != nil {
		return nil, err
	}
	defer mediaFileStmt.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	tasks := make([]EnrichmentTask, 0, len(pending))
	for _, pendingInsert := range pending {
		item := pendingInsert.Item
		var refID int
		switch table {
		case "music_tracks":
			err = insertStmt.QueryRowContext(ctx,
				libraryID, item.Title, item.Path, item.Duration, item.FileSizeBytes, nullStr(item.FileModTime), nullStr(item.FileHash), nullStr(item.FileHashKind), nullStr(seenAt), item.MatchStatus, nullStr(item.Artist), nullStr(item.Album), nullStr(item.AlbumArtist), nullStr(item.PosterPath), nullStr(item.MusicBrainzArtistID), nullStr(item.MusicBrainzReleaseGroupID), nullStr(item.MusicBrainzReleaseID), nullStr(item.MusicBrainzRecordingID), item.DiscNumber, item.TrackNumber, item.ReleaseYear,
			).Scan(&refID)
		case "tv_episodes", "anime_episodes":
			err = insertStmt.QueryRowContext(ctx,
				libraryID, item.Title, item.Path, item.Duration, item.FileSizeBytes, nullStr(item.FileModTime), nullStr(item.FileHash), nullStr(item.FileHashKind), nullStr(seenAt), item.MatchStatus, item.TMDBID, nullStr(item.TVDBID), nullStr(item.Overview), nullStr(item.PosterPath), nullStr(item.BackdropPath), nullStr(item.ReleaseDate), nullFloat64(item.VoteAverage), nullStr(item.IMDbID), nullFloat64(item.IMDbRating), item.Season, item.Episode, item.MetadataReviewNeeded, item.MetadataConfirmed,
			).Scan(&refID)
		default:
			err = insertStmt.QueryRowContext(ctx,
				libraryID, item.Title, item.Path, item.Duration, item.FileSizeBytes, nullStr(item.FileModTime), nullStr(item.FileHash), nullStr(item.FileHashKind), nullStr(seenAt), item.MatchStatus, item.TMDBID, nullStr(item.TVDBID), nullStr(item.Overview), nullStr(item.PosterPath), nullStr(item.BackdropPath), nullStr(item.ReleaseDate), nullFloat64(item.VoteAverage), nullStr(item.IMDbID), nullFloat64(item.IMDbRating),
			).Scan(&refID)
		}
		if err != nil {
			return nil, err
		}

		var globalID int
		if err := globalStmt.QueryRowContext(ctx, kind, refID).Scan(&globalID); err != nil {
			return nil, err
		}
		if _, err := mediaFileStmt.ExecContext(ctx,
			globalID,
			item.Path,
			item.FileSizeBytes,
			nullStr(item.FileModTime),
			nullStr(item.FileHash),
			nullStr(item.FileHashKind),
			item.Duration,
			nil,
			nullStr(seenAt),
			1,
			now,
			now,
		); err != nil {
			return nil, err
		}
		tasks = append(tasks, EnrichmentTask{
			LibraryID: libraryID,
			Kind:      kind,
			RefID:     refID,
			GlobalID:  globalID,
			Path:      item.Path,
		})
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	rollback = false
	return tasks, nil
}

// findRelocatedTVEpisodeRow returns an existing row for the same library, show folder, season, and
// episode whose file path is gone (rename/move) so discovery can update the path instead of inserting
// a duplicate. Rows for other shows under the same library are ignored so S01E01 in Show A cannot
// steal a relocation match from Show B.
func findRelocatedTVEpisodeRow(ctx context.Context, dbConn *sql.DB, table, kind string, libraryID, season, episode int, libraryRoot, newPath string) (existingMediaRow, bool, error) {
	var zero existingMediaRow
	if table != "tv_episodes" && table != "anime_episodes" {
		return zero, false, nil
	}
	if episode <= 0 {
		return zero, false, nil
	}
	newShowRoot := filepath.Clean(metadata.ShowRootPath(libraryRoot, newPath))
	if newShowRoot == "" || newShowRoot == "." {
		return zero, false, nil
	}
	query := `SELECT m.path, m.id, COALESCE(g.id, 0), COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.duration, 0), COALESCE(m.last_seen_at, ''), COALESCE(m.missing_since, ''), COALESCE(m.tmdb_id, 0), m.tvdb_id, m.imdb_id, COALESCE(m.match_status, 'local'), COALESCE(m.metadata_review_needed, 0), COALESCE(m.metadata_confirmed, 0)
FROM ` + table + ` m
LEFT JOIN media_global g ON g.kind = ? AND g.ref_id = m.id
WHERE m.library_id = ? AND COALESCE(m.season, 0) = ? AND COALESCE(m.episode, 0) = ?`
	rows, err := dbConn.QueryContext(ctx, query, kind, libraryID, season, episode)
	if err != nil {
		return zero, false, err
	}
	defer rows.Close()

	var absent []existingMediaRow
	for rows.Next() {
		var row existingMediaRow
		var tvdbID, imdbID sql.NullString
		if err := rows.Scan(&row.Path, &row.RefID, &row.GlobalID, &row.FileSizeBytes, &row.FileModTime, &row.FileHash, &row.FileHashKind, &row.Duration, &row.LastSeenAt, &row.MissingSince, &row.TMDBID, &tvdbID, &imdbID, &row.MatchStatus, &row.MetadataReviewNeeded, &row.MetadataConfirmed); err != nil {
			return zero, false, err
		}
		if tvdbID.Valid {
			row.TVDBID = tvdbID.String
		}
		if imdbID.Valid {
			row.IMDbID = imdbID.String
		}
		rowShowRoot := filepath.Clean(metadata.ShowRootPath(libraryRoot, row.Path))
		if rowShowRoot != newShowRoot {
			continue
		}
		if row.Path == newPath {
			continue
		}
		if row.MissingSince != "" {
			absent = append(absent, row)
			continue
		}
		if _, statErr := os.Stat(row.Path); errors.Is(statErr, os.ErrNotExist) {
			absent = append(absent, row)
		}
	}
	if err := rows.Err(); err != nil {
		return zero, false, err
	}
	if len(absent) != 1 {
		return zero, false, nil
	}
	return absent[0], true, nil
}

func appendPlaceholders(dst []string, count int) []string {
	for i := 0; i < count; i++ {
		dst = append(dst, "?")
	}
	return dst
}

func batchUpdateMissingMedia(ctx context.Context, dbConn *sql.DB, table, kind string, staleIDs []int, missingSince string) error {
	if len(staleIDs) == 0 {
		return nil
	}
	for start := 0; start < len(staleIDs); start += scanInsertChunkSize {
		end := start + scanInsertChunkSize
		if end > len(staleIDs) {
			end = len(staleIDs)
		}
		chunk := staleIDs[start:end]
		placeholders := appendPlaceholders(nil, len(chunk))
		args := make([]any, 0, len(chunk)+1)
		args = append(args, missingSince)
		for _, id := range chunk {
			args = append(args, id)
		}
		if _, err := dbConn.ExecContext(ctx,
			`UPDATE `+table+` SET missing_since = ?, last_seen_at = COALESCE(last_seen_at, '') WHERE id IN (`+strings.Join(placeholders, ",")+`)`,
			args...,
		); err != nil {
			return err
		}

		now := time.Now().UTC().Format(time.RFC3339)
		mediaArgs := make([]any, 0, len(chunk)+2)
		mediaArgs = append(mediaArgs, missingSince, now, kind)
		for _, id := range chunk {
			mediaArgs = append(mediaArgs, id)
		}
		if _, err := dbConn.ExecContext(ctx,
			`UPDATE media_files
			    SET missing_since = ?, updated_at = ?
			  WHERE media_id IN (
				SELECT id FROM media_global WHERE kind = ? AND ref_id IN (`+strings.Join(placeholders, ",")+`)
			  )`,
			mediaArgs...,
		); err != nil && !strings.Contains(strings.ToLower(err.Error()), "no such table: media_files") {
			return err
		}
	}
	return nil
}

func ScanLibraryDiscovery(
	ctx context.Context,
	dbConn *sql.DB,
	root, mediaType string,
	libraryID int,
	options ScanOptions,
) (ScanDelta, error) {
	delta := ScanDelta{}
	if mediaType == "" {
		mediaType = LibraryTypeMovie
	}
	if libraryID <= 0 {
		return delta, fmt.Errorf("library id is required")
	}

	kind := mediaType
	table := mediaTableForKind(kind)
	scanSubpaths, err := NormalizeScanSubpaths(options.Subpaths)
	if err != nil {
		return delta, err
	}
	scanRoots, markRoots, err := resolveScanRoots(root, scanSubpaths)
	if err != nil {
		return delta, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	existingByPath, err := preloadExistingMediaByPath(dbConn, table, kind, libraryID)
	if err != nil {
		return delta, err
	}

	seenPaths := map[string]struct{}{}
	pending := make([]pendingDiscoveredInsert, 0, scanInsertChunkSize)
	emitProgress := func() {
		if options.Progress != nil {
			options.Progress(ScanProgress{
				Processed: delta.Result.Added + delta.Result.Updated + delta.Result.Skipped,
				Result:    delta.Result,
			})
		}
	}
	flushPending := func() error {
		if len(pending) == 0 {
			return nil
		}
		tasks, err := flushPendingDiscoveredInserts(ctx, dbConn, table, kind, libraryID, pending, now)
		if err != nil {
			return err
		}
		delta.TouchedFiles = append(delta.TouchedFiles, tasks...)
		for range pending {
			delta.Result.Added++
			emitProgress()
		}
		pending = pending[:0]
		return nil
	}

	for _, scanRoot := range scanRoots {
		err = iterateLibraryFiles(ctx, scanRoot, kind, func(path string) {
			if options.Activity != nil {
				options.Activity(ScanActivity{
					Phase:  "discovery",
					Target: "directory",
					Path:   path,
				})
			}
		}, func() {
			delta.Result.Skipped++
			emitProgress()
		}, func(candidate scanCandidate) error {
			if options.Activity != nil {
				options.Activity(ScanActivity{
					Phase:  "discovery",
					Target: "file",
					Path:   candidate.Path,
				})
			}
			if _, ok := seenPaths[candidate.Path]; ok {
				return nil
			}
			seenPaths[candidate.Path] = struct{}{}

			relPath, err := filepath.Rel(root, candidate.Path)
			if err != nil {
				return err
			}
			candidate.RelPath = relPath

			existing := existingByPath[candidate.Path]
			isNew := existing.RefID == 0

			item, _, _, err := buildScannedMediaItem(root, kind, candidate)
			if err != nil {
				return err
			}
			if isNew && (kind == LibraryTypeTV || kind == LibraryTypeAnime) {
				relocated, ok, err := findRelocatedTVEpisodeRow(ctx, dbConn, table, kind, libraryID, item.Season, item.Episode, root, candidate.Path)
				if err != nil {
					return err
				}
				if ok {
					delete(existingByPath, relocated.Path)
					existing = relocated
					isNew = false
				}
			}
			if !isNew {
				applyExistingMetadata(&item, existing, kind)
				if kind == LibraryTypeTV || kind == LibraryTypeAnime {
					item.MetadataReviewNeeded = existing.MetadataReviewNeeded
					item.MetadataConfirmed = existing.MetadataConfirmed
				}
			}

			hasStableFileState := !isNew &&
				existing.MissingSince == "" &&
				existing.FileSizeBytes == candidate.Size &&
				existing.FileModTime == candidate.ModTime
			isUnchanged := !isNew &&
				existing.Path == candidate.Path &&
				hasStableFileState &&
				existing.FileHash != "" &&
				existing.FileHashKind != ""

			if isUnchanged {
				if err := markMediaPresent(ctx, dbConn, table, existing.RefID, candidate.Size, candidate.ModTime, existing.FileHash, existing.FileHashKind, now); err != nil {
					return err
				}
				if existing.GlobalID > 0 {
					if err := upsertMediaFileForMediaID(ctx, dbConn, existing.GlobalID, MediaItem{
						Path:          candidate.Path,
						Duration:      existing.Duration,
						FileSizeBytes: candidate.Size,
						FileModTime:   candidate.ModTime,
						FileHash:      existing.FileHash,
						FileHashKind:  existing.FileHashKind,
					}, true); err != nil {
						return err
					}
				}
				delta.Result.Updated++
				emitProgress()
				return nil
			}

			item.FileHash = ""
			item.FileHashKind = ""
			if isNew {
				pending = append(pending, pendingDiscoveredInsert{Item: item})
				if len(pending) >= scanInsertChunkSize {
					return flushPending()
				}
				return nil
			}

			if err := updateScannedItem(ctx, dbConn, table, existing.RefID, item, now); err != nil {
				return err
			}
			if existing.GlobalID > 0 {
				if err := upsertMediaFileForMediaID(ctx, dbConn, existing.GlobalID, item, true); err != nil {
					return err
				}
			}
			delta.TouchedFiles = append(delta.TouchedFiles, EnrichmentTask{
				LibraryID: libraryID,
				Kind:      kind,
				RefID:     existing.RefID,
				GlobalID:  existing.GlobalID,
				Path:      item.Path,
			})
			delta.Result.Updated++
			emitProgress()
			return nil
		})
		if err != nil {
			return delta, err
		}
	}
	if err := flushPending(); err != nil {
		return delta, err
	}
	if err := markMissingMedia(ctx, dbConn, table, kind, libraryID, markRoots, seenPaths, now); err != nil {
		return delta, err
	}
	emitProgress()
	return delta, nil
}

func enrichTask(
	ctx context.Context,
	dbConn *sql.DB,
	root, mediaType string,
	libraryID int,
	task EnrichmentTask,
	options ScanOptions,
) error {
	table := mediaTableForKind(mediaType)
	existing, err := lookupExistingMedia(dbConn, table, mediaType, libraryID, task.Path)
	if err != nil {
		return err
	}
	if existing.RefID == 0 {
		return nil
	}
	info, err := os.Stat(task.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	candidate := scanCandidate{
		Path:    task.Path,
		RelPath: "",
		Name:    filepath.Base(task.Path),
		Size:    info.Size(),
		ModTime: info.ModTime().UTC().Format(time.RFC3339Nano),
	}
	if relPath, err := filepath.Rel(root, task.Path); err == nil {
		candidate.RelPath = relPath
	}

	needsRehash := existing.FileHash == "" ||
		existing.FileHashKind == "" ||
		existing.FileSizeBytes != info.Size() ||
		existing.FileModTime != candidate.ModTime
	// Skip redundant ffprobe/hash work when this task was already fully enriched (e.g. recovery
	// lists were overly broad before we filtered ListLibraryEnrichmentTasks).
	if mediaType == LibraryTypeMusic &&
		!needsRehash &&
		options.MusicIdentifier == nil &&
		existing.Duration > 0 {
		return nil
	}

	item, _, musicInfo, err := buildScannedMediaItem(root, mediaType, candidate)
	if err != nil {
		return err
	}
	applyExistingMetadata(&item, existing, mediaType)
	if mediaType == LibraryTypeTV || mediaType == LibraryTypeAnime {
		item.MetadataReviewNeeded = existing.MetadataReviewNeeded
		item.MetadataConfirmed = existing.MetadataConfirmed
	}
	if existing.Duration > 0 {
		item.Duration = existing.Duration
	}
	item.MatchStatus = existing.MatchStatus

	var (
		embeddedSubtitles []EmbeddedSubtitle
		embeddedAudio     []EmbeddedAudioTrack
		probedVideo       *VideoProbeResult
	)
	if mediaType == LibraryTypeMusic {
		if options.ProbeMedia && !SkipFFprobeInScan {
			if probed, duration, err := readAudioMetadata(ctx, task.Path); err == nil {
				merged := metadata.MergeMusicMetadata(metadata.ParsePathForMusic(candidate.RelPath, candidate.Name), probed, item.Title)
				item.Title = merged.Title
				item.Artist = merged.Artist
				item.Album = merged.Album
				item.AlbumArtist = merged.AlbumArtist
				item.DiscNumber = merged.DiscNumber
				item.TrackNumber = merged.TrackNumber
				item.ReleaseYear = merged.ReleaseYear
				item.Duration = duration
				musicInfo = metadata.MusicInfo{
					Title:       merged.Title,
					Artist:      merged.Artist,
					Album:       merged.Album,
					AlbumArtist: merged.AlbumArtist,
					DiscNumber:  merged.DiscNumber,
					TrackNumber: merged.TrackNumber,
					ReleaseYear: merged.ReleaseYear,
				}
			}
		}
		if options.MusicIdentifier != nil {
			if res := options.MusicIdentifier.IdentifyMusic(ctx, musicInfo); res != nil {
				applyMusicMatchResultToMediaItem(&item, res)
				item.MatchStatus = MatchStatusIdentified
			}
		}
	} else if options.ProbeMedia && !SkipFFprobeInScan {
		if probed, err := readVideoMetadata(ctx, task.Path); err == nil {
			probedVideo = &probed
			if probed.Duration > 0 {
				item.Duration = probed.Duration
			}
			if options.ProbeEmbeddedSubtitles {
				embeddedSubtitles = probed.EmbeddedSubtitles
			}
			embeddedAudio = probed.EmbeddedAudioTracks
		}
	}

	hash := existing.FileHash
	hashKind := existing.FileHashKind
	if needsRehash {
		var hashErr error
		hash, hashErr = computeMediaHash(ctx, task.Path)
		if hashErr != nil {
			return hashErr
		}
		hashKind = fileHashKindSampledSHA256
	} else if hashKind == "" && hash != "" {
		hashKind = fileHashKindSHA256
	}
	item.FileHash = hash
	item.FileHashKind = hashKind
	now := time.Now().UTC().Format(time.RFC3339)
	if err := updateScannedItem(ctx, dbConn, table, existing.RefID, item, now); err != nil {
		return err
	}
	globalID := task.GlobalID
	if globalID <= 0 {
		globalID = existing.GlobalID
	}
	if globalID > 0 {
		if err := upsertMediaFileForMediaID(ctx, dbConn, globalID, item, true); err != nil {
			return err
		}
		if probedVideo != nil {
			if err := UpdateMediaFileIntroFromProbe(ctx, dbConn, globalID, task.Path, *probedVideo); err != nil {
				slog.Warn("persist intro chapters", "media_id", globalID, "path", task.Path, "error", err)
			}
		}
	}
	if mediaType == LibraryTypeMusic {
		return nil
	}
	if options.ScanSidecarSubtitles && globalID > 0 {
		if err := scanForSubtitles(ctx, dbConn, globalID, task.Path); err != nil {
			slog.Warn("scan subtitles", "path", task.Path, "error", err)
		}
	}
	persistEmbeddedStreams(ctx, dbConn, globalID, embeddedSubtitles, embeddedAudio)
	return nil
}

func EnrichLibraryTasks(
	ctx context.Context,
	dbConn *sql.DB,
	root, mediaType string,
	libraryID int,
	tasks []EnrichmentTask,
	options ScanOptions,
) error {
	if len(tasks) == 0 {
		return nil
	}
	if mediaType == "" {
		mediaType = LibraryTypeMovie
	}

	// Keep enrichment narrow to the paths discovery actually touched.
	unique := make([]EnrichmentTask, 0, len(tasks))
	seen := make(map[string]struct{}, len(tasks))
	for _, task := range tasks {
		if task.Path == "" {
			continue
		}
		if _, ok := seen[task.Path]; ok {
			continue
		}
		seen[task.Path] = struct{}{}
		unique = append(unique, task)
	}
	if len(unique) == 0 {
		return nil
	}

	jobs := make(chan EnrichmentTask)
	errs := make(chan error, len(unique))
	workerCount := enrichmentWorkerCount
	if workerCount > len(unique) {
		workerCount = len(unique)
	}
	if workerCount < 1 {
		workerCount = 1
	}

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range jobs {
				if options.Activity != nil && task.Path != "" {
					options.Activity(ScanActivity{
						Phase:  "enrichment",
						Target: "file",
						Path:   task.Path,
					})
				}
				if err := enrichTask(ctx, dbConn, root, mediaType, libraryID, task, options); err != nil {
					errs <- err
				}
			}
		}()
	}
	for _, task := range unique {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return ctx.Err()
		case jobs <- task:
		}
	}
	close(jobs)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			return err
		}

	}
	return nil
}

func HandleScanLibraryWithOptions(
	ctx context.Context,
	dbConn *sql.DB,
	root, mediaType string,
	libraryID int,
	options ScanOptions,
) (ScanResult, error) {
	result := ScanResult{}
	if mediaType == "" {
		mediaType = LibraryTypeMovie
	}
	if libraryID <= 0 {
		return result, fmt.Errorf("library id is required")
	}

	kind := mediaType
	table := mediaTableForKind(kind)
	identifier := options.Identifier
	musicIdentifier := options.MusicIdentifier
	probeMedia := options.ProbeMedia
	probeEmbeddedSubtitleStreams := options.ProbeEmbeddedSubtitles && probeMedia
	scanSidecarSubtitles := options.ScanSidecarSubtitles
	hashMode := options.HashMode
	if hashMode == "" {
		hashMode = ScanHashModeInline
	}
	scanSubpaths, err := NormalizeScanSubpaths(options.Subpaths)
	if err != nil {
		return result, err
	}
	scanRoots, markRoots, err := resolveScanRoots(root, scanSubpaths)
	if err != nil {
		return result, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	existingByPath, err := preloadExistingMediaByPath(dbConn, table, kind, libraryID)
	if err != nil {
		return result, err
	}
	seenPaths := map[string]struct{}{}
	emitProgress := func() {
		if options.Progress != nil {
			options.Progress(ScanProgress{
				Processed: result.Added + result.Updated + result.Skipped,
				Result:    result,
			})
		}
	}
	for _, scanRoot := range scanRoots {
		err = iterateLibraryFiles(ctx, scanRoot, kind, func(path string) {
			if options.Activity != nil {
				options.Activity(ScanActivity{
					Phase:  "discovery",
					Target: "directory",
					Path:   path,
				})
			}
		}, func() {
			result.Skipped++
			emitProgress()
		}, func(candidate scanCandidate) error {
			if options.Activity != nil {
				options.Activity(ScanActivity{
					Phase:  "discovery",
					Target: "file",
					Path:   candidate.Path,
				})
			}
			path := candidate.Path
			if _, ok := seenPaths[path]; ok {
				return nil
			}
			seenPaths[path] = struct{}{}

			relPath, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			candidate.RelPath = relPath

			existing := existingByPath[path]
			isNew := existing.RefID == 0

			title := strings.TrimSuffix(candidate.Name, filepath.Ext(candidate.Name))
			if title == "" {
				title = candidate.Name
			}

			mItem := MediaItem{
				Title:         title,
				Path:          path,
				Type:          kind,
				MatchStatus:   MatchStatusLocal,
				FileSizeBytes: candidate.Size,
				FileModTime:   candidate.ModTime,
				FileHash:      existing.FileHash,
				FileHashKind:  existing.FileHashKind,
			}
			var fileInfo metadata.MediaInfo
			var musicInfo metadata.MusicInfo
			switch kind {
			case LibraryTypeMusic:
				pathInfo := metadata.ParsePathForMusic(candidate.RelPath, candidate.Name)
				audioMeta := metadata.MusicMetadata{}
				if probeMedia && !SkipFFprobeInScan {
					if probed, duration, err := readAudioMetadata(ctx, path); err == nil {
						audioMeta = probed
						mItem.Duration = duration
					}
				}
				merged := metadata.MergeMusicMetadata(pathInfo, audioMeta, title)
				mItem.Title = merged.Title
				mItem.Artist = merged.Artist
				mItem.Album = merged.Album
				mItem.AlbumArtist = merged.AlbumArtist
				mItem.DiscNumber = merged.DiscNumber
				mItem.TrackNumber = merged.TrackNumber
				mItem.ReleaseYear = merged.ReleaseYear
				musicInfo = metadata.MusicInfo{
					Title:       merged.Title,
					Artist:      merged.Artist,
					Album:       merged.Album,
					AlbumArtist: merged.AlbumArtist,
					DiscNumber:  merged.DiscNumber,
					TrackNumber: merged.TrackNumber,
					ReleaseYear: merged.ReleaseYear,
				}
			case LibraryTypeMovie:
				movieInfo := metadata.ParseMovie(candidate.RelPath, candidate.Name)
				mItem.Title = metadata.MovieDisplayTitle(movieInfo, title)
				fileInfo = metadata.MovieMediaInfo(movieInfo)
			case LibraryTypeTV, LibraryTypeAnime:
				fileInfo = metadata.ParseFilename(candidate.Name)
				pathInfo := metadata.ParsePathForTV(candidate.RelPath, candidate.Name)
				merged := metadata.MergePathInfo(pathInfo, fileInfo)
				showRoot := metadata.ShowRootPath(root, path)
				metadata.ApplyShowNFO(&merged, showRoot)
				if kind == LibraryTypeAnime && merged.IsSpecial && merged.Episode > 0 {
					merged.Season = 0
				}
				mItem.Season = merged.Season
				mItem.Episode = merged.Episode
				mItem.Title = buildEpisodeDisplayTitle(pathInfo.ShowName, merged, title, fileInfo.Title)
				fileInfo = merged
			}

			if isNew && (kind == LibraryTypeTV || kind == LibraryTypeAnime) {
				relocated, ok, err := findRelocatedTVEpisodeRow(ctx, dbConn, table, kind, libraryID, mItem.Season, mItem.Episode, root, path)
				if err != nil {
					return err
				}
				if ok {
					delete(existingByPath, relocated.Path)
					existing = relocated
					isNew = false
					mItem.FileHash = existing.FileHash
					mItem.FileHashKind = existing.FileHashKind
				}
			}

			hasStableFileState := !isNew &&
				existing.MissingSince == "" &&
				existing.FileSizeBytes == candidate.Size &&
				existing.FileModTime == candidate.ModTime
			isUnchanged := !isNew &&
				existing.Path == path &&
				hasStableFileState &&
				existing.FileHash != "" &&
				existing.FileHashKind != ""

			identifyInfo := fileInfo
			hasMetadata := existingHasMetadata(kind, existing)
			forceRefresh := kind != LibraryTypeMusic && hasExplicitProviderID(identifyInfo) && !existing.MetadataConfirmed
			shouldIdentify := identifier != nil &&
				(kind == LibraryTypeTV || kind == LibraryTypeAnime || kind == LibraryTypeMovie) &&
				(!hasMetadata || forceRefresh)
			shouldIdentifyMusic := kind == LibraryTypeMusic && musicIdentifier != nil
			if isUnchanged && !shouldIdentify && !shouldIdentifyMusic {
				if err := markMediaPresent(ctx, dbConn, table, existing.RefID, candidate.Size, candidate.ModTime, existing.FileHash, existing.FileHashKind, now); err != nil {
					return err
				}
				if existing.GlobalID > 0 {
					if err := upsertMediaFileForMediaID(ctx, dbConn, existing.GlobalID, MediaItem{
						Path:          path,
						Duration:      existing.Duration,
						FileSizeBytes: candidate.Size,
						FileModTime:   candidate.ModTime,
						FileHash:      existing.FileHash,
						FileHashKind:  existing.FileHashKind,
					}, true); err != nil {
						return err
					}
				}
				result.Updated++
				emitProgress()
				return nil
			}
			if shouldIdentify {
				mItem.MetadataReviewNeeded = false
				mItem.MetadataConfirmed = false
				switch kind {
				case LibraryTypeTV:
					if res := identifier.IdentifyTV(ctx, identifyInfo); res != nil {
						applyMatchResultToMediaItem(&mItem, res)
						mItem.MatchStatus = MatchStatusIdentified
					} else {
						mItem.MatchStatus = MatchStatusUnmatched
					}
				case LibraryTypeAnime:
					if res := identifier.IdentifyAnime(ctx, identifyInfo); res != nil {
						applyMatchResultToMediaItem(&mItem, res)
						mItem.MatchStatus = MatchStatusIdentified
					} else {
						mItem.MatchStatus = MatchStatusUnmatched
					}
				case LibraryTypeMovie:
					if res := identifier.IdentifyMovie(ctx, identifyInfo); res != nil {
						applyMatchResultToMediaItem(&mItem, res)
						mItem.MatchStatus = MatchStatusIdentified
					} else {
						mItem.MatchStatus = MatchStatusUnmatched
					}
				}
			} else if shouldIdentifyMusic {
				if res := musicIdentifier.IdentifyMusic(ctx, musicInfo); res != nil {
					applyMusicMatchResultToMediaItem(&mItem, res)
					mItem.MatchStatus = MatchStatusIdentified
				} else {
					mItem.MatchStatus = MatchStatusUnmatched
				}
			} else if !isNew {
				applyExistingMetadata(&mItem, existing, kind)
			}
			if (kind == LibraryTypeTV || kind == LibraryTypeAnime) && !shouldIdentify {
				mItem.MetadataReviewNeeded = existing.MetadataReviewNeeded
				mItem.MetadataConfirmed = existing.MetadataConfirmed
			}
			if (shouldIdentify || shouldIdentifyMusic) && mItem.MatchStatus == MatchStatusUnmatched {
				result.Unmatched++
			}

			itemHashMode := hashMode
			needsHashBackfill := hashMode == ScanHashModeDefer &&
				hasStableFileState &&
				!shouldIdentify &&
				!shouldIdentifyMusic &&
				(existing.FileHash == "" || existing.FileHashKind == "")
			if needsHashBackfill {
				// A deferred discovery pass may have been interrupted before enrichment ran.
				itemHashMode = ScanHashModeInline
			}
			if itemHashMode == ScanHashModeDefer {
				// Discovery scans can defer hashing so rows become visible quickly.
				mItem.FileHash = ""
				mItem.FileHashKind = ""
			} else if hasStableFileState && mItem.FileHash != "" && mItem.FileHashKind != "" {
				// Hash already valid for this size/mtime; keep existing values.
			} else if hasStableFileState && mItem.FileHash != "" && mItem.FileHashKind == "" {
				// Legacy row: hash computed before file_hash_kind existed.
				mItem.FileHashKind = fileHashKindSHA256
			} else {
				if hash, err := computeMediaHash(ctx, path); err == nil {
					mItem.FileHash = hash
					mItem.FileHashKind = fileHashKindSampledSHA256
				} else {
					return err
				}
			}

			refID := existing.RefID
			globalID := existing.GlobalID
			if isNew {
				refID, globalID, err = insertScannedItem(ctx, dbConn, table, kind, libraryID, mItem, now)
				if err != nil {
					return err
				}
				result.Added++
			} else {
				if err := updateScannedItem(ctx, dbConn, table, refID, mItem, now); err != nil {
					return err
				}
				result.Updated++
			}
			if globalID > 0 {
				if err := upsertMediaFileForMediaID(ctx, dbConn, globalID, mItem, true); err != nil {
					return err
				}
			}
			emitProgress()

			if kind == LibraryTypeMusic {
				return nil
			}
			if scanSidecarSubtitles {
				if err := scanForSubtitles(ctx, dbConn, globalID, path); err != nil {
					slog.Warn("scan subtitles", "path", path, "error", err)
				}
			}

			var (
				embeddedSubs        []EmbeddedSubtitle
				embeddedAudioTracks []EmbeddedAudioTrack
			)
			if probeMedia && !SkipFFprobeInScan {
				if probed, err := readVideoMetadata(ctx, path); err == nil {
					if mItem.Duration == 0 && probed.Duration > 0 {
						mItem.Duration = probed.Duration
						if err := updateMediaDuration(ctx, dbConn, table, refID, mItem.Duration); err != nil {
							return err
						}
						if globalID > 0 {
							if err := upsertMediaFileForMediaID(ctx, dbConn, globalID, mItem, true); err != nil {
								return err
							}
						}
					}
					if probeEmbeddedSubtitleStreams {
						embeddedSubs = probed.EmbeddedSubtitles
					}
					embeddedAudioTracks = probed.EmbeddedAudioTracks
					if globalID > 0 {
						if err := UpdateMediaFileIntroFromProbe(ctx, dbConn, globalID, path, probed); err != nil {
							slog.Warn("persist intro chapters", "media_id", globalID, "path", path, "error", err)
						}
					}
				}
			}
			persistEmbeddedStreams(ctx, dbConn, globalID, embeddedSubs, embeddedAudioTracks)
			return nil
		})
		if err != nil {
			return result, err
		}
	}
	if err := markMissingMedia(ctx, dbConn, table, kind, libraryID, markRoots, seenPaths, now); err != nil {
		return result, err
	}
	emitProgress()
	return result, nil
}

const (
	// fileHashKindSHA256 marks a legacy full-file SHA-256 (entire byte stream).
	fileHashKindSHA256 = "sha256"
	// fileHashKindSampledSHA256 is SHA-256 over file size (8-byte BE) plus three 1 MiB samples:
	// start, middle, end. For files ≤ 3 MiB the whole file is hashed (same as full-file for tiny media).
	fileHashKindSampledSHA256 = "sampled-sha256-v1"
)

// hashSampleBlock is the size of each sampled region for sampled-sha256-v1.
const hashSampleBlock = 1 << 20

// hashReadThrottleBytesPerSec limits sequential read throughput during hashing so library scans
// do not saturate the disk (0 disables throttling).
const hashReadThrottleBytesPerSec = 32 * 1024 * 1024

func NormalizeScanSubpaths(subpaths []string) ([]string, error) {
	if len(subpaths) == 0 {
		return nil, nil
	}
	normalized := make([]string, 0, len(subpaths))
	for _, subpath := range subpaths {
		subpath = strings.TrimSpace(subpath)
		if subpath == "" || subpath == "." {
			return nil, nil
		}
		clean := filepath.Clean(subpath)
		if clean == "." {
			return nil, nil
		}
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
			return nil, fmt.Errorf("invalid scan subpath %q", subpath)
		}
		normalized = append(normalized, clean)
	}
	sort.Strings(normalized)
	out := make([]string, 0, len(normalized))
	for _, subpath := range normalized {
		if len(out) > 0 && isSubpath(out[len(out)-1], subpath) {
			continue
		}
		out = append(out, subpath)
	}
	return out, nil
}

func isSubpath(parent, child string) bool {
	if parent == child {
		return true
	}
	return strings.HasPrefix(child, parent+string(os.PathSeparator))
}

func resolveScanRoots(root string, subpaths []string) ([]string, []string, error) {
	if len(subpaths) == 0 {
		return []string{root}, []string{root}, nil
	}
	roots := make([]string, 0, len(subpaths))
	markRoots := make([]string, 0, len(subpaths))
	for _, subpath := range subpaths {
		scanRoot := filepath.Join(root, subpath)
		markRoots = append(markRoots, scanRoot)
		info, err := os.Stat(scanRoot)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, nil, err
		}
		if !info.IsDir() {
			return nil, nil, fmt.Errorf("scan subpath is not a directory: %s", subpath)
		}
		roots = append(roots, scanRoot)
	}
	return roots, markRoots, nil
}

func applyMatchResultToMediaItem(item *MediaItem, res *metadata.MatchResult) {
	if item == nil || res == nil {
		return
	}
	item.Title = res.Title
	item.Overview = res.Overview
	item.PosterPath = res.PosterURL
	item.BackdropPath = res.BackdropURL
	item.ReleaseDate = res.ReleaseDate
	item.VoteAverage = res.VoteAverage
	item.IMDbID = res.IMDbID
	item.IMDbRating = res.IMDbRating
	switch res.Provider {
	case "tmdb":
		if id, err := parseInt(res.ExternalID); err == nil {
			item.TMDBID = id
			item.TVDBID = ""
		}
	case "tvdb":
		item.TVDBID = res.ExternalID
	}
}

func applyMusicMatchResultToMediaItem(item *MediaItem, res *metadata.MusicMatchResult) {
	if item == nil || res == nil {
		return
	}
	if res.Title != "" {
		item.Title = res.Title
	}
	if res.Artist != "" {
		item.Artist = res.Artist
	}
	if res.Album != "" {
		item.Album = res.Album
	}
	if res.AlbumArtist != "" {
		item.AlbumArtist = res.AlbumArtist
	}
	if res.PosterURL != "" {
		item.PosterPath = res.PosterURL
	}
	if res.ReleaseYear > 0 {
		item.ReleaseYear = res.ReleaseYear
	}
	if res.DiscNumber > 0 {
		item.DiscNumber = res.DiscNumber
	}
	if res.TrackNumber > 0 {
		item.TrackNumber = res.TrackNumber
	}
	item.MusicBrainzArtistID = res.ArtistID
	item.MusicBrainzReleaseGroupID = res.ReleaseGroupID
	item.MusicBrainzReleaseID = res.ReleaseID
	item.MusicBrainzRecordingID = res.RecordingID
}

func applyExistingMetadata(item *MediaItem, existing existingMediaRow, kind string) {
	if item == nil {
		return
	}
	item.MatchStatus = existing.MatchStatus
	item.PosterPath = existing.PosterPath
	item.TMDBID = existing.TMDBID
	item.TVDBID = existing.TVDBID
	item.IMDbID = existing.IMDbID
	item.MusicBrainzArtistID = existing.MusicBrainzArtistID
	item.MusicBrainzReleaseGroupID = existing.MusicBrainzReleaseGroupID
	item.MusicBrainzReleaseID = existing.MusicBrainzReleaseID
	item.MusicBrainzRecordingID = existing.MusicBrainzRecordingID
	if kind == LibraryTypeTV || kind == LibraryTypeAnime {
		item.MetadataReviewNeeded = existing.MetadataReviewNeeded
		item.MetadataConfirmed = existing.MetadataConfirmed
	}
}

func hashThrottleAfterRead(ctx context.Context, n int) error {
	if n <= 0 || hashReadThrottleBytesPerSec <= 0 {
		return nil
	}
	d := time.Duration(n) * time.Second / time.Duration(hashReadThrottleBytesPerSec)
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func writeFileSizeToHash(h hash.Hash, size int64) {
	_ = binary.Write(h, binary.BigEndian, uint64(size))
}

func hashFullFileThrottled(ctx context.Context, f *os.File, h hash.Hash, size int64) error {
	buf := make([]byte, hashSampleBlock)
	var read int64
	for read < size {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		n, err := f.Read(buf)
		if n > 0 {
			if _, werr := h.Write(buf[:n]); werr != nil {
				return werr
			}
			read += int64(n)
			if err := hashThrottleAfterRead(ctx, n); err != nil {
				return err
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func hashReadAtThrottled(ctx context.Context, f *os.File, h hash.Hash, off int64, size int64) error {
	if off < 0 || off >= size {
		return nil
	}
	n := int64(hashSampleBlock)
	if off+n > size {
		n = size - off
	}
	if n <= 0 {
		return nil
	}
	buf := make([]byte, n)
	got, err := f.ReadAt(buf, off)
	if got > 0 {
		if _, werr := h.Write(buf[:got]); werr != nil {
			return werr
		}
		if err := hashThrottleAfterRead(ctx, got); err != nil {
			return err
		}
	}
	if err != nil && err != io.EOF {
		return err
	}
	return nil
}

func computeFileHash(ctx context.Context, path string) (string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	size := fi.Size()

	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	writeFileSizeToHash(h, size)

	if size <= 3*hashSampleBlock {
		if err := hashFullFileThrottled(ctx, f, h, size); err != nil {
			return "", err
		}
	} else {
		mid := (size - hashSampleBlock) / 2
		for _, off := range []int64{0, mid, size - hashSampleBlock} {
			if err := hashReadAtThrottled(ctx, f, h, off, size); err != nil {
				return "", err
			}
		}
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func markMediaPresent(ctx context.Context, dbConn *sql.DB, table string, refID int, fileSizeBytes int64, fileModTime, fileHash, fileHashKind, seenAt string) error {
	_, err := dbConn.ExecContext(
		ctx,
		`UPDATE `+table+` SET file_size_bytes = ?, file_mod_time = ?, file_hash = ?, file_hash_kind = ?, last_seen_at = ?, missing_since = NULL WHERE id = ?`,
		fileSizeBytes,
		nullStr(fileModTime),
		nullStr(fileHash),
		nullStr(fileHashKind),
		nullStr(seenAt),
		refID,
	)
	return err
}

func markMissingMedia(ctx context.Context, dbConn *sql.DB, table, kind string, libraryID int, scanRoots []string, seenPaths map[string]struct{}, missingSince string) error {
	rows, err := dbConn.Query(`SELECT id, path FROM `+table+` WHERE library_id = ?`, libraryID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var staleIDs []int
	for rows.Next() {
		var refID int
		var path string
		if err := rows.Scan(&refID, &path); err != nil {
			return err
		}
		if _, ok := seenPaths[path]; ok {
			continue
		}
		if !pathWithinAnyRoot(path, scanRoots) {
			continue
		}
		staleIDs = append(staleIDs, refID)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return batchUpdateMissingMedia(ctx, dbConn, table, kind, staleIDs, missingSince)
}

func pathWithinAnyRoot(path string, roots []string) bool {
	for _, root := range roots {
		if path == root || strings.HasPrefix(path, root+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

type existingMediaRow struct {
	RefID                     int
	GlobalID                  int
	Path                      string
	FileSizeBytes             int64
	FileModTime               string
	FileHash                  string
	FileHashKind              string
	Duration                  int
	LastSeenAt                string
	MissingSince              string
	TMDBID                    int
	TVDBID                    string
	IMDbID                    string
	PosterPath                string
	MusicBrainzArtistID       string
	MusicBrainzReleaseGroupID string
	MusicBrainzReleaseID      string
	MusicBrainzRecordingID    string
	MatchStatus               string
	MetadataReviewNeeded      bool
	MetadataConfirmed         bool
}

func allowedExtensions(kind string) map[string]struct{} {
	if kind == LibraryTypeMusic {
		return audioExtensions
	}
	return videoExtensions
}

func shouldSkipScanPath(kind, relPath, filename string) bool {
	if kind != LibraryTypeMovie {
		return false
	}
	return metadata.ParseMovie(relPath, filename).IsExtra
}

func buildEpisodeDisplayTitle(showName string, info metadata.MediaInfo, fallbackTitle, fileTitle string) string {
	displayShow := strings.TrimSpace(showName)
	if normalized := strings.ToLower(displayShow); strings.HasPrefix(normalized, "season ") || strings.HasPrefix(normalized, "s0") {
		displayShow = ""
	}
	if candidate := prettifyDisplayTitle(info.Title); candidate != "" && (displayShow == "" || len(displayShow) <= 2) {
		displayShow = candidate
	}
	if displayShow == "" && info.Title != "" {
		displayShow = prettifyDisplayTitle(info.Title)
	}
	if displayShow == "" {
		displayShow = fallbackTitle
	}
	if info.Episode > 0 {
		title := fmt.Sprintf("%s - S%02dE%02d", displayShow, info.Season, info.Episode)
		extraTitle := prettifyTitle(fileTitle)
		if extraTitle != "" &&
			!metadata.IsGenericEpisodeTitle(fileTitle, info.Season, info.Episode) &&
			!strings.EqualFold(metadata.NormalizeSeriesTitle(extraTitle), metadata.NormalizeSeriesTitle(displayShow)) {
			title += " - " + extraTitle
		}
		return title
	}
	return displayShow
}

func prettifyTitle(s string) string {
	s = strings.TrimSpace(strings.TrimSuffix(s, filepath.Ext(s)))
	s = strings.ReplaceAll(s, ".", " ")
	s = strings.ReplaceAll(s, "_", " ")
	return strings.TrimSpace(s)
}

func prettifyDisplayTitle(s string) string {
	s = prettifyTitle(s)
	if s == strings.ToLower(s) {
		words := strings.Fields(s)
		for i, word := range words {
			if word == "" {
				continue
			}
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
		return strings.Join(words, " ")
	}
	return s
}

func preloadExistingMediaByPath(dbConn *sql.DB, table, kind string, libraryID int) (map[string]existingMediaRow, error) {
	query := `SELECT m.path, m.id, COALESCE(g.id, 0), COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.last_seen_at, ''), COALESCE(m.missing_since, ''), COALESCE(m.match_status, 'local') FROM ` + table + ` m
LEFT JOIN media_global g ON g.kind = ? AND g.ref_id = m.id
WHERE m.library_id = ?`
	if table == "music_tracks" {
		query = `SELECT m.path, m.id, COALESCE(g.id, 0), COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.duration, 0), COALESCE(m.last_seen_at, ''), COALESCE(m.missing_since, ''), COALESCE(m.match_status, 'local'), COALESCE(m.poster_path, ''), COALESCE(m.musicbrainz_artist_id, ''), COALESCE(m.musicbrainz_release_group_id, ''), COALESCE(m.musicbrainz_release_id, ''), COALESCE(m.musicbrainz_recording_id, '') FROM music_tracks m
LEFT JOIN media_global g ON g.kind = 'music' AND g.ref_id = m.id
WHERE m.library_id = ?`
	}
	if table == "tv_episodes" || table == "anime_episodes" {
		query = `SELECT m.path, m.id, COALESCE(g.id, 0), COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.duration, 0), COALESCE(m.last_seen_at, ''), COALESCE(m.missing_since, ''), COALESCE(m.tmdb_id, 0), COALESCE(m.tvdb_id, ''), COALESCE(m.imdb_id, ''), COALESCE(m.match_status, 'local'), COALESCE(m.metadata_review_needed, 0), COALESCE(m.metadata_confirmed, 0)
FROM ` + table + ` m
LEFT JOIN media_global g ON g.kind = ? AND g.ref_id = m.id
WHERE m.library_id = ?`
	} else if table != "music_tracks" {
		query = `SELECT m.path, m.id, COALESCE(g.id, 0), COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.duration, 0), COALESCE(m.last_seen_at, ''), COALESCE(m.missing_since, ''), COALESCE(m.tmdb_id, 0), COALESCE(m.tvdb_id, ''), COALESCE(m.imdb_id, ''), COALESCE(m.match_status, 'local')
FROM ` + table + ` m
LEFT JOIN media_global g ON g.kind = ? AND g.ref_id = m.id
WHERE m.library_id = ?`
	}

	var (
		rows *sql.Rows
		err  error
	)
	if table == "music_tracks" {
		rows, err = dbConn.Query(query, libraryID)
	} else {
		rows, err = dbConn.Query(query, kind, libraryID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]existingMediaRow)
	for rows.Next() {
		var row existingMediaRow
		switch table {
		case "music_tracks":
			if err := rows.Scan(&row.Path, &row.RefID, &row.GlobalID, &row.FileSizeBytes, &row.FileModTime, &row.FileHash, &row.FileHashKind, &row.Duration, &row.LastSeenAt, &row.MissingSince, &row.MatchStatus, &row.PosterPath, &row.MusicBrainzArtistID, &row.MusicBrainzReleaseGroupID, &row.MusicBrainzReleaseID, &row.MusicBrainzRecordingID); err != nil {
				return nil, err
			}
		case "tv_episodes", "anime_episodes":
			if err := rows.Scan(&row.Path, &row.RefID, &row.GlobalID, &row.FileSizeBytes, &row.FileModTime, &row.FileHash, &row.FileHashKind, &row.Duration, &row.LastSeenAt, &row.MissingSince, &row.TMDBID, &row.TVDBID, &row.IMDbID, &row.MatchStatus, &row.MetadataReviewNeeded, &row.MetadataConfirmed); err != nil {
				return nil, err
			}
		default:
			if err := rows.Scan(&row.Path, &row.RefID, &row.GlobalID, &row.FileSizeBytes, &row.FileModTime, &row.FileHash, &row.FileHashKind, &row.Duration, &row.LastSeenAt, &row.MissingSince, &row.TMDBID, &row.TVDBID, &row.IMDbID, &row.MatchStatus); err != nil {
				return nil, err
			}
		}
		out[row.Path] = row
	}
	return out, rows.Err()
}

func lookupExistingMedia(dbConn *sql.DB, table, kind string, libraryID int, path string) (existingMediaRow, error) {
	var row existingMediaRow
	if table == "music_tracks" {
		err := dbConn.QueryRow(`SELECT m.id, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.duration, 0), COALESCE(m.last_seen_at, ''), COALESCE(m.missing_since, ''), COALESCE(m.match_status, 'local'), COALESCE(m.poster_path, ''), COALESCE(m.musicbrainz_artist_id, ''), COALESCE(m.musicbrainz_release_group_id, ''), COALESCE(m.musicbrainz_release_id, ''), COALESCE(m.musicbrainz_recording_id, '') FROM music_tracks m WHERE m.library_id = ? AND m.path = ?`, libraryID, path).
			Scan(&row.RefID, &row.FileSizeBytes, &row.FileModTime, &row.FileHash, &row.FileHashKind, &row.Duration, &row.LastSeenAt, &row.MissingSince, &row.MatchStatus, &row.PosterPath, &row.MusicBrainzArtistID, &row.MusicBrainzReleaseGroupID, &row.MusicBrainzReleaseID, &row.MusicBrainzRecordingID)
		if errors.Is(err, sql.ErrNoRows) {
			return row, nil
		}
		if err != nil {
			return row, err
		}
	} else {
		var tvdbID, imdbID sql.NullString
		var err error
		if table == "tv_episodes" || table == "anime_episodes" {
			var metadataReviewNeeded sql.NullBool
			var metadataConfirmed sql.NullBool
			err = dbConn.QueryRow(`SELECT m.id, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.duration, 0), COALESCE(m.last_seen_at, ''), COALESCE(m.missing_since, ''), COALESCE(m.tmdb_id, 0), m.tvdb_id, m.imdb_id, COALESCE(m.match_status, 'local'), COALESCE(m.metadata_review_needed, 0), COALESCE(m.metadata_confirmed, 0) FROM `+table+` m WHERE m.library_id = ? AND m.path = ?`, libraryID, path).
				Scan(&row.RefID, &row.FileSizeBytes, &row.FileModTime, &row.FileHash, &row.FileHashKind, &row.Duration, &row.LastSeenAt, &row.MissingSince, &row.TMDBID, &tvdbID, &imdbID, &row.MatchStatus, &metadataReviewNeeded, &metadataConfirmed)
			if metadataReviewNeeded.Valid {
				row.MetadataReviewNeeded = metadataReviewNeeded.Bool
			}
			if metadataConfirmed.Valid {
				row.MetadataConfirmed = metadataConfirmed.Bool
			}
		} else {
			err = dbConn.QueryRow(`SELECT m.id, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.duration, 0), COALESCE(m.last_seen_at, ''), COALESCE(m.missing_since, ''), COALESCE(m.tmdb_id, 0), m.tvdb_id, m.imdb_id, COALESCE(m.match_status, 'local') FROM `+table+` m WHERE m.library_id = ? AND m.path = ?`, libraryID, path).
				Scan(&row.RefID, &row.FileSizeBytes, &row.FileModTime, &row.FileHash, &row.FileHashKind, &row.Duration, &row.LastSeenAt, &row.MissingSince, &row.TMDBID, &tvdbID, &imdbID, &row.MatchStatus)
		}
		if errors.Is(err, sql.ErrNoRows) {
			return row, nil
		}
		if err != nil {
			return row, err
		}
		if tvdbID.Valid {
			row.TVDBID = tvdbID.String
		}
		if imdbID.Valid {
			row.IMDbID = imdbID.String
		}
	}
	row.Path = path
	_ = dbConn.QueryRow(`SELECT id FROM media_global WHERE kind = ? AND ref_id = ?`, kind, row.RefID).Scan(&row.GlobalID)
	return row, nil
}

func insertScannedItem(ctx context.Context, dbConn *sql.DB, table, kind string, libraryID int, mItem MediaItem, seenAt string) (int, int, error) {
	tx, err := dbConn.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}

	var refID int
	switch table {
	case "music_tracks":
		err = tx.QueryRowContext(ctx, `INSERT INTO music_tracks (library_id, title, path, duration, file_size_bytes, file_mod_time, file_hash, file_hash_kind, last_seen_at, missing_since, match_status, artist, album, album_artist, poster_path, musicbrainz_artist_id, musicbrainz_release_group_id, musicbrainz_release_id, musicbrainz_recording_id, disc_number, track_number, release_year) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			libraryID, mItem.Title, mItem.Path, mItem.Duration, mItem.FileSizeBytes, nullStr(mItem.FileModTime), nullStr(mItem.FileHash), nullStr(mItem.FileHashKind), nullStr(seenAt), mItem.MatchStatus, nullStr(mItem.Artist), nullStr(mItem.Album), nullStr(mItem.AlbumArtist), nullStr(mItem.PosterPath), nullStr(mItem.MusicBrainzArtistID), nullStr(mItem.MusicBrainzReleaseGroupID), nullStr(mItem.MusicBrainzReleaseID), nullStr(mItem.MusicBrainzRecordingID), mItem.DiscNumber, mItem.TrackNumber, mItem.ReleaseYear).Scan(&refID)
		if err != nil {
			_ = tx.Rollback()
			return 0, 0, err
		}
	case "tv_episodes", "anime_episodes":
		err = tx.QueryRowContext(ctx, `INSERT INTO `+table+` (library_id, title, path, duration, file_size_bytes, file_mod_time, file_hash, file_hash_kind, last_seen_at, missing_since, match_status, tmdb_id, tvdb_id, overview, poster_path, backdrop_path, release_date, vote_average, imdb_id, imdb_rating, season, episode, metadata_review_needed, metadata_confirmed) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			libraryID, mItem.Title, mItem.Path, mItem.Duration, mItem.FileSizeBytes, nullStr(mItem.FileModTime), nullStr(mItem.FileHash), nullStr(mItem.FileHashKind), nullStr(seenAt), mItem.MatchStatus, mItem.TMDBID, nullStr(mItem.TVDBID), nullStr(mItem.Overview), nullStr(mItem.PosterPath), nullStr(mItem.BackdropPath), nullStr(mItem.ReleaseDate), nullFloat64(mItem.VoteAverage), nullStr(mItem.IMDbID), nullFloat64(mItem.IMDbRating), mItem.Season, mItem.Episode, mItem.MetadataReviewNeeded, mItem.MetadataConfirmed).Scan(&refID)
		if err != nil {
			_ = tx.Rollback()
			return 0, 0, err
		}
	default:
		err = tx.QueryRowContext(ctx, `INSERT INTO `+table+` (library_id, title, path, duration, file_size_bytes, file_mod_time, file_hash, file_hash_kind, last_seen_at, missing_since, match_status, tmdb_id, tvdb_id, overview, poster_path, backdrop_path, release_date, vote_average, imdb_id, imdb_rating) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			libraryID, mItem.Title, mItem.Path, mItem.Duration, mItem.FileSizeBytes, nullStr(mItem.FileModTime), nullStr(mItem.FileHash), nullStr(mItem.FileHashKind), nullStr(seenAt), mItem.MatchStatus, mItem.TMDBID, nullStr(mItem.TVDBID), nullStr(mItem.Overview), nullStr(mItem.PosterPath), nullStr(mItem.BackdropPath), nullStr(mItem.ReleaseDate), nullFloat64(mItem.VoteAverage), nullStr(mItem.IMDbID), nullFloat64(mItem.IMDbRating)).Scan(&refID)
		if err != nil {
			_ = tx.Rollback()
			return 0, 0, err
		}
	}
	var globalID int
	err = tx.QueryRowContext(ctx, `INSERT INTO media_global (kind, ref_id) VALUES (?, ?) RETURNING id`, kind, refID).Scan(&globalID)
	if err != nil {
		_ = tx.Rollback()
		return 0, 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return refID, globalID, nil
}

func updateScannedItem(ctx context.Context, dbConn *sql.DB, table string, refID int, mItem MediaItem, seenAt string) error {
	if table == "music_tracks" {
		_, err := dbConn.ExecContext(ctx, `UPDATE music_tracks SET title = ?, path = ?, duration = ?, file_size_bytes = ?, file_mod_time = ?, file_hash = ?, file_hash_kind = ?, last_seen_at = ?, missing_since = NULL, match_status = ?, artist = ?, album = ?, album_artist = ?, poster_path = ?, musicbrainz_artist_id = ?, musicbrainz_release_group_id = ?, musicbrainz_release_id = ?, musicbrainz_recording_id = ?, disc_number = ?, track_number = ?, release_year = ? WHERE id = ?`,
			mItem.Title, mItem.Path, mItem.Duration, mItem.FileSizeBytes, nullStr(mItem.FileModTime), nullStr(mItem.FileHash), nullStr(mItem.FileHashKind), nullStr(seenAt), mItem.MatchStatus, nullStr(mItem.Artist), nullStr(mItem.Album), nullStr(mItem.AlbumArtist), nullStr(mItem.PosterPath), nullStr(mItem.MusicBrainzArtistID), nullStr(mItem.MusicBrainzReleaseGroupID), nullStr(mItem.MusicBrainzReleaseID), nullStr(mItem.MusicBrainzRecordingID), mItem.DiscNumber, mItem.TrackNumber, mItem.ReleaseYear, refID)
		return err
	}
	if table == "tv_episodes" || table == "anime_episodes" {
		_, err := dbConn.ExecContext(ctx, `UPDATE `+table+` SET title = ?, path = ?, duration = ?, file_size_bytes = ?, file_mod_time = ?, file_hash = ?, file_hash_kind = ?, last_seen_at = ?, missing_since = NULL, match_status = ?, tmdb_id = ?, tvdb_id = ?, overview = ?, poster_path = ?, backdrop_path = ?, release_date = ?, vote_average = ?, imdb_id = ?, imdb_rating = ?, season = ?, episode = ?, metadata_review_needed = ?, metadata_confirmed = ? WHERE id = ?`,
			mItem.Title, mItem.Path, mItem.Duration, mItem.FileSizeBytes, nullStr(mItem.FileModTime), nullStr(mItem.FileHash), nullStr(mItem.FileHashKind), nullStr(seenAt), mItem.MatchStatus, mItem.TMDBID, nullStr(mItem.TVDBID), nullStr(mItem.Overview), nullStr(mItem.PosterPath), nullStr(mItem.BackdropPath), nullStr(mItem.ReleaseDate), nullFloat64(mItem.VoteAverage), nullStr(mItem.IMDbID), nullFloat64(mItem.IMDbRating), mItem.Season, mItem.Episode, mItem.MetadataReviewNeeded, mItem.MetadataConfirmed, refID)
		return err
	}
	_, err := dbConn.ExecContext(ctx, `UPDATE `+table+` SET title = ?, path = ?, duration = ?, file_size_bytes = ?, file_mod_time = ?, file_hash = ?, file_hash_kind = ?, last_seen_at = ?, missing_since = NULL, match_status = ?, tmdb_id = ?, tvdb_id = ?, overview = ?, poster_path = ?, backdrop_path = ?, release_date = ?, vote_average = ?, imdb_id = ?, imdb_rating = ? WHERE id = ?`,
		mItem.Title, mItem.Path, mItem.Duration, mItem.FileSizeBytes, nullStr(mItem.FileModTime), nullStr(mItem.FileHash), nullStr(mItem.FileHashKind), nullStr(seenAt), mItem.MatchStatus, mItem.TMDBID, nullStr(mItem.TVDBID), nullStr(mItem.Overview), nullStr(mItem.PosterPath), nullStr(mItem.BackdropPath), nullStr(mItem.ReleaseDate), nullFloat64(mItem.VoteAverage), nullStr(mItem.IMDbID), nullFloat64(mItem.IMDbRating), refID)
	return err
}

func updateMediaDuration(ctx context.Context, dbConn *sql.DB, table string, refID int, duration int) error {
	_, err := dbConn.ExecContext(ctx, `UPDATE `+table+` SET duration = ? WHERE id = ?`, duration, refID)
	return err
}

func pruneMissingMedia(ctx context.Context, dbConn *sql.DB, table, kind string, libraryID int, seenPaths map[string]struct{}) (int, error) {
	rows, err := dbConn.Query(`SELECT m.id, m.path, COALESCE(g.id, 0) FROM `+table+` m LEFT JOIN media_global g ON g.kind = ? AND g.ref_id = m.id WHERE m.library_id = ?`, kind, libraryID)
	if err != nil {
		return 0, err
	}
	type staleRow struct {
		refID    int
		globalID int
		path     string
	}
	var stale []staleRow
	for rows.Next() {
		var refID, globalID int
		var path string
		if err := rows.Scan(&refID, &path, &globalID); err != nil {
			rows.Close()
			return 0, err
		}
		if _, ok := seenPaths[path]; ok {
			continue
		}
		stale = append(stale, staleRow{refID: refID, globalID: globalID, path: path})
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	removed := 0
	for _, row := range stale {
		if row.globalID > 0 {
			if _, err := dbConn.ExecContext(ctx, `DELETE FROM subtitles WHERE media_id = ?`, row.globalID); err != nil {
				return removed, err
			}
			if _, err := dbConn.ExecContext(ctx, `DELETE FROM embedded_subtitles WHERE media_id = ?`, row.globalID); err != nil {
				return removed, err
			}
			if _, err := dbConn.ExecContext(ctx, `DELETE FROM embedded_audio_tracks WHERE media_id = ?`, row.globalID); err != nil {
				return removed, err
			}
			if _, err := dbConn.ExecContext(ctx, `DELETE FROM media_global WHERE id = ?`, row.globalID); err != nil {
				return removed, err
			}
		}
		if _, err := dbConn.ExecContext(ctx, `DELETE FROM `+table+` WHERE id = ?`, row.refID); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func parseInt(s string) (int, error) {
	return strconv.Atoi(s)
}

func hasExplicitProviderID(info metadata.MediaInfo) bool {
	return info.TMDBID > 0 || info.TVDBID != ""
}

func existingHasMetadata(kind string, row existingMediaRow) bool {
	if (kind == LibraryTypeTV || kind == LibraryTypeAnime) && row.MetadataConfirmed {
		return true
	}
	hasProviderID := row.TMDBID != 0
	if kind != LibraryTypeAnime {
		hasProviderID = hasProviderID || row.TVDBID != ""
	}
	return hasProviderID && row.IMDbID != ""
}

func nullFloat64(v float64) interface{} {
	if v == 0 {
		return nil
	}
	return v
}
