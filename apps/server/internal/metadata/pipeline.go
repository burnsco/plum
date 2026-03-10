package metadata

import (
	"context"
)

// Pipeline runs identification against multiple providers (TMDB, TVDB, etc.).
type Pipeline struct {
	movieProvider MovieProvider
	tvProviders   []TVProvider
}

// NewPipeline builds a pipeline from API keys. Empty keys skip that provider.
func NewPipeline(tmdbKey, tvdbKey string) *Pipeline {
	p := &Pipeline{}
	if tmdbKey != "" {
		tmdb := NewTMDBClient(tmdbKey)
		p.movieProvider = tmdb
		if len(p.tvProviders) == 0 {
			p.tvProviders = []TVProvider{tmdb}
		} else {
			p.tvProviders = append([]TVProvider{tmdb}, p.tvProviders...)
		}
	}
	if tvdbKey != "" {
		p.tvProviders = append(p.tvProviders, NewTVDBClient(tvdbKey, ""))
	}
	return p
}

// IdentifyMovie returns the first successful movie match from the movie provider (TMDB only in Phase 1).
func (p *Pipeline) IdentifyMovie(ctx context.Context, info MediaInfo) *MatchResult {
	if p.movieProvider == nil {
		return nil
	}
	results, err := p.movieProvider.SearchMovie(ctx, info.Title)
	if err != nil || len(results) == 0 {
		return nil
	}
	return &results[0]
}

// IdentifyTV returns the first successful TV match from the provider list (TMDB then TVDB).
func (p *Pipeline) IdentifyTV(ctx context.Context, info MediaInfo) *MatchResult {
	if len(p.tvProviders) == 0 {
		return nil
	}
	for _, prov := range p.tvProviders {
		results, err := prov.SearchTV(ctx, info.Title)
		if err != nil || len(results) == 0 {
			continue
		}
		series := results[0]
		if info.Season > 0 && info.Episode > 0 {
			ep, err := prov.GetEpisode(ctx, series.ExternalID, info.Season, info.Episode)
			if err == nil && ep != nil {
				return ep
			}
			// Fall back to series-level metadata
		}
		return &series
	}
	return nil
}
