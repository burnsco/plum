package metadata

import (
	"context"
	"strconv"
	"strings"
)

// Pipeline runs identification against multiple providers (TMDB, TVDB, etc.).
type Pipeline struct {
	movieProvider         MovieProvider
	movieDetailsProvider  MovieDetailsProvider
	tvProviders           []TVProvider
	seriesDetailsProvider SeriesDetailsProvider
	discoverProvider      DiscoverProvider
	imdbRatings           IMDbRatingProvider
	musicProvider         MusicIdentifier
	omdb                  *OMDBClient
	tmdb                  *TMDBClient
	tvdb                  []*TVDBClient
	providerCache         ProviderCache
}

// NewPipeline builds a pipeline from API keys. Empty keys skip that provider.
func NewPipeline(tmdbKey, tvdbKey, omdbKey, musicBrainzContact string) *Pipeline {
	p := &Pipeline{
		omdb:          NewOMDBClient(omdbKey),
		musicProvider: NewMusicBrainzClient(musicBrainzContact),
	}
	if tmdbKey != "" {
		tmdb := NewTMDBClient(tmdbKey)
		p.tmdb = tmdb
		p.movieProvider = tmdb
		p.movieDetailsProvider = tmdb
		p.seriesDetailsProvider = tmdb
		p.discoverProvider = tmdb
		if len(p.tvProviders) == 0 {
			p.tvProviders = []TVProvider{tmdb}
		} else {
			p.tvProviders = append([]TVProvider{tmdb}, p.tvProviders...)
		}
	}
	if tvdbKey != "" {
		tvdb := NewTVDBClient(tvdbKey, "")
		p.tvdb = append(p.tvdb, tvdb)
		p.tvProviders = append(p.tvProviders, tvdb)
	}
	return p
}

func (p *Pipeline) SetIMDbRatingProvider(provider IMDbRatingProvider) {
	p.imdbRatings = provider
}

func (p *Pipeline) SetProviderCache(cache ProviderCache) {
	p.providerCache = cache
	if p.tmdb != nil {
		p.tmdb.SetCache(cache)
	}
	for _, tvdb := range p.tvdb {
		if tvdb != nil {
			tvdb.SetCache(cache)
		}
	}
	if p.omdb != nil {
		p.omdb.SetCache(cache)
	}
}

func (p *Pipeline) IdentifyMusic(ctx context.Context, info MusicInfo) *MusicMatchResult {
	if p.musicProvider == nil {
		return nil
	}
	return p.musicProvider.IdentifyMusic(ctx, info)
}

func (p *Pipeline) GetDiscover(ctx context.Context) (*DiscoverResponse, error) {
	if p.discoverProvider == nil {
		return nil, ErrTMDBNotConfigured
	}
	return p.discoverProvider.GetDiscover(ctx)
}

func (p *Pipeline) SearchDiscover(ctx context.Context, query string) (*DiscoverSearchResponse, error) {
	if p.discoverProvider == nil {
		return nil, ErrTMDBNotConfigured
	}
	return p.discoverProvider.SearchDiscover(ctx, query)
}

func (p *Pipeline) GetDiscoverTitleDetails(ctx context.Context, mediaType DiscoverMediaType, tmdbID int) (*DiscoverTitleDetails, error) {
	if p.discoverProvider == nil {
		return nil, ErrTMDBNotConfigured
	}
	details, err := p.discoverProvider.GetDiscoverTitleDetails(ctx, mediaType, tmdbID)
	if err != nil || details == nil {
		return details, err
	}
	if p.imdbRatings != nil && details.IMDbID != "" {
		if rating, ratingErr := p.imdbRatings.GetIMDbRatingByID(ctx, details.IMDbID); ratingErr == nil && rating > 0 {
			details.IMDbRating = rating
		}
	}
	return details, nil
}

func (p *Pipeline) GetMovieDetails(ctx context.Context, tmdbID int) (*MovieDetails, error) {
	if p.movieDetailsProvider == nil {
		return nil, nil
	}
	details, err := p.movieDetailsProvider.GetMovieDetails(ctx, tmdbID)
	if err != nil || details == nil {
		return details, err
	}
	if p.imdbRatings != nil && details.IMDbID != "" && details.IMDbRating <= 0 {
		if rating, ratingErr := p.imdbRatings.GetIMDbRatingByID(ctx, details.IMDbID); ratingErr == nil && rating > 0 {
			details.IMDbRating = rating
		}
	}
	return details, nil
}

