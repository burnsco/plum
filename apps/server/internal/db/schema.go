package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

func IsSQLiteBusy(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "database is locked") || strings.Contains(text, "sqlite_busy")
}

// RetryOnBusy retries fn up to maxAttempts when SQLite returns a busy/locked error,
// using exponential back-off starting at baseDelay.
func RetryOnBusy(ctx context.Context, maxAttempts int, baseDelay time.Duration, fn func() error) error {
	var err error
	for attempt := range maxAttempts {
		if err = fn(); err == nil || !IsSQLiteBusy(err) {
			return err
		}
		if ctx.Err() != nil {
			return err
		}
		delay := baseDelay * time.Duration(1<<uint(attempt))
		if delay > 10*time.Second {
			delay = 10 * time.Second
		}
		slog.Warn("sqlite busy, retrying", "attempt", attempt+1, "delay", delay)
		select {
		case <-ctx.Done():
			return err
		case <-time.After(delay):
		}
	}
	return err
}

// sqlitePragmas are applied to every new connection via the DSN so pool connections
// all have foreign_keys and busy_timeout set (connection-specific in SQLite).
//
// cache_size: negative value is a limit in KiB (here ~64 MiB page cache).
// mmap_size: bytes of DB file mapped read-only; improves cold reads on local disks (avoid huge values on network FS).
// After very large imports, running ANALYZE once can help the planner; Plum does not run it automatically.
const sqlitePragmas = "_pragma=foreign_keys(1)&_pragma=busy_timeout(30000)&_pragma=journal_mode(WAL)" +
	"&_pragma=cache_size(-65536)&_pragma=mmap_size(67108864)"

// plumSQLitePath stores the configured DB path (before DSN query params) for ancillary dirs (e.g. intro fingerprint cache).
var (
	plumSQLitePathMu sync.RWMutex
	plumSQLitePath   string
)

func plumAuxDBFilePath() string {
	plumSQLitePathMu.RLock()
	defer plumSQLitePathMu.RUnlock()
	return plumSQLitePath
}

func sqlitePathForAuxFiles(conn string) string {
	s := strings.TrimSpace(conn)
	if s == "" || s == ":memory:" {
		return ""
	}
	if strings.HasPrefix(s, "file:") {
		s = strings.TrimPrefix(s, "file:")
		if i := strings.IndexAny(s, "?"); i >= 0 {
			s = s[:i]
		}
		// file:///path or file:path
		s = strings.TrimPrefix(s, "//")
	}
	if i := strings.IndexAny(s, "?"); i >= 0 {
		s = s[:i]
	}
	return s
}

func InitDB(conn string) (*sql.DB, error) {
	if conn == "" {
		conn = "./data/plum.db"
	}
	plumSQLitePathMu.Lock()
	plumSQLitePath = sqlitePathForAuxFiles(conn)
	plumSQLitePathMu.Unlock()
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
  forced INTEGER NOT NULL DEFAULT 0,
  is_default INTEGER NOT NULL DEFAULT 0,
  hearing_impaired INTEGER NOT NULL DEFAULT 0,
  path TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_subtitles_media_id ON subtitles(media_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_subtitles_path ON subtitles(path);

CREATE TABLE IF NOT EXISTS embedded_subtitles (
  media_id INTEGER NOT NULL,
  stream_index INTEGER NOT NULL,
  language TEXT NOT NULL,
  title TEXT NOT NULL,
  forced INTEGER NOT NULL DEFAULT 0,
  is_default INTEGER NOT NULL DEFAULT 0,
  hearing_impaired INTEGER NOT NULL DEFAULT 0
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
  intro_probed_at TEXT,
  intro_locked INTEGER NOT NULL DEFAULT 0,
  credits_start_sec REAL,
  credits_end_sec REAL,
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
  hide_from_continue_watching INTEGER NOT NULL DEFAULT 0,
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
	{
		version: 31,
		name:    "media_files_intro_probed_at",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			return addColumnIfMissingTx(ctx, tx, "media_files", "intro_probed_at", "TEXT")
		},
	},
	{
		version: 32,
		name:    "media_files_intro_locked",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			return addColumnIfMissingTx(ctx, tx, "media_files", "intro_locked", "INTEGER NOT NULL DEFAULT 0")
		},
	},
	{
		version: 33,
		name:    "media_files_credits_bounds",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			if err := addColumnIfMissingTx(ctx, tx, "media_files", "credits_start_sec", "REAL"); err != nil {
				return err
			}
			return addColumnIfMissingTx(ctx, tx, "media_files", "credits_end_sec", "REAL")
		},
	},
	{
		version: 34,
		name:    "playback_progress_hide_from_continue_watching",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			return addColumnIfMissingTx(
				ctx,
				tx,
				"playback_progress",
				"hide_from_continue_watching",
				"INTEGER NOT NULL DEFAULT 0",
			)
		},
	},
	{
		version: 35,
		name:    "subtitle_metadata_flags",
		apply: func(ctx context.Context, tx *sql.Tx) error {
			for _, column := range []struct {
				table string
				name  string
				def   string
			}{
				{table: "subtitles", name: "forced", def: "INTEGER NOT NULL DEFAULT 0"},
				{table: "subtitles", name: "is_default", def: "INTEGER NOT NULL DEFAULT 0"},
				{table: "subtitles", name: "hearing_impaired", def: "INTEGER NOT NULL DEFAULT 0"},
				{table: "embedded_subtitles", name: "forced", def: "INTEGER NOT NULL DEFAULT 0"},
				{table: "embedded_subtitles", name: "is_default", def: "INTEGER NOT NULL DEFAULT 0"},
				{table: "embedded_subtitles", name: "hearing_impaired", def: "INTEGER NOT NULL DEFAULT 0"},
			} {
				if err := addColumnIfMissingTx(ctx, tx, column.table, column.name, column.def); err != nil {
					return err
				}
			}
			return nil
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
