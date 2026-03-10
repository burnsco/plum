package metadata

import "context"

// Identifier is implemented by Pipeline and by test mocks. Used by the scanner to resolve metadata.
type Identifier interface {
	IdentifyTV(ctx context.Context, info MediaInfo) *MatchResult
	IdentifyMovie(ctx context.Context, info MediaInfo) *MatchResult
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
	Provider    string // e.g. "tmdb", "tvdb"
	ExternalID  string // provider-specific id (string supports both TMDB int and TVDB string)
}

// TVProvider searches for TV series and fetches episode details.
type TVProvider interface {
	SearchTV(ctx context.Context, query string) ([]MatchResult, error)
	GetEpisode(ctx context.Context, seriesID string, season, episode int) (*MatchResult, error)
}

// MovieProvider searches for movies.
type MovieProvider interface {
	SearchMovie(ctx context.Context, query string) ([]MatchResult, error)
}