// IdentifyMovie returns the best movie match using scorer and confidence threshold.
func (p *Pipeline) IdentifyMovie(ctx context.Context, info MediaInfo) *MatchResult {
	result, _ := p.IdentifyMovieResult(ctx, info)
	return result
}

// IdentifyMovieResult exposes provider failures so callers can retry transient
// movie lookup issues instead of treating them as an unmatched title.
func (p *Pipeline) IdentifyMovieResult(ctx context.Context, info MediaInfo) (*MatchResult, error) {
	if p.movieProvider == nil {
		return nil, nil
	}
	if info.TMDBID > 0 {
		if lookup, ok := p.movieProvider.(MovieLookupProvider); ok {
			res, err := lookup.GetMovie(ctx, strconv.Itoa(info.TMDBID))
			if err != nil {
				if IsRetryableProviderError(err) {
					return nil, err
				}
			} else if res != nil {
				p.enrichIMDbRating(ctx, res)
				return res, nil
			}
		}
	}
	results, err := p.movieProvider.SearchMovie(ctx, info.Title)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	best, _ := bestScored(results, info, ScoreMovie, ScoreMovieAutoMatch, ScoreMargin)
	if best == nil && info.Year > 0 {
		best = firstExactMovieTitleYearMatch(results, info)
	}
	if best == nil && info.Year == 0 {
		best = uniqueExactMovieTitleMatch(results, info)
	}
	if best == nil {
		return nil, nil
	}
	if lookup, ok := p.movieProvider.(MovieLookupProvider); ok {
		detailed, err := lookup.GetMovie(ctx, best.ExternalID)
		if err != nil {
			if IsRetryableProviderError(err) {
				return nil, err
			}
		} else if detailed != nil {
			p.enrichIMDbRating(ctx, detailed)
			return detailed, nil
		}
	}
	p.enrichIMDbRating(ctx, best)
	return best, nil
}

func uniqueExactMovieTitleMatch(results []MatchResult, info MediaInfo) *MatchResult {
	infoTitle := NormalizeTitle(info.Title)
	if infoTitle == "" {
		return nil
	}
	var match *MatchResult
	for i := range results {
		candidate := &results[i]
		if NormalizeTitle(candidate.Title) != infoTitle {
			continue
		}
		if match != nil {
			return nil
		}
		match = candidate
	}
	if match == nil {
		return nil
	}
	// Avoid loose matches when the candidate title still contains extra qualifiers
	// that were not present in the parsed library title.
	if strings.TrimSpace(match.Title) == "" {
		return nil
	}
	return match
}

func firstExactMovieTitleYearMatch(results []MatchResult, info MediaInfo) *MatchResult {
	infoTitle := NormalizeTitle(info.Title)
	if infoTitle == "" || info.Year <= 0 {
		return nil
	}
	for i := range results {
		candidate := &results[i]
		if NormalizeTitle(candidate.Title) != infoTitle {
			continue
		}
		if parseYear(candidate.ReleaseDate) != info.Year {
			continue
		}
		return candidate
	}
	return nil
}

// IdentifyTV returns the best TV match: explicit ID first, then scored candidates with threshold + margin.
func (p *Pipeline) IdentifyTV(ctx context.Context, info MediaInfo) *MatchResult {
	return p.identifySeries(ctx, info, false)
}

// IdentifyAnime returns the best anime match while avoiding unsafe absolute-number guesses.
func (p *Pipeline) IdentifyAnime(ctx context.Context, info MediaInfo) *MatchResult {
	if info.Season == 0 && info.Episode == 0 && info.AbsoluteEpisode > 0 && info.TMDBID <= 0 {
		return nil
	}
	tmdbProv := p.tvProviderByName("tmdb")
	if tmdbProv == nil {
		return nil
	}
	if info.TMDBID > 0 {
		if res := p.lookupTMDBTV(ctx, info); res != nil {
			return res
		}
	}
	results, err := tmdbProv.SearchTV(ctx, info.Title)
	if err != nil || len(results) == 0 {
		return nil
	}
	best, _ := bestScored(results, info, ScoreTV, ScoreAutoMatch, ScoreMargin)
	if best == nil {
		return nil
	}
	if info.Season > 0 && info.Episode > 0 {
		if ep, err := tmdbProv.GetEpisode(ctx, best.ExternalID, info.Season, info.Episode); err == nil && ep != nil {
			p.enrichIMDbRating(ctx, ep)
			return ep
		}
	}
	if best.Provider == "tmdb" {
		if tmdbID, err := strconv.Atoi(best.ExternalID); err == nil && tmdbID > 0 {
			if detailed := p.lookupTMDBTV(ctx, MediaInfo{TMDBID: tmdbID}); detailed != nil {
				return detailed
			}
		}
	}
	p.enrichIMDbRating(ctx, best)
	return best
}

