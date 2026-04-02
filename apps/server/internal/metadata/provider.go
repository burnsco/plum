package metadata

import (
	"context"
	"errors"
	"strconv"
)

// Identifier is implemented by Pipeline and by test mocks. Used by the scanner to resolve metadata.
type Identifier interface {
	IdentifyTV(ctx context.Context, info MediaInfo) *MatchResult
	IdentifyAnime(ctx context.Context, info MediaInfo) *MatchResult
	IdentifyMovie(ctx context.Context, info MediaInfo) *MatchResult
}

// MovieIdentifierWithError exposes retryable provider failures for internal callers
// that need to distinguish "no result" from "provider temporarily failed".
type MovieIdentifierWithError interface {
	IdentifyMovieResult(ctx context.Context, info MediaInfo) (*MatchResult, error)
}

// MusicInfo is the provider-facing metadata used to identify a track.
type MusicInfo struct {
	Title       string
	Artist      string
	Album       string
	AlbumArtist string
	DiscNumber  int
	TrackNumber int
	ReleaseYear int
}

// MusicIdentifier resolves music metadata for a track.
type MusicIdentifier interface {
	IdentifyMusic(ctx context.Context, info MusicInfo) *MusicMatchResult
}

// MusicMatchResult is a provider-agnostic metadata result for a music track.
type MusicMatchResult struct {
	Title          string
	Artist         string
	Album          string
	AlbumArtist    string
	PosterURL      string
	ReleaseYear    int
	DiscNumber     int
	TrackNumber    int
	Provider       string
	RecordingID    string
	ReleaseID      string
	ReleaseGroupID string
	ArtistID       string
}

type CastMember struct {
	Name        string `json:"name"`
	Character   string `json:"character,omitempty"`
	Order       int    `json:"order,omitempty"`
	ProfilePath string `json:"profile_path,omitempty"`
	Provider    string `json:"provider,omitempty"`
	ProviderID  string `json:"provider_id,omitempty"`
}

// MatchResult is a provider-agnostic metadata result for a movie or TV episode.
// PosterURL and BackdropURL are full URLs so the pipeline owns URL shape.
type MatchResult struct {
	Title       string
	Overview    string
	PosterURL   string
	BackdropURL string
	ReleaseDate string
	VoteAverage float64
	IMDbID      string
	IMDbRating  float64
	Genres      []string
	Cast        []CastMember
	Runtime     int
	Provider    string // e.g. "tmdb", "tvdb"
	ExternalID  string // provider-specific id (string supports both TMDB int and TVDB string)
}

// TVProvider searches for TV series and fetches episode details.
type TVProvider interface {
	ProviderName() string
	SearchTV(ctx context.Context, query string) ([]MatchResult, error)
	GetEpisode(ctx context.Context, seriesID string, season, episode int) (*MatchResult, error)
}

// SeriesSearchProvider supports show search and episode lookup for manual and fallback identification flows.
type SeriesSearchProvider interface {
	SearchTV(ctx context.Context, query string) ([]MatchResult, error)
	GetEpisode(ctx context.Context, provider, seriesID string, season, episode int) (*MatchResult, error)
}

// MovieProvider searches for movies.
type MovieProvider interface {
	SearchMovie(ctx context.Context, query string) ([]MatchResult, error)
}

// MovieLookupProvider can resolve a movie directly by provider ID.
type MovieLookupProvider interface {
	GetMovie(ctx context.Context, movieID string) (*MatchResult, error)
}

type MovieDetails struct {
	Title        string       `json:"title"`
	Overview     string       `json:"overview"`
	PosterPath   string       `json:"poster_path"`
	BackdropPath string       `json:"backdrop_path"`
	ReleaseDate  string       `json:"release_date"`
	IMDbID       string       `json:"imdb_id,omitempty"`
	IMDbRating   float64      `json:"imdb_rating,omitempty"`
	Genres       []string     `json:"genres"`
	Cast         []CastMember `json:"cast"`
	Runtime      int          `json:"runtime,omitempty"`
}

type MovieDetailsProvider interface {
	GetMovieDetails(ctx context.Context, tmdbID int) (*MovieDetails, error)
}

