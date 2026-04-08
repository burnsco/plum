package db

import (
	"encoding/json"
	"fmt"
	"time"

	"plum/internal/metadata"
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

type Subtitle struct {
	ID              int    `json:"id"`
	MediaID         int    `json:"-"`
	Title           string `json:"title"`
	Language        string `json:"language"`
	Format          string `json:"format"`
	Forced          bool   `json:"forced,omitempty"`
	Default         bool   `json:"default,omitempty"`
	HearingImpaired bool   `json:"hearingImpaired,omitempty"`
	Path            string `json:"-"`
}

func SidecarSubtitleLogicalID(id int) string {
	return fmt.Sprintf("ext:%d", id)
}

func (s Subtitle) MarshalJSON() ([]byte, error) {
	type subtitleJSON struct {
		ID              int    `json:"id"`
		Title           string `json:"title"`
		Language        string `json:"language"`
		Format          string `json:"format"`
		LogicalID       string `json:"logicalId"`
		Forced          bool   `json:"forced,omitempty"`
		Default         bool   `json:"default,omitempty"`
		HearingImpaired bool   `json:"hearingImpaired,omitempty"`
	}
	return json.Marshal(subtitleJSON{
		ID:              s.ID,
		Title:           s.Title,
		Language:        s.Language,
		Format:          s.Format,
		LogicalID:       SidecarSubtitleLogicalID(s.ID),
		Forced:          s.Forced,
		Default:         s.Default,
		HearingImpaired: s.HearingImpaired,
	})
}

type EmbeddedSubtitle struct {
	MediaID         int    `json:"-"`
	StreamIndex     int    `json:"streamIndex"`
	Language        string `json:"language"`
	Title           string `json:"title"`
	Codec           string `json:"codec,omitempty"`
	Supported       *bool  `json:"supported,omitempty"`
	Forced          bool   `json:"forced,omitempty"`
	Default         bool   `json:"default,omitempty"`
	HearingImpaired bool   `json:"hearingImpaired,omitempty"`
}

func EmbeddedSubtitleLogicalID(streamIndex int) string {
	return fmt.Sprintf("emb:%d", streamIndex)
}

func (s EmbeddedSubtitle) MarshalJSON() ([]byte, error) {
	type embeddedSubtitleJSON struct {
		StreamIndex     int    `json:"streamIndex"`
		Language        string `json:"language"`
		Title           string `json:"title"`
		Codec           string `json:"codec,omitempty"`
		LogicalID       string `json:"logicalId"`
		Supported       *bool  `json:"supported,omitempty"`
		Forced          bool   `json:"forced,omitempty"`
		Default         bool   `json:"default,omitempty"`
		HearingImpaired bool   `json:"hearingImpaired,omitempty"`
	}
	return json.Marshal(embeddedSubtitleJSON{
		StreamIndex:     s.StreamIndex,
		Language:        s.Language,
		Title:           s.Title,
		Codec:           s.Codec,
		LogicalID:       EmbeddedSubtitleLogicalID(s.StreamIndex),
		Supported:       s.Supported,
		Forced:          s.Forced,
		Default:         s.Default,
		HearingImpaired: s.HearingImpaired,
	})
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
	// IntroLocked when true: automatic intro probes must not overwrite intro bounds.
	IntroLocked bool `json:"intro_locked,omitempty"`
	// CreditsStartSeconds/CreditsEndSeconds mark an end-credits window (manual or detector).
	CreditsStartSeconds *float64 `json:"credits_start_seconds,omitempty"`
	CreditsEndSeconds   *float64 `json:"credits_end_seconds,omitempty"`

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