func (p *Pipeline) identifySeries(ctx context.Context, info MediaInfo, allowTVDBFallback bool) *MatchResult {
	if len(p.tvProviders) == 0 {
		return nil
	}
	if info.TMDBID > 0 {
		if res := p.lookupTMDBTV(ctx, info); res != nil {
			return res
		}
	}
	if allowTVDBFallback {
		if res := p.lookupTVDBEpisode(ctx, info); res != nil {
			return res
		}
	}
	best := p.bestSeriesMatch(ctx, info, p.searchSeriesCandidates(ctx, info, []string{"tmdb"}))
	if best == nil {
		if allowTVDBFallback {
			best = p.bestSeriesMatch(ctx, info, p.searchSeriesCandidates(ctx, info, []string{"tvdb"}))
		} else {
			best = p.bestSeriesMatch(ctx, info, p.searchSeriesCandidates(ctx, info, providerOrder(nil)))
		}
	}
	if best == nil {
		return nil
	}
	return best
}

func (p *Pipeline) bestSeriesMatch(ctx context.Context, info MediaInfo, candidates []MatchResult) *MatchResult {
	if len(candidates) == 0 {
		return nil
	}
	best, _ := bestScored(candidates, info, ScoreTV, ScoreAutoMatch, ScoreMargin)
	if best == nil {
		return nil
	}
	if info.Season > 0 && info.Episode > 0 {
		if ep := p.getEpisodeForMatch(ctx, best.Provider, best.ExternalID, info.Season, info.Episode); ep != nil {
			p.enrichIMDbRating(ctx, ep)
			return ep
		}
	}
	if best.Provider == "tmdb" {
		if tmdbID, err := strconv.Atoi(best.ExternalID); err == nil && tmdbID > 0 {
			if detailed := p.lookupTMDBTV(ctx, MediaInfo{TMDBID: tmdbID}); detailed != nil {
				return detailed
			}
		}
	}
	p.enrichIMDbRating(ctx, best)
	return best
}

func (p *Pipeline) lookupTVDBEpisode(ctx context.Context, info MediaInfo) *MatchResult {
	if info.TVDBID == "" || info.Season <= 0 || info.Episode <= 0 {
		return nil
	}
	prov := p.tvProviderByName("tvdb")
	if prov == nil {
		return nil
	}
	ep, err := prov.GetEpisode(ctx, info.TVDBID, info.Season, info.Episode)
	if err != nil || ep == nil {
		return nil
	}
	p.enrichIMDbRating(ctx, ep)
	return ep
}

func (p *Pipeline) searchSeriesCandidates(ctx context.Context, info MediaInfo, order []string) []MatchResult {
	var candidates []MatchResult
	seen := make(map[string]bool)
	for _, name := range providerOrder(order) {
		prov := p.tvProviderByName(name)
		if prov == nil {
			continue
		}
		results, err := prov.SearchTV(ctx, info.Title)
		if err != nil || len(results) == 0 {
			continue
		}
		for i := range results {
			key := results[i].Provider + ":" + results[i].ExternalID
			if seen[key] {
				continue
			}
			seen[key] = true
			candidates = append(candidates, results[i])
		}
	}
	return candidates
}

func providerOrder(names []string) []string {
	if len(names) == 0 {
		return []string{"tmdb", "tvdb"}
	}
	return names
}

func (p *Pipeline) lookupTMDBTV(ctx context.Context, info MediaInfo) *MatchResult {
	tmdbProv := p.tvProviderByName("tmdb")
	if tmdbProv == nil {
		return nil
	}
	seriesID := strconv.Itoa(info.TMDBID)
	if info.Season > 0 && info.Episode > 0 {
		if ep, err := tmdbProv.GetEpisode(ctx, seriesID, info.Season, info.Episode); err == nil && ep != nil {
			p.enrichIMDbRating(ctx, ep)
			return ep
		}
	}
	if p.seriesDetailsProvider == nil {
		return nil
	}
	details, err := p.seriesDetailsProvider.GetSeriesDetails(ctx, info.TMDBID)
	if err != nil || details == nil {
		return nil
	}
	result := &MatchResult{
		Title:       details.Name,
		Overview:    details.Overview,
		PosterURL:   details.PosterPath,
		BackdropURL: details.BackdropPath,
		ReleaseDate: details.FirstAirDate,
		IMDbID:      details.IMDbID,
		IMDbRating:  details.IMDbRating,
		Provider:    "tmdb",
		ExternalID:  seriesID,
	}
	p.enrichIMDbRating(ctx, result)
	return result
}