// SeriesDetails is minimal TV series info for the show-detail UI.
type SeriesDetails struct {
	Name             string       `json:"name"`
	Overview         string       `json:"overview"`
	PosterPath       string       `json:"poster_path"`   // full URL or path
	BackdropPath     string       `json:"backdrop_path"` // full URL or path
	FirstAirDate     string       `json:"first_air_date"`
	VoteAverage      float64      `json:"vote_average,omitempty"`
	IMDbID           string       `json:"imdb_id,omitempty"`
	IMDbRating       float64      `json:"imdb_rating,omitempty"`
	Genres           []string     `json:"genres"`
	Cast             []CastMember `json:"cast"`
	Runtime          int          `json:"runtime,omitempty"`
	NumberOfSeasons  int          `json:"number_of_seasons,omitempty"`
	NumberOfEpisodes int          `json:"number_of_episodes,omitempty"`
	TVDBID           string       `json:"-"`
}

// SeriesDetailsProvider fetches TV series metadata by TMDB ID.
type SeriesDetailsProvider interface {
	GetSeriesDetails(ctx context.Context, tmdbID int) (*SeriesDetails, error)
}

type DiscoverMediaType string

const (
	DiscoverMediaTypeMovie DiscoverMediaType = "movie"
	DiscoverMediaTypeTV    DiscoverMediaType = "tv"
)

type DiscoverBrowseCategory string

const (
	DiscoverBrowseCategoryTrending      DiscoverBrowseCategory = "trending"
	DiscoverBrowseCategoryPopularMovies DiscoverBrowseCategory = "popular-movies"
	DiscoverBrowseCategoryPopularTV     DiscoverBrowseCategory = "popular-tv"
	DiscoverBrowseCategoryNowPlaying    DiscoverBrowseCategory = "now-playing"
	DiscoverBrowseCategoryUpcoming      DiscoverBrowseCategory = "upcoming"
	DiscoverBrowseCategoryOnTheAir      DiscoverBrowseCategory = "on-the-air"
	DiscoverBrowseCategoryTopRated      DiscoverBrowseCategory = "top-rated"
)

type MediaStackServiceKind string

const (
	MediaStackServiceRadarr   MediaStackServiceKind = "radarr"
	MediaStackServiceSonarrTV MediaStackServiceKind = "sonarr-tv"
)

type DiscoverAcquisitionState string

const (
	DiscoverAcquisitionStateNotAdded    DiscoverAcquisitionState = "not_added"
	DiscoverAcquisitionStateAdded       DiscoverAcquisitionState = "added"
	DiscoverAcquisitionStateDownloading DiscoverAcquisitionState = "downloading"
	DiscoverAcquisitionStateAvailable   DiscoverAcquisitionState = "available"
)

var ErrTMDBNotConfigured = errors.New("tmdb discover requires TMDB_API_KEY")

// ProviderError describes a provider HTTP or transport failure.
type ProviderError struct {
	Provider   string
	StatusCode int
	Retryable  bool
	Cause      error
}

func (e *ProviderError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return e.Provider + ": " + e.Cause.Error()
	}
	if e.StatusCode > 0 {
		return e.Provider + ": status " + strconv.Itoa(e.StatusCode)
	}
	return e.Provider + ": provider request failed"
}

