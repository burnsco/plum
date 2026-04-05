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
	fanart                *FanartClient
	providerCache         ProviderCache
}

// NewPipeline builds a pipeline from API keys. Empty keys skip that provider.
func NewPipeline(tmdbKey, tvdbKey, omdbKey, fanartKey, musicBrainzContact string) *Pipeline {
	p := &Pipeline{
		omdb:          NewOMDBClient(omdbKey),
		fanart:        NewFanartClient(fanartKey),
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

// ReconfigureKeys rebuilds provider clients from API keys and reapplies cache and IMDb rating wiring.
// Safe for concurrent reads only in the sense that replacement is atomic for pointer fields; avoid
// calling during heavy metadata work on untrusted networks.
func (p *Pipeline) ReconfigureKeys(tmdbKey, tvdbKey, omdbKey, fanartKey, musicBrainzContact string) {
	next := NewPipeline(tmdbKey, tvdbKey, omdbKey, fanartKey, musicBrainzContact)
	next.SetIMDbRatingProvider(p.imdbRatings)
	next.SetProviderCache(p.providerCache)
	*p = *next
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
	if p.fanart != nil {
		p.fanart.SetCache(cache)
	}
}

func (p *Pipeline) IdentifyMusic(ctx context.Context, info MusicInfo) *MusicMatchResult {
	if p.musicProvider == nil {
		return nil
	}
	return p.musicProvider.IdentifyMusic(ctx, info)
}

func (p *Pipeline) SearchMovie(ctx context.Context, query string) ([]MatchResult, error) {
	if p.movieProvider == nil {
		return []MatchResult{}, nil
	}
	return p.movieProvider.SearchMovie(ctx, query)
}

func (p *Pipeline) GetMovie(ctx context.Context, movieID string) (*MatchResult, error) {
	lookup, ok := p.movieProvider.(MovieLookupProvider)
	if !ok || lookup == nil {
		return nil, nil
	}
	return lookup.GetMovie(ctx, movieID)
}

func (p *Pipeline) ProviderStatuses() []ArtworkProviderStatus {
	statuses := []ArtworkProviderStatus{
		{Provider: "fanart", Enabled: p.fanart != nil, Available: p.fanart != nil},
		{Provider: "tmdb", Enabled: p.tmdb != nil, Available: p.tmdb != nil},
		{Provider: "tvdb", Enabled: p.tvProviderByName("tvdb") != nil, Available: p.tvProviderByName("tvdb") != nil},
		{Provider: "omdb", Enabled: p.omdb != nil, Available: p.omdb != nil},
	}
	for index := range statuses {
		if statuses[index].Available {
			continue
		}
		switch statuses[index].Provider {
		case "fanart":
			statuses[index].Reason = "Missing FANART_API_KEY"
		case "tmdb":
			statuses[index].Reason = "Missing TMDB_API_KEY"
		case "tvdb":
			statuses[index].Reason = "Missing TVDB_API_KEY"
		case "omdb":
			statuses[index].Reason = "Missing OMDB_API_KEY"
		}
	}
	return statuses
}

func appendPosterCandidate(candidates []PosterCandidate, seen map[string]struct{}, provider string, imageURL string) []PosterCandidate {
	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" {
		return candidates
	}
	key := provider + ":" + imageURL
	if _, ok := seen[key]; ok {
		return candidates
	}
	seen[key] = struct{}{}
	label := strings.ToUpper(provider[:1]) + provider[1:]
	if provider == "tmdb" {
		label = "TMDB"
	} else if provider == "tvdb" {
		label = "TVDB"
	} else if provider == "omdb" {
		label = "OMDb"
	} else if provider == "fanart" {
		label = "Fanart"
	}
	return append(candidates, PosterCandidate{
		Provider:  provider,
		Label:     label,
		ImageURL:  imageURL,
		SourceURL: imageURL,
	})
}

func appendPosterCandidates(candidates []PosterCandidate, seen map[string]struct{}, provider string, imageURLs []string) []PosterCandidate {
	for _, imageURL := range imageURLs {
		candidates = appendPosterCandidate(candidates, seen, provider, imageURL)
	}
	return candidates
}

func (p *Pipeline) GetMoviePosterCandidates(ctx context.Context, tmdbID int, imdbID string) ([]PosterCandidate, error) {
	candidates := make([]PosterCandidate, 0, 8)
	seen := map[string]struct{}{}
	if p.fanart != nil && tmdbID > 0 {
		if posters, err := p.fanart.moviePosters(ctx, tmdbID); err == nil {
			candidates = appendPosterCandidates(candidates, seen, "fanart", posters)
		}
	}
	if p.movieDetailsProvider != nil && tmdbID > 0 {
		if details, err := p.movieDetailsProvider.GetMovieDetails(ctx, tmdbID); err == nil && details != nil {
			candidates = appendPosterCandidate(candidates, seen, "tmdb", details.PosterPath)
		}
	}
	if p.tmdb != nil && tmdbID > 0 {
		if posters, err := p.tmdb.getMoviePosters(ctx, tmdbID); err == nil {
			candidates = appendPosterCandidates(candidates, seen, "tmdb", posters)
		}
	}
	if p.omdb != nil && imdbID != "" {
		if poster, err := p.omdb.GetPosterByID(ctx, imdbID); err == nil {
			candidates = appendPosterCandidate(candidates, seen, "omdb", poster)
		}
	}
	return candidates, nil
}

func (p *Pipeline) GetShowPosterCandidates(ctx context.Context, title string, tmdbID int, tvdbID string) ([]PosterCandidate, error) {
	candidates := make([]PosterCandidate, 0, 8)
	seen := map[string]struct{}{}
	if p.seriesDetailsProvider != nil && tmdbID > 0 {
		if details, err := p.seriesDetailsProvider.GetSeriesDetails(ctx, tmdbID); err == nil && details != nil {
			candidates = appendPosterCandidate(candidates, seen, "tmdb", details.PosterPath)
			if tvdbID == "" && strings.TrimSpace(details.TVDBID) != "" {
				tvdbID = strings.TrimSpace(details.TVDBID)
			}
		}
	}
	if p.tmdb != nil && tmdbID > 0 {
		if posters, err := p.tmdb.getTVPosters(ctx, tmdbID); err == nil {
			candidates = appendPosterCandidates(candidates, seen, "tmdb", posters)
		}
	}
	if p.fanart != nil && tvdbID != "" {
		if posters, err := p.fanart.showPosters(ctx, tvdbID); err == nil {
			candidates = appendPosterCandidates(candidates, seen, "fanart", posters)
		}
	}
	if prov := p.tvProviderByName("tvdb"); prov != nil {
		if tvdbClient, ok := prov.(*TVDBClient); ok {
			if posters, err := tvdbClient.showPosters(ctx, title, tvdbID); err == nil {
				candidates = appendPosterCandidates(candidates, seen, "tvdb", posters)
			}
		}
	}
	return candidates, nil
}

func (p *Pipeline) GetSeasonPosterCandidates(ctx context.Context, title string, tmdbID int, tvdbID string, season int) ([]PosterCandidate, error) {
	candidates := make([]PosterCandidate, 0, 3)
	seen := map[string]struct{}{}
	if p.fanart != nil && tvdbID != "" {
		if poster, err := p.fanart.seasonPoster(ctx, tvdbID, season); err == nil {
			candidates = appendPosterCandidate(candidates, seen, "fanart", poster)
		}
	}
	if p.tmdb != nil && tmdbID > 0 {
		if poster, err := p.tmdb.seasonPoster(ctx, tmdbID, season); err == nil {
			candidates = appendPosterCandidate(candidates, seen, "tmdb", poster)
		}
	}
	if prov := p.tvProviderByName("tvdb"); prov != nil {
		if tvdbClient, ok := prov.(*TVDBClient); ok {
			if poster, err := tvdbClient.seasonPoster(ctx, title, tvdbID, season); err == nil {
				candidates = appendPosterCandidate(candidates, seen, "tvdb", poster)
			}
		}
	}
	return candidates, nil
}

func (p *Pipeline) GetEpisodePosterCandidates(ctx context.Context, title string, tmdbID int, tvdbID string, imdbID string, season int, episode int) ([]PosterCandidate, error) {
	candidates := make([]PosterCandidate, 0, 3)
	seen := map[string]struct{}{}
	if p.tmdb != nil && tmdbID > 0 {
		if match, err := p.tmdb.GetEpisode(ctx, strconv.Itoa(tmdbID), season, episode); err == nil && match != nil {
			candidates = appendPosterCandidate(candidates, seen, "tmdb", match.PosterURL)
		}
	}
	if prov := p.tvProviderByName("tvdb"); prov != nil && tvdbID != "" {
		if match, err := prov.GetEpisode(ctx, tvdbID, season, episode); err == nil && match != nil {
			candidates = appendPosterCandidate(candidates, seen, "tvdb", match.PosterURL)
		}
	}
	if p.omdb != nil && imdbID != "" {
		if poster, err := p.omdb.GetPosterByID(ctx, imdbID); err == nil {
			candidates = appendPosterCandidate(candidates, seen, "omdb", poster)
		}
	}
	return candidates, nil
}

func (p *Pipeline) GetDiscover(ctx context.Context) (*DiscoverResponse, error) {
	if p.discoverProvider == nil {
		return nil, ErrTMDBNotConfigured
	}
	return p.discoverProvider.GetDiscover(ctx)
}

func (p *Pipeline) GetDiscoverGenres(ctx context.Context) (*DiscoverGenresResponse, error) {
	if p.discoverProvider == nil {
		return nil, ErrTMDBNotConfigured
	}
	return p.discoverProvider.GetDiscoverGenres(ctx)
}

func (p *Pipeline) BrowseDiscover(
	ctx context.Context,
	category DiscoverBrowseCategory,
	mediaType DiscoverMediaType,
	genreID int,
	page int,
) (*DiscoverBrowseResponse, error) {
	if p.discoverProvider == nil {
		return nil, ErrTMDBNotConfigured
	}
	return p.discoverProvider.BrowseDiscover(ctx, category, mediaType, genreID, page)
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
	var missingReleaseYear []*MatchResult
	for i := range results {
		candidate := &results[i]
		if NormalizeTitle(candidate.Title) != infoTitle {
			continue
		}
		cy := parseYear(candidate.ReleaseDate)
		if cy == info.Year {
			return candidate
		}
		if cy == 0 {
			missingReleaseYear = append(missingReleaseYear, candidate)
		}
	}
	// TMDB search hits sometimes omit release_date; if exactly one exact-title row has no year,
	// prefer it when the library title includes a year (common for remakes like Crash 1996 vs 2004).
	if len(missingReleaseYear) == 1 {
		return missingReleaseYear[0]
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
		VoteAverage: details.VoteAverage,
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
		return prov.GetEpisode(ctx, seriesID, season, episode)
	}
	for _, prov := range p.tvProviders {
		ep, err := prov.GetEpisode(ctx, seriesID, season, episode)
		if err == nil && ep != nil {
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