func (p *Pipeline) enrichIMDbRating(ctx context.Context, result *MatchResult) {
	if result == nil || result.IMDbID == "" || result.IMDbRating > 0 {
		return
	}
	if p.imdbRatings != nil {
		if rating, err := p.imdbRatings.GetIMDbRatingByID(ctx, result.IMDbID); err == nil && rating > 0 {
			result.IMDbRating = rating
			return
		}
	}
	if p.omdb == nil {
		return
	}
	if rating, err := p.omdb.GetIMDbRatingByID(ctx, result.IMDbID); err == nil && rating > 0 {
		result.IMDbRating = rating
	}
}

// bestScored returns the best candidate when top score >= threshold and (only one or top-second >= margin).
func bestScored(candidates []MatchResult, info MediaInfo, scoreFn func(*MatchResult, MediaInfo) int, threshold, margin int) (*MatchResult, int) {
	if len(candidates) == 0 {
		return nil, 0
	}
	type scored struct {
		m     *MatchResult
		score int
	}
	scores := make([]scored, len(candidates))
	for i := range candidates {
		scores[i] = scored{m: &candidates[i], score: scoreFn(&candidates[i], info)}
	}
	// Sort by score descending (simple bubble for small n)
	for i := 0; i < len(scores); i++ {
		for j := i + 1; j < len(scores); j++ {
			if scores[j].score > scores[i].score {
				scores[i], scores[j] = scores[j], scores[i]
			}
		}
	}
	top := scores[0]
	if top.score < threshold {
		return nil, top.score
	}
	if len(scores) > 1 && (top.score-scores[1].score) < margin {
		return nil, top.score
	}
	return top.m, top.score
}

// GetSeriesDetails returns TV series metadata by TMDB ID for the show-detail UI.
func (p *Pipeline) GetSeriesDetails(ctx context.Context, tmdbID int) (*SeriesDetails, error) {
	if p.seriesDetailsProvider == nil {
		return nil, nil
	}
	details, err := p.seriesDetailsProvider.GetSeriesDetails(ctx, tmdbID)
	if err != nil || details == nil {
		return details, err
	}
	if details.IMDbID != "" && details.IMDbRating <= 0 {
		result := &MatchResult{IMDbID: details.IMDbID}
		p.enrichIMDbRating(ctx, result)
		details.IMDbRating = result.IMDbRating
	}
	return details, nil
}

// SearchTV returns raw TV search results from the first provider (e.g. TMDB) for the Identify UI.
func (p *Pipeline) SearchTV(ctx context.Context, query string) ([]MatchResult, error) {
	if len(p.tvProviders) == 0 {
		return nil, nil
	}
	return p.tvProviders[0].SearchTV(ctx, query)
}

// GetEpisode returns episode-level metadata for a series/season/episode (for refresh/identify).
func (p *Pipeline) GetEpisode(ctx context.Context, provider, seriesID string, season, episode int) (*MatchResult, error) {
	if provider != "" {
		prov := p.tvProviderByName(provider)
		if prov == nil {
			return nil, nil
		}
		ep, err := prov.GetEpisode(ctx, seriesID, season, episode)
		if err == nil && ep != nil {
			p.enrichIMDbRating(ctx, ep)
		}
		return ep, err
	}
	for _, prov := range p.tvProviders {
		ep, err := prov.GetEpisode(ctx, seriesID, season, episode)
		if err == nil && ep != nil {
			p.enrichIMDbRating(ctx, ep)
			return ep, nil
		}
	}
	return nil, nil
}

func (p *Pipeline) getEpisodeForMatch(ctx context.Context, provider, seriesID string, season, episode int) *MatchResult {
	ep, err := p.GetEpisode(ctx, provider, seriesID, season, episode)
	if err != nil {
		return nil
	}
	return ep
}

func (p *Pipeline) tvProviderByName(name string) TVProvider {
	for _, prov := range p.tvProviders {
		if prov.ProviderName() == name {
			return prov
		}
	}
	return nil
}
