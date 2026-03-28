package metadata

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func tvInfoFromPath(relPath, filename string) MediaInfo {
	fileInfo := ParseFilename(filename)
	pathInfo := ParsePathForTV(relPath, filename)
	return MergePathInfo(pathInfo, fileInfo)
}

type stubTVProvider struct {
	name          string
	searchResults []MatchResult
	episodeResult *MatchResult
	episodeCalls  []string
}

func (s *stubTVProvider) ProviderName() string {
	return s.name
}

func (s *stubTVProvider) SearchTV(_ context.Context, _ string) ([]MatchResult, error) {
	return s.searchResults, nil
}

func (s *stubTVProvider) GetEpisode(_ context.Context, seriesID string, season, episode int) (*MatchResult, error) {
	s.episodeCalls = append(s.episodeCalls, seriesID)
	if s.episodeResult == nil {
		return nil, nil
	}
	return s.episodeResult, nil
}

type stubMovieProvider struct {
	searchResults []MatchResult
	searchErr     error
	lookupResult  *MatchResult
	lookupErr     error
	lookupCalls   []string
}

func (s *stubMovieProvider) SearchMovie(_ context.Context, _ string) ([]MatchResult, error) {
	return s.searchResults, s.searchErr
}

func (s *stubMovieProvider) GetMovie(_ context.Context, movieID string) (*MatchResult, error) {
	s.lookupCalls = append(s.lookupCalls, movieID)
	if s.lookupErr != nil {
		return nil, s.lookupErr
	}
	return s.lookupResult, nil
}

type stubSeriesDetailsProvider struct {
	details *SeriesDetails
}

func (s *stubSeriesDetailsProvider) GetSeriesDetails(_ context.Context, _ int) (*SeriesDetails, error) {
	return s.details, nil
}

type stubIMDbRatingProvider struct {
	rating float64
}

func (s *stubIMDbRatingProvider) GetIMDbRatingByID(_ context.Context, _ string) (float64, error) {
	return s.rating, nil
}

type rewriteTransport struct {
	base *url.URL
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.URL.Scheme = t.base.Scheme
	cloned.URL.Host = t.base.Host
	cloned.Host = t.base.Host
	return http.DefaultTransport.RoundTrip(cloned)
}

func TestIdentifyTV_ExplicitTMDBIDUsesEpisodeMetadata(t *testing.T) {
	tmdb := &stubTVProvider{
		name: "tmdb",
		episodeResult: &MatchResult{
			Title:      "Show - S01E02 - Episode",
			Provider:   "tmdb",
			ExternalID: "123",
		},
	}
	p := &Pipeline{
		tvProviders:           []TVProvider{tmdb},
		seriesDetailsProvider: &stubSeriesDetailsProvider{details: &SeriesDetails{Name: "Show"}},
	}

	res := p.IdentifyTV(context.Background(), MediaInfo{TMDBID: 123, Season: 1, Episode: 2})
	if res == nil {
		t.Fatal("expected match")
	}
	if res.Title != "Show - S01E02 - Episode" {
		t.Fatalf("title = %q", res.Title)
	}
	if len(tmdb.episodeCalls) != 1 || tmdb.episodeCalls[0] != "123" {
		t.Fatalf("episode lookup calls = %#v", tmdb.episodeCalls)
	}
}