func (e *ProviderError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func IsRetryableProviderError(err error) bool {
	var providerErr *ProviderError
	return errors.As(err, &providerErr) && providerErr.Retryable
}

type DiscoverLibraryMatch struct {
	LibraryID   int    `json:"library_id"`
	LibraryName string `json:"library_name"`
	LibraryType string `json:"library_type"`
	Kind        string `json:"kind"`
	ShowKey     string `json:"show_key,omitempty"`
}

type DiscoverAcquisition struct {
	State        DiscoverAcquisitionState `json:"state"`
	Source       MediaStackServiceKind    `json:"source,omitempty"`
	CanAdd       bool                     `json:"can_add,omitempty"`
	IsConfigured bool                     `json:"is_configured,omitempty"`
}

type DiscoverItem struct {
	MediaType      DiscoverMediaType      `json:"media_type"`
	TMDBID         int                    `json:"tmdb_id"`
	Title          string                 `json:"title"`
	Overview       string                 `json:"overview,omitempty"`
	PosterPath     string                 `json:"poster_path,omitempty"`
	BackdropPath   string                 `json:"backdrop_path,omitempty"`
	ReleaseDate    string                 `json:"release_date,omitempty"`
	FirstAirDate   string                 `json:"first_air_date,omitempty"`
	VoteAverage    float64                `json:"vote_average,omitempty"`
	LibraryMatches []DiscoverLibraryMatch `json:"library_matches,omitempty"`
	Acquisition    *DiscoverAcquisition   `json:"acquisition,omitempty"`
}

type DiscoverShelf struct {
	ID    string         `json:"id"`
	Title string         `json:"title"`
	Items []DiscoverItem `json:"items"`
}

type DiscoverResponse struct {
	Shelves []DiscoverShelf `json:"shelves"`
}

type DiscoverSearchResponse struct {
	Movies []DiscoverItem `json:"movies"`
	TV     []DiscoverItem `json:"tv"`
}

type DiscoverGenre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type DiscoverGenresResponse struct {
	MovieGenres []DiscoverGenre `json:"movie_genres"`
	TVGenres    []DiscoverGenre `json:"tv_genres"`
}

type DiscoverBrowseResponse struct {
	Items        []DiscoverItem        `json:"items"`
	Page         int                   `json:"page"`
	TotalPages   int                   `json:"total_pages"`
	TotalResults int                   `json:"total_results"`
	MediaType    DiscoverMediaType     `json:"media_type,omitempty"`
	Genre        *DiscoverGenre        `json:"genre,omitempty"`
	Category     DiscoverBrowseCategory `json:"category,omitempty"`
}

type DiscoverTitleVideo struct {
	Name     string `json:"name"`
	Site     string `json:"site"`
	Key      string `json:"key"`
	Type     string `json:"type"`
	Official bool   `json:"official,omitempty"`
}

type DiscoverTitleDetails struct {
	MediaType        DiscoverMediaType      `json:"media_type"`
	TMDBID           int                    `json:"tmdb_id"`
	Title            string                 `json:"title"`
	Overview         string                 `json:"overview"`
	PosterPath       string                 `json:"poster_path,omitempty"`
	BackdropPath     string                 `json:"backdrop_path,omitempty"`
	ReleaseDate      string                 `json:"release_date,omitempty"`
	FirstAirDate     string                 `json:"first_air_date,omitempty"`
	VoteAverage      float64                `json:"vote_average,omitempty"`
	IMDbID           string                 `json:"imdb_id,omitempty"`
	IMDbRating       float64                `json:"imdb_rating,omitempty"`
	Status           string                 `json:"status,omitempty"`
	Genres           []string               `json:"genres"`
	Runtime          int                    `json:"runtime,omitempty"`
	NumberOfSeasons  int                    `json:"number_of_seasons,omitempty"`
	NumberOfEpisodes int                    `json:"number_of_episodes,omitempty"`
	Videos           []DiscoverTitleVideo   `json:"videos"`
	LibraryMatches   []DiscoverLibraryMatch `json:"library_matches,omitempty"`
	Acquisition      *DiscoverAcquisition   `json:"acquisition,omitempty"`
}

type DiscoverProvider interface {
	GetDiscover(ctx context.Context) (*DiscoverResponse, error)
	GetDiscoverGenres(ctx context.Context) (*DiscoverGenresResponse, error)
	BrowseDiscover(ctx context.Context, category DiscoverBrowseCategory, mediaType DiscoverMediaType, genreID int, page int) (*DiscoverBrowseResponse, error)
	SearchDiscover(ctx context.Context, query string) (*DiscoverSearchResponse, error)
	GetDiscoverTitleDetails(ctx context.Context, mediaType DiscoverMediaType, tmdbID int) (*DiscoverTitleDetails, error)
}

// IMDbRatingProvider resolves an IMDb rating by IMDb title id.
type IMDbRatingProvider interface {
	GetIMDbRatingByID(ctx context.Context, imdbID string) (float64, error)
}

type ArtworkProviderStatus struct {
	Provider  string `json:"provider"`
	Enabled   bool   `json:"enabled"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

type PosterCandidate struct {
	Provider  string `json:"provider"`
	Label     string `json:"label"`
	ImageURL  string `json:"image_url"`
	SourceURL string `json:"source_url"`
}

type MetadataArtworkProvider interface {
	ProviderStatuses() []ArtworkProviderStatus
	GetMoviePosterCandidates(ctx context.Context, tmdbID int, imdbID string) ([]PosterCandidate, error)
	GetShowPosterCandidates(ctx context.Context, title string, tmdbID int, tvdbID string) ([]PosterCandidate, error)
	GetSeasonPosterCandidates(ctx context.Context, title string, tmdbID int, tvdbID string, season int) ([]PosterCandidate, error)
	GetEpisodePosterCandidates(ctx context.Context, title string, tmdbID int, tvdbID string, imdbID string, season int, episode int) ([]PosterCandidate, error)
}