func TestGetShowPosterCandidates_UsesEnrichedTVDBIDForFanart(t *testing.T) {
	var fanartRequested bool
	var tvdbRequested bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v4/login":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"data":{"token":"token"}}`)
		case "/v4/series/123/episodes/default":
			tvdbRequested = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{
				"data": {
					"series": {"id": 123, "name": "Show", "image": "/series.jpg", "firstAired": "2024-01-01"},
					"episodes": [
						{"id": 1, "name": "Pilot", "overview": "Pilot", "aired": "2024-01-01", "image": "/episode.jpg", "seasonNumber": 1, "number": 1}
					]
				}
			}`)
		case "/v3/tv/123":
			fanartRequested = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"tvposter":[{"url":"https://fanart.example/poster.jpg"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	baseURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	oldClient := providerHTTPClient
	providerHTTPClient = &http.Client{Transport: &rewriteTransport{base: baseURL}}
	t.Cleanup(func() {
		providerHTTPClient = oldClient
	})

	p := &Pipeline{
		seriesDetailsProvider: &stubSeriesDetailsProvider{
			details: &SeriesDetails{TVDBID: "123"},
		},
		tvProviders: []TVProvider{&TVDBClient{APIKey: "tvdb"}},
		fanart:      &FanartClient{APIKey: "fanart"},
	}

	candidates, err := p.GetShowPosterCandidates(context.Background(), "Show", 42, "")
	if err != nil {
		t.Fatalf("GetShowPosterCandidates returned error: %v", err)
	}
	if !fanartRequested {
		t.Fatal("expected fanart lookup to run after TVDB ID enrichment")
	}
	if !tvdbRequested {
		t.Fatal("expected TVDB lookup to run")
	}
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d: %#v", len(candidates), candidates)
	}
	if candidates[0].Provider != "fanart" || candidates[1].Provider != "tvdb" {
		t.Fatalf("unexpected candidate providers: %#v", candidates)
	}
}

func TestTVDBSeasonPoster_EmptySeriesIDReturnsNoError(t *testing.T) {
	client := &TVDBClient{APIKey: "tvdb"}

	poster, err := client.seasonPoster(context.Background(), "Show", "", 1)
	if err != nil {
		t.Fatalf("seasonPoster returned error: %v", err)
	}
	if poster != "" {
		t.Fatalf("expected empty poster, got %q", poster)
	}
}

func TestIdentifyMovieResult_ReturnsRetryableProviderErrors(t *testing.T) {
	provider := &stubMovieProvider{
		searchErr: &ProviderError{
			Provider:   "tmdb",
			StatusCode: http.StatusTooManyRequests,
			Retryable:  true,
		},
	}
	p := &Pipeline{movieProvider: provider}

	result, err := p.IdentifyMovieResult(context.Background(), MediaInfo{Title: "Blade", Year: 1998})
	if result != nil {
		t.Fatalf("expected no result, got %#v", result)
	}
	if !IsRetryableProviderError(err) {
		t.Fatalf("expected retryable provider error, got %v", err)
	}
}

func TestIdentifyMovieResult_ExplicitTMDBIDFallsBackToSearchWhenLookupFailsNonRetryably(t *testing.T) {
	provider := &stubMovieProvider{
		searchResults: []MatchResult{{
			Title:       "Blade",
			ReleaseDate: "1998-08-21",
			Provider:    "tmdb",
			ExternalID:  "36647",
		}},
		lookupErr: errors.New("not found"),
	}
	p := &Pipeline{movieProvider: provider}

	result, err := p.IdentifyMovieResult(context.Background(), MediaInfo{Title: "Blade", Year: 1998, TMDBID: 444})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Title != "Blade" {
		t.Fatalf("expected fallback search result, got %#v", result)
	}
}

func TestIdentifyMovieResult_FallsBackToSearchCandidateWhenLookupFailsNonRetryably(t *testing.T) {
	provider := &stubMovieProvider{
		searchResults: []MatchResult{{
			Title:       "Blade",
			ReleaseDate: "1998-08-21",
			Provider:    "tmdb",
			ExternalID:  "36647",
		}},
		lookupErr: errors.New("not found"),
	}
	p := &Pipeline{movieProvider: provider}

	result, err := p.IdentifyMovieResult(context.Background(), MediaInfo{Title: "Blade", Year: 1998})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Title != "Blade" {
		t.Fatalf("expected fallback search result, got %#v", result)
	}
}

func TestIdentifyTV_DoesNotFallbackToFirstLowConfidenceResult(t *testing.T) {
	tmdb := &stubTVProvider{
		name: "tmdb",
		searchResults: []MatchResult{
			{Title: "Completely Different Show", Provider: "tmdb", ExternalID: "10"},
		},
	}
	p := &Pipeline{tvProviders: []TVProvider{tmdb}}

	res := p.IdentifyTV(context.Background(), MediaInfo{Title: "Wanted Show"})
	if res != nil {
		t.Fatalf("expected no match, got %#v", res)
	}
}

func TestIdentifyTV_AutoMatchesShowSeasonLayout(t *testing.T) {
	tmdb := &stubTVProvider{
		name: "tmdb",
		searchResults: []MatchResult{
			{Title: "Show", ReleaseDate: "2024-01-01", Provider: "tmdb", ExternalID: "10"},
		},
		episodeResult: &MatchResult{
			Title:      "Show - S01E01 - Pilot",
			Provider:   "tmdb",
			ExternalID: "10",
		},
	}
	p := &Pipeline{tvProviders: []TVProvider{tmdb}}

	info := tvInfoFromPath("Show/Season 01/S01E01.mkv", "S01E01.mkv")
	res := p.IdentifyTV(context.Background(), info)
	if res == nil {
		t.Fatal("expected match")
	}
	if res.Title != "Show - S01E01 - Pilot" {
		t.Fatalf("title = %q", res.Title)
	}
}

func TestIdentifyTV_AutoMatchesShowDashSeasonLayout(t *testing.T) {
	tmdb := &stubTVProvider{
		name: "tmdb",
		searchResults: []MatchResult{
			{Title: "Show", Provider: "tmdb", ExternalID: "10"},
		},
		episodeResult: &MatchResult{
			Title:      "Show - S01E01 - Pilot",
			Provider:   "tmdb",
			ExternalID: "10",
		},
	}
	p := &Pipeline{tvProviders: []TVProvider{tmdb}}

	info := tvInfoFromPath("Show-Season1/S01E01.mkv", "S01E01.mkv")
	res := p.IdentifyTV(context.Background(), info)
	if res == nil {
		t.Fatal("expected match")
	}
	if res.Title != "Show - S01E01 - Pilot" {
		t.Fatalf("title = %q", res.Title)
	}
}

func TestIdentifyTV_StripsTrailingYearFromShowFolderTitle(t *testing.T) {
	tmdb := &stubTVProvider{
		name: "tmdb",
		searchResults: []MatchResult{
			{Title: "Show", ReleaseDate: "2024-01-01", Provider: "tmdb", ExternalID: "10"},
		},
		episodeResult: &MatchResult{
			Title:      "Show - S01E01 - Pilot",
			Provider:   "tmdb",
			ExternalID: "10",
		},
	}
	p := &Pipeline{tvProviders: []TVProvider{tmdb}}

	info := tvInfoFromPath("Show (2024)/Season 01/S01E01.mkv", "S01E01.mkv")
	res := p.IdentifyTV(context.Background(), info)
	if res == nil {
		t.Fatal("expected match")
	}
	if res.Title != "Show - S01E01 - Pilot" {
		t.Fatalf("title = %q", res.Title)
	}
}

func TestIdentifyTV_LeavesAmbiguousCandidatesUnmatched(t *testing.T) {
	tmdb := &stubTVProvider{
		name: "tmdb",
		searchResults: []MatchResult{
			{Title: "Show", ReleaseDate: "2024-01-01", Provider: "tmdb", ExternalID: "10"},
			{Title: "Show", ReleaseDate: "2024-01-01", Provider: "tmdb", ExternalID: "11"},
		},
	}
	p := &Pipeline{tvProviders: []TVProvider{tmdb}}

	info := tvInfoFromPath("Show/Season 01/S01E01.mkv", "S01E01.mkv")
	res := p.IdentifyTV(context.Background(), info)
	if res != nil {
		t.Fatalf("expected no match, got %#v", res)
	}
}

func TestIdentifyTV_UsesMatchingProviderForEpisodeLookup(t *testing.T) {
	tmdb := &stubTVProvider{name: "tmdb"}
	tvdb := &stubTVProvider{
		name: "tvdb",
		searchResults: []MatchResult{
			{Title: "Show", Provider: "tvdb", ExternalID: "series-55"},
		},
		episodeResult: &MatchResult{
			Title:      "Show - S01E03 - Episode",
			Provider:   "tvdb",
			ExternalID: "series-55",
		},
	}
	p := &Pipeline{tvProviders: []TVProvider{tmdb, tvdb}}

	res := p.IdentifyTV(context.Background(), MediaInfo{Title: "Show", TVDBID: "series-55", Season: 1, Episode: 3})
	if res == nil {
		t.Fatal("expected match")
	}
	if len(tmdb.episodeCalls) != 0 {
		t.Fatalf("tmdb should not be used for tvdb match, calls = %#v", tmdb.episodeCalls)
	}
	if len(tvdb.episodeCalls) != 1 || tvdb.episodeCalls[0] != "series-55" {
		t.Fatalf("tvdb episode lookup calls = %#v", tvdb.episodeCalls)
	}
}

func TestIdentifyAnime_DoesNotFallbackToTVDBSearchWhenTMDBDoesNotResolve(t *testing.T) {
	tmdb := &stubTVProvider{
		name: "tmdb",
	}
	tvdb := &stubTVProvider{
		name: "tvdb",
		searchResults: []MatchResult{
			{Title: "Frieren", Provider: "tvdb", ExternalID: "series-55"},
		},
		episodeResult: &MatchResult{
			Title:      "Frieren - S01E12 - Episode",
			Provider:   "tvdb",
			ExternalID: "series-55",
		},
	}
	p := &Pipeline{tvProviders: []TVProvider{tmdb, tvdb}}

	res := p.IdentifyAnime(context.Background(), MediaInfo{Title: "Frieren", Season: 1, Episode: 12})
	if res != nil {
		t.Fatalf("expected no match, got %#v", res)
	}
	if len(tmdb.episodeCalls) != 0 {
		t.Fatalf("tmdb episode lookup calls = %#v", tmdb.episodeCalls)
	}
	if len(tvdb.episodeCalls) != 0 {
		t.Fatalf("tvdb should not be used, calls = %#v", tvdb.episodeCalls)
	}
}

func TestIdentifyAnime_UsesTMDBResultEvenWhenTVDBAlsoMatches(t *testing.T) {
	tmdb := &stubTVProvider{
		name: "tmdb",
		searchResults: []MatchResult{
			{Title: "Frieren", Provider: "tmdb", ExternalID: "10"},
		},
		episodeResult: &MatchResult{
			Title:      "Frieren - S01E12 - Episode",
			Provider:   "tmdb",
			ExternalID: "10",
		},
	}
	tvdb := &stubTVProvider{
		name: "tvdb",
		searchResults: []MatchResult{
			{Title: "Frieren", Provider: "tvdb", ExternalID: "series-55"},
		},
	}
	p := &Pipeline{tvProviders: []TVProvider{tmdb, tvdb}}

	res := p.IdentifyAnime(context.Background(), MediaInfo{Title: "Frieren", Season: 1, Episode: 12})
	if res == nil {
		t.Fatal("expected match")
	}
	if res.Provider != "tmdb" {
		t.Fatalf("provider = %q", res.Provider)
	}
	if len(tmdb.episodeCalls) != 1 || tmdb.episodeCalls[0] != "10" {
		t.Fatalf("tmdb episode lookup calls = %#v", tmdb.episodeCalls)
	}
	if len(tvdb.episodeCalls) != 0 {
		t.Fatalf("tvdb episode lookup calls = %#v", tvdb.episodeCalls)
	}
}

func TestIdentifyAnime_PrefersExactSeriesOverHigherRankedSequelSearchResult(t *testing.T) {
	tmdb := &stubTVProvider{
		name: "tmdb",
		searchResults: []MatchResult{
			{Title: "Dragon Ball Z", ReleaseDate: "1989-04-26", Provider: "tmdb", ExternalID: "20"},
			{Title: "Dragon Ball", ReleaseDate: "1986-02-26", Provider: "tmdb", ExternalID: "10"},
		},
		episodeResult: &MatchResult{
			Title:      "Dragon Ball - S01E01 - Secret of the Dragon Balls",
			Provider:   "tmdb",
			ExternalID: "10",
		},
	}
	p := &Pipeline{tvProviders: []TVProvider{tmdb}}

	res := p.IdentifyAnime(context.Background(), MediaInfo{
		Title:   "Dragon Ball",
		Year:    1986,
		Season:  1,
		Episode: 1,
	})
	if res == nil {
		t.Fatal("expected match")
	}
	if res.Provider != "tmdb" || res.ExternalID != "10" {
		t.Fatalf("unexpected result = %#v", res)
	}
	if len(tmdb.episodeCalls) != 1 || tmdb.episodeCalls[0] != "10" {
		t.Fatalf("tmdb episode lookup calls = %#v", tmdb.episodeCalls)
	}
}

func TestIdentifyAnime_ExplicitTMDBIDUsesEpisodeMetadata(t *testing.T) {
	tmdb := &stubTVProvider{
		name: "tmdb",
		episodeResult: &MatchResult{
			Title:      "Frieren - S01E12 - Episode",
			Provider:   "tmdb",
			ExternalID: "123",
		},
	}
	p := &Pipeline{
		tvProviders:           []TVProvider{tmdb},
		seriesDetailsProvider: &stubSeriesDetailsProvider{details: &SeriesDetails{Name: "Frieren"}},
	}

	res := p.IdentifyAnime(context.Background(), MediaInfo{TMDBID: 123, Season: 1, Episode: 12})
	if res == nil {
		t.Fatal("expected match")
	}
	if res.Provider != "tmdb" {
		t.Fatalf("provider = %q", res.Provider)
	}
	if len(tmdb.episodeCalls) != 1 || tmdb.episodeCalls[0] != "123" {
		t.Fatalf("episode lookup calls = %#v", tmdb.episodeCalls)
	}
}

func TestIdentifyTV_ExplicitTMDBIDWithoutEpisodeCarriesIMDbMetadata(t *testing.T) {
	p := &Pipeline{
		tvProviders: []TVProvider{&stubTVProvider{name: "tmdb"}},
		seriesDetailsProvider: &stubSeriesDetailsProvider{details: &SeriesDetails{
			Name:       "Show",
			IMDbID:     "tt1234567",
			IMDbRating: 8.4,
		}},
	}

	res := p.IdentifyTV(context.Background(), MediaInfo{TMDBID: 123})
	if res == nil {
		t.Fatal("expected match")
	}
	if res.IMDbID != "tt1234567" {
		t.Fatalf("imdb id = %q", res.IMDbID)
	}
	if res.IMDbRating != 8.4 {
		t.Fatalf("imdb rating = %v", res.IMDbRating)
	}
}

func TestIdentifyTV_FallsBackToTMDBSeriesDetailsForIMDbMetadataWhenEpisodeLookupMisses(t *testing.T) {
	tmdb := &stubTVProvider{
		name: "tmdb",
		searchResults: []MatchResult{
			{Title: "Show", Provider: "tmdb", ExternalID: "123"},
		},
	}
	p := &Pipeline{
		tvProviders: []TVProvider{tmdb},
		seriesDetailsProvider: &stubSeriesDetailsProvider{details: &SeriesDetails{
			Name:   "Show",
			IMDbID: "tt7654321",
		}},
		imdbRatings: &stubIMDbRatingProvider{rating: 7.7},
	}

	res := p.IdentifyTV(context.Background(), MediaInfo{
		Title:   "Show",
		Season:  1,
		Episode: 1,
	})
	if res == nil {
		t.Fatal("expected match")
	}
	if res.IMDbID != "tt7654321" {
		t.Fatalf("imdb id = %q", res.IMDbID)
	}
	if res.IMDbRating != 7.7 {
		t.Fatalf("imdb rating = %v", res.IMDbRating)
	}
}

func TestGetSeriesDetails_EnrichesIMDbRating(t *testing.T) {
	p := &Pipeline{
		seriesDetailsProvider: &stubSeriesDetailsProvider{details: &SeriesDetails{
			Name:   "Show",
			IMDbID: "tt7654321",
		}},
		imdbRatings: &stubIMDbRatingProvider{rating: 9.1},
	}

	res, err := p.GetSeriesDetails(context.Background(), 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected details")
	}
	if res.IMDbRating != 9.1 {
		t.Fatalf("imdb rating = %v", res.IMDbRating)
	}
}

func TestIdentifyAnime_ExplicitTVDBIDDoesNotBypassTMDBOnlyLookup(t *testing.T) {
	tvdb := &stubTVProvider{
		name: "tvdb",
		episodeResult: &MatchResult{
			Title:      "Frieren - S01E12 - Episode",
			Provider:   "tvdb",
			ExternalID: "series-55",
		},
	}
	p := &Pipeline{tvProviders: []TVProvider{tvdb}}

	res := p.IdentifyAnime(context.Background(), MediaInfo{Title: "Frieren", TVDBID: "series-55", Season: 1, Episode: 12})
	if res != nil {
		t.Fatalf("expected no match, got %#v", res)
	}
	if len(tvdb.episodeCalls) != 0 {
		t.Fatalf("tvdb episode lookup calls = %#v", tvdb.episodeCalls)
	}
}

func TestIdentifyAnime_AbsoluteEpisodeOnlyReturnsNoMatch(t *testing.T) {
	tmdb := &stubTVProvider{
		name: "tmdb",
		searchResults: []MatchResult{
			{Title: "Frieren", Provider: "tmdb", ExternalID: "10"},
		},
	}
	p := &Pipeline{tvProviders: []TVProvider{tmdb}}

	res := p.IdentifyAnime(context.Background(), MediaInfo{Title: "Frieren", AbsoluteEpisode: 12})
	if res != nil {
		t.Fatalf("expected no match, got %#v", res)
	}
}

func TestIdentifyMovie_ExactTitleAndYearAutoMatches(t *testing.T) {
	movie := &stubMovieProvider{
		searchResults: []MatchResult{
			{Title: "Die My Love", ReleaseDate: "2025-01-01", Provider: "tmdb", ExternalID: "444"},
		},
	}
	p := &Pipeline{movieProvider: movie}

	res := p.IdentifyMovie(context.Background(), MediaInfo{Title: "Die My Love", Year: 2025})
	if res == nil {
		t.Fatal("expected match")
	}
	if res.Title != "Die My Love" {
		t.Fatalf("title = %q", res.Title)
	}
}

func TestIdentifyMovie_ExactTitleYearTieUsesFirstRankedResult(t *testing.T) {
	movie := &stubMovieProvider{
		searchResults: []MatchResult{
			{Title: "Smile", ReleaseDate: "2022-09-23", Provider: "tmdb", ExternalID: "111"},
			{Title: "Smile", ReleaseDate: "2022-10-28", Provider: "tmdb", ExternalID: "222"},
		},
	}
	p := &Pipeline{movieProvider: movie}

	res := p.IdentifyMovie(context.Background(), MediaInfo{Title: "Smile", Year: 2022})
	if res == nil {
		t.Fatal("expected match")
	}
	if res.ExternalID != "111" {
		t.Fatalf("external id = %q", res.ExternalID)
	}
}

func TestIdentifyMovie_ExactTitleWithoutYearAutoMatchesWhenUnique(t *testing.T) {
	movie := &stubMovieProvider{
		searchResults: []MatchResult{
			{Title: "Die My Love", ReleaseDate: "2025-01-01", Provider: "tmdb", ExternalID: "444"},
			{Title: "Different Movie", ReleaseDate: "2024-01-01", Provider: "tmdb", ExternalID: "555"},
		},
	}
	p := &Pipeline{movieProvider: movie}

	res := p.IdentifyMovie(context.Background(), MediaInfo{Title: "Die My Love"})
	if res == nil {
		t.Fatal("expected match")
	}
	if res.Title != "Die My Love" {
		t.Fatalf("title = %q", res.Title)
	}
}

func TestIdentifyMovie_NoisyFolderTitleAutoMatches(t *testing.T) {
	movie := &stubMovieProvider{
		searchResults: []MatchResult{
			{Title: "I Heart Huckabees", ReleaseDate: "2004-01-01", Provider: "tmdb", ExternalID: "444"},
		},
	}
	p := &Pipeline{movieProvider: movie}

	parsed := ParseMovie(
		"I.Heart.Huckabees.2004.1080p.AMZN.WEBRip.DDP5.1.x264-monkee/I.Heart.Huckabees.2004.1080p.AMZN.WEBRip.DDP5.1.x264-monkee.mkv",
		"I.Heart.Huckabees.2004.1080p.AMZN.WEBRip.DDP5.1.x264-monkee.mkv",
	)
	res := p.IdentifyMovie(context.Background(), MovieMediaInfo(parsed))
	if res == nil {
		t.Fatal("expected match")
	}
	if res.Title != "I Heart Huckabees" {
		t.Fatalf("title = %q", res.Title)
	}
}

func TestIdentifyMovie_ExactTitleWithoutYearStaysUnmatchedWhenAmbiguous(t *testing.T) {
	movie := &stubMovieProvider{
		searchResults: []MatchResult{
			{Title: "Suspiria", ReleaseDate: "1977-01-01", Provider: "tmdb", ExternalID: "111"},
			{Title: "Suspiria", ReleaseDate: "2018-01-01", Provider: "tmdb", ExternalID: "222"},
		},
	}
	p := &Pipeline{movieProvider: movie}

	res := p.IdentifyMovie(context.Background(), MediaInfo{Title: "Suspiria"})
	if res != nil {
		t.Fatalf("expected no match, got %#v", res)
	}
}

func TestIdentifyMovie_ConflictingYearStaysUnmatched(t *testing.T) {
	movie := &stubMovieProvider{
		searchResults: []MatchResult{
			{Title: "Die My Love", ReleaseDate: "2024-01-01", Provider: "tmdb", ExternalID: "444"},
		},
	}
	p := &Pipeline{movieProvider: movie}

	res := p.IdentifyMovie(context.Background(), MediaInfo{Title: "Die My Love", Year: 2025})
	if res != nil {
		t.Fatalf("expected no match, got %#v", res)
	}
}

func TestIdentifyMovie_ExplicitTMDBIDUsesLookup(t *testing.T) {
	movie := &stubMovieProvider{
		lookupResult: &MatchResult{Title: "Movie", Provider: "tmdb", ExternalID: "444"},
	}
	p := &Pipeline{movieProvider: movie}

	res := p.IdentifyMovie(context.Background(), MediaInfo{TMDBID: 444})
	if res == nil {
		t.Fatal("expected match")
	}
	if res.Title != "Movie" {
		t.Fatalf("title = %q", res.Title)
	}
	if len(movie.lookupCalls) != 1 || movie.lookupCalls[0] != "444" {
		t.Fatalf("lookup calls = %#v", movie.lookupCalls)
	}
}
