package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

const tmdbBaseURL = "https://api.themoviedb.org/3"
const tmdbImageBase = "https://image.tmdb.org/t/p"

type TMDBClient struct {
	APIKey  string
	cache   ProviderCache
	baseURL string

	mu           sync.RWMutex
	tvDetails    map[int]*tmdbTVDetails
	movieDetails map[int]*tmdbMovieDetails
	tvIMDbIDs    map[int]string
	movieIMDbIDs map[int]string
}

func (c *TMDBClient) ProviderName() string {
	return "tmdb"
}

type TMDBResult struct {
	ID           int     `json:"id"`
	Name         string  `json:"name,omitempty"`
	Title        string  `json:"title,omitempty"`
	Overview     string  `json:"overview"`
	PosterPath   string  `json:"poster_path"`
	BackdropPath string  `json:"backdrop_path"`
	ReleaseDate  string  `json:"release_date,omitempty"`
	FirstAirDate string  `json:"first_air_date,omitempty"`
	VoteAverage  float64 `json:"vote_average"`
}

type tmdbCreditCast struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Character   string `json:"character"`
	Order       int    `json:"order"`
	ProfilePath string `json:"profile_path"`
}

type tmdbCredits struct {
	Cast []tmdbCreditCast `json:"cast"`
}

type tmdbMovieDetails struct {
	TMDBResult
	Runtime     int                     `json:"runtime"`
	Genres      []tmdbGenre             `json:"genres"`
	Credits     tmdbCredits             `json:"credits"`
	ExternalIDs tmdbExternalIDsResponse `json:"external_ids"`
}

type tmdbTVDetails struct {
	TMDBResult
	Genres           []tmdbGenre             `json:"genres"`
	EpisodeRunTime   []int                   `json:"episode_run_time"`
	NumberOfSeasons  int                     `json:"number_of_seasons"`
	NumberOfEpisodes int                     `json:"number_of_episodes"`
	Credits          tmdbCredits             `json:"credits"`
	ExternalIDs      tmdbExternalIDsResponse `json:"external_ids"`
}

type tmdbExternalIDsResponse struct {
	IMDbID string `json:"imdb_id"`
	TVDBID int    `json:"tvdb_id"`
}

type tmdbImageAsset struct {
	FilePath    string  `json:"file_path"`
	ISO6391     string  `json:"iso_639_1"`
	VoteAverage float64 `json:"vote_average"`
}

type tmdbImagesResponse struct {
	Posters []tmdbImageAsset `json:"posters"`
}

type tmdbEpisodeDetails struct {
	Name         string  `json:"name"`
	Overview     string  `json:"overview"`
	StillPath    string  `json:"still_path"`
	AirDate      string  `json:"air_date"`
	VoteAverage  float64 `json:"vote_average"`
	SeasonNumber int     `json:"season_number"`
	EpisodeNum   int     `json:"episode_number"`
}

type TMDBSearchResponse struct {
	Results []TMDBResult `json:"results"`
}

func NewTMDBClient(apiKey string) *TMDBClient {
	return &TMDBClient{
		APIKey:       apiKey,
		baseURL:      tmdbBaseURL,
		tvDetails:    map[int]*tmdbTVDetails{},
		movieDetails: map[int]*tmdbMovieDetails{},
		tvIMDbIDs:    map[int]string{},
		movieIMDbIDs: map[int]string{},
	}
}

func (c *TMDBClient) SetCache(cache ProviderCache) {
	c.cache = cache
}

func (c *TMDBClient) SearchTV(ctx context.Context, query string) ([]MatchResult, error) {
	u := fmt.Sprintf("%s/search/tv?api_key=%s&query=%s", c.baseURL, c.APIKey, url.QueryEscape(query))
	resp, err := doCachedJSONRequest(ctx, providerHTTPClient, c.cache, "tmdb", http.MethodGet, u, nil, nil, 24*time.Hour, 1)
	if err != nil {
		return nil, err
	}
	var res TMDBSearchResponse
	if err := json.Unmarshal(resp.Body, &res); err != nil {
		return nil, err
	}
	out := make([]MatchResult, 0, len(res.Results))
	for _, r := range res.Results {
		out = append(out, c.tmdbResultToMatch(r, r.Name, r.FirstAirDate))
	}
	return out, nil
}

func (c *TMDBClient) SearchMovie(ctx context.Context, query string) ([]MatchResult, error) {
	u := fmt.Sprintf("%s/search/movie?api_key=%s&query=%s", c.baseURL, c.APIKey, url.QueryEscape(query))
	resp, err := doCachedJSONRequest(ctx, providerHTTPClient, c.cache, "tmdb", http.MethodGet, u, nil, nil, 24*time.Hour, 1)
	if err != nil {
		return nil, err
	}
	var res TMDBSearchResponse
	if err := json.Unmarshal(resp.Body, &res); err != nil {
		return nil, err
	}
	out := make([]MatchResult, 0, len(res.Results))
	for _, r := range res.Results {
		out = append(out, c.tmdbResultToMatch(r, r.Title, r.ReleaseDate))
	}
	return out, nil
}

func (c *TMDBClient) GetMovie(ctx context.Context, movieID string) (*MatchResult, error) {
	id, err := strconv.Atoi(movieID)
	if err != nil {
		return nil, err
	}
	detail, err := c.getMovieDetails(ctx, id)
	if err != nil || detail == nil {
		return nil, err
	}
	m := c.tmdbResultToMatch(detail.TMDBResult, detail.Title, detail.ReleaseDate)
	m.IMDbID, _ = c.getMovieIMDbID(ctx, id)
	m.Genres = tmdbGenresToNames(detail.Genres)
	m.Cast = tmdbCreditsToCast(detail.Credits)
	m.Runtime = detail.Runtime
	return &m, nil
}

func (c *TMDBClient) GetEpisode(ctx context.Context, seriesID string, season, episode int) (*MatchResult, error) {
	tvID, err := strconv.Atoi(seriesID)
	if err != nil {
		return nil, err
	}
	ep, err := c.getEpisodeDetails(tvID, season, episode)
	if err != nil {
		return nil, err
	}
	series, _ := c.getTVDetails(ctx, tvID)
	posterPath := ep.StillPath
	if posterPath == "" && series != nil {
		posterPath = series.PosterPath
	}
	releaseDate := ep.AirDate
	if releaseDate == "" && series != nil {
		releaseDate = series.FirstAirDate
	}
	title := ep.Name
	if title == "" && series != nil {
		title = fmt.Sprintf("%s - S%02dE%02d", series.Name, season, episode)
	} else if series != nil {
		title = fmt.Sprintf("%s - S%02dE%02d - %s", series.Name, season, episode, ep.Name)
	}
	m := c.tmdbResultToMatch(TMDBResult{
		Overview:     ep.Overview,
		PosterPath:   posterPath,
		BackdropPath: ep.StillPath,
		ReleaseDate:  releaseDate,
		VoteAverage:  ep.VoteAverage,
	}, title, releaseDate)
	if m.BackdropURL == "" && series != nil {
		m.BackdropURL = tmdbImageURL(series.BackdropPath, "w500")
	}
	m.IMDbID, _ = c.getTVIMDbID(ctx, tvID)
	if series != nil {
		m.Genres = tmdbGenresToNames(series.Genres)
		m.Cast = tmdbCreditsToCast(series.Credits)
		m.Runtime = tmdbPrimaryRuntime(series.EpisodeRunTime)
	}
	m.Provider = "tmdb"
	// Use series ID (tvID) so all episodes of the same show share one tmdb_id for grouping.
	m.ExternalID = strconv.Itoa(tvID)
	return &m, nil
}

func (c *TMDBClient) tmdbResultToMatch(r TMDBResult, title, releaseDate string) MatchResult {
	return MatchResult{
		Title:       title,
		Overview:    r.Overview,
		PosterURL:   tmdbImageURL(r.PosterPath, "w500"),
		BackdropURL: tmdbImageURL(r.BackdropPath, "w500"),
		ReleaseDate: releaseDate,
		VoteAverage: r.VoteAverage,
		Provider:    "tmdb",
		ExternalID:  strconv.Itoa(r.ID),
	}
}

func (c *TMDBClient) getTVDetails(ctx context.Context, id int) (*tmdbTVDetails, error) {
	c.mu.RLock()
	if cached, ok := c.tvDetails[id]; ok {
		c.mu.RUnlock()
		copy := *cached
		return &copy, nil
	}
	c.mu.RUnlock()
	u := fmt.Sprintf("%s/tv/%d?api_key=%s&append_to_response=credits,external_ids", c.baseURL, id, c.APIKey)
	resp, err := doCachedJSONRequest(ctx, providerHTTPClient, c.cache, "tmdb", http.MethodGet, u, nil, nil, 7*24*time.Hour, 1)
	if err != nil {
		return nil, err
	}
	var res tmdbTVDetails
	if err := json.Unmarshal(resp.Body, &res); err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.tvDetails[id] = &res
	c.mu.Unlock()
	return &res, nil
}

func (c *TMDBClient) getMovieDetails(ctx context.Context, id int) (*tmdbMovieDetails, error) {
	c.mu.RLock()
	if cached, ok := c.movieDetails[id]; ok {
		c.mu.RUnlock()
		copy := *cached
		return &copy, nil
	}
	c.mu.RUnlock()
	u := fmt.Sprintf("%s/movie/%d?api_key=%s&append_to_response=credits,external_ids", c.baseURL, id, c.APIKey)
	resp, err := doCachedJSONRequest(ctx, providerHTTPClient, c.cache, "tmdb", http.MethodGet, u, nil, nil, 7*24*time.Hour, 1)
	if err != nil {
		return nil, err
	}
	var res tmdbMovieDetails
	if err := json.Unmarshal(resp.Body, &res); err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.movieDetails[id] = &res
	c.mu.Unlock()
	return &res, nil
}

func (c *TMDBClient) getMoviePosters(ctx context.Context, id int) ([]string, error) {
	if id <= 0 {
		return nil, nil
	}
	u := fmt.Sprintf("%s/movie/%d/images?api_key=%s&include_image_language=en,null", c.baseURL, id, c.APIKey)
	resp, err := doCachedJSONRequest(ctx, providerHTTPClient, c.cache, "tmdb", http.MethodGet, u, nil, nil, 7*24*time.Hour, 1)
	if err != nil {
		return nil, err
	}
	var res tmdbImagesResponse
	if err := json.Unmarshal(resp.Body, &res); err != nil {
		return nil, err
	}
	return tmdbPosterImageURLs(res.Posters), nil
}

func (c *TMDBClient) getTVPosters(ctx context.Context, id int) ([]string, error) {
	if id <= 0 {
		return nil, nil
	}
	u := fmt.Sprintf("%s/tv/%d/images?api_key=%s&include_image_language=en,null", c.baseURL, id, c.APIKey)
	resp, err := doCachedJSONRequest(ctx, providerHTTPClient, c.cache, "tmdb", http.MethodGet, u, nil, nil, 7*24*time.Hour, 1)
	if err != nil {
		return nil, err
	}
	var res tmdbImagesResponse
	if err := json.Unmarshal(resp.Body, &res); err != nil {
		return nil, err
	}
	return tmdbPosterImageURLs(res.Posters), nil
}

func tmdbPosterImageURLs(images []tmdbImageAsset) []string {
	urls := make([]string, 0, len(images))
	for _, image := range images {
		if image.FilePath == "" {
			continue
		}
		urls = append(urls, tmdbImageURL(image.FilePath, "w500"))
	}
	return urls
}

// GetSeriesDetails returns TV series metadata by TMDB ID for the show-detail UI.
func (c *TMDBClient) GetSeriesDetails(ctx context.Context, tmdbID int) (*SeriesDetails, error) {
	detail, err := c.getTVDetails(ctx, tmdbID)
	if err != nil || detail == nil {
		return nil, err
	}
	imdbID, _ := c.getTVIMDbID(ctx, tmdbID)
	tvdbID := ""
	if detail.ExternalIDs.TVDBID > 0 {
		tvdbID = strconv.Itoa(detail.ExternalIDs.TVDBID)
	}
	return &SeriesDetails{
		Name:             detail.Name,
		Overview:         detail.Overview,
		PosterPath:       tmdbImageURL(detail.PosterPath, "w500"),
		BackdropPath:     tmdbImageURL(detail.BackdropPath, "w500"),
		FirstAirDate:     detail.FirstAirDate,
		VoteAverage:      detail.VoteAverage,
		IMDbID:           imdbID,
		Genres:           tmdbGenresToNames(detail.Genres),
		Cast:             tmdbCreditsToCast(detail.Credits),
		Runtime:          tmdbPrimaryRuntime(detail.EpisodeRunTime),
		NumberOfSeasons:  detail.NumberOfSeasons,
		NumberOfEpisodes: detail.NumberOfEpisodes,
		TVDBID:           tvdbID,
	}, nil
}

func (c *TMDBClient) GetMovieDetails(ctx context.Context, tmdbID int) (*MovieDetails, error) {
	detail, err := c.getMovieDetails(ctx, tmdbID)
	if err != nil || detail == nil {
		return nil, err
	}
	imdbID, _ := c.getMovieIMDbID(ctx, tmdbID)
	return &MovieDetails{
		Title:        detail.Title,
		Overview:     detail.Overview,
		PosterPath:   tmdbImageURL(detail.PosterPath, "w500"),
		BackdropPath: tmdbImageURL(detail.BackdropPath, "w500"),
		ReleaseDate:  detail.ReleaseDate,
		IMDbID:       imdbID,
		Genres:       tmdbGenresToNames(detail.Genres),
		Cast:         tmdbCreditsToCast(detail.Credits),
		Runtime:      detail.Runtime,
	}, nil
}

func (c *TMDBClient) getMovieIMDbID(ctx context.Context, id int) (string, error) {
	if detail, err := c.getMovieDetails(ctx, id); err == nil && detail != nil && detail.ExternalIDs.IMDbID != "" {
		return detail.ExternalIDs.IMDbID, nil
	}
	c.mu.RLock()
	if cached, ok := c.movieIMDbIDs[id]; ok {
		c.mu.RUnlock()
		return cached, nil
	}
	c.mu.RUnlock()
	imdbID, err := c.getIMDbID(ctx, fmt.Sprintf("%s/movie/%d/external_ids?api_key=%s", c.baseURL, id, c.APIKey))
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	c.movieIMDbIDs[id] = imdbID
	c.mu.Unlock()
	return imdbID, nil
}

func (c *TMDBClient) getTVIMDbID(ctx context.Context, id int) (string, error) {
	if detail, err := c.getTVDetails(ctx, id); err == nil && detail != nil && detail.ExternalIDs.IMDbID != "" {
		return detail.ExternalIDs.IMDbID, nil
	}
	c.mu.RLock()
	if cached, ok := c.tvIMDbIDs[id]; ok {
		c.mu.RUnlock()
		return cached, nil
	}
	c.mu.RUnlock()
	imdbID, err := c.getIMDbID(ctx, fmt.Sprintf("%s/tv/%d/external_ids?api_key=%s", c.baseURL, id, c.APIKey))
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	c.tvIMDbIDs[id] = imdbID
	c.mu.Unlock()
	return imdbID, nil
}

func (c *TMDBClient) getIMDbID(ctx context.Context, endpoint string) (string, error) {
	resp, err := doCachedJSONRequest(ctx, providerHTTPClient, c.cache, "tmdb", http.MethodGet, endpoint, nil, nil, 30*24*time.Hour, 1)
	if err != nil {
		return "", err
	}
	var payload tmdbExternalIDsResponse
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return "", err
	}
	return payload.IMDbID, nil
}

func (c *TMDBClient) getEpisodeDetails(tvID, season, episode int) (*tmdbEpisodeDetails, error) {
	u := fmt.Sprintf("%s/tv/%d/season/%d/episode/%d?api_key=%s", c.baseURL, tvID, season, episode, c.APIKey)
	resp, err := doCachedJSONRequest(context.Background(), providerHTTPClient, c.cache, "tmdb", http.MethodGet, u, nil, nil, 7*24*time.Hour, 1)
	if err != nil {
		return nil, err
	}
	var res tmdbEpisodeDetails
	if err := json.Unmarshal(resp.Body, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *TMDBClient) seasonPoster(ctx context.Context, tvID int, season int) (string, error) {
	if c == nil || tvID <= 0 || season < 0 {
		return "", nil
	}
	u := fmt.Sprintf("%s/tv/%d/season/%d?api_key=%s", c.baseURL, tvID, season, c.APIKey)
	resp, err := doCachedJSONRequest(ctx, providerHTTPClient, c.cache, "tmdb", http.MethodGet, u, nil, nil, 7*24*time.Hour, 1)
	if err != nil {
		return "", err
	}
	var payload struct {
		PosterPath string `json:"poster_path"`
	}
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return "", err
	}
	return tmdbImageURL(payload.PosterPath, "w500"), nil
}

func tmdbImageURL(path, size string) string {
	if path == "" {
		return ""
	}
	if size == "" {
		size = "w500"
	}
	return fmt.Sprintf("%s/%s%s", tmdbImageBase, size, path)
}

// GetPosterURL returns the full TMDB poster URL for a path (e.g. from DB).
// Kept for backward compatibility with existing code that stores paths.
func GetPosterURL(path string, size string) string {
	return tmdbImageURL(path, size)
}

func tmdbCreditsToCast(credits tmdbCredits) []CastMember {
	if len(credits.Cast) == 0 {
		return nil
	}
	out := make([]CastMember, 0, len(credits.Cast))
	for _, member := range credits.Cast {
		if member.Name == "" {
			continue
		}
		out = append(out, CastMember{
			Name:        member.Name,
			Character:   member.Character,
			Order:       member.Order,
			ProfilePath: tmdbImageURL(member.ProfilePath, "w185"),
			Provider:    "tmdb",
			ProviderID:  strconv.Itoa(member.ID),
		})
		if len(out) >= 20 {
			break
		}
	}
	return out
}

func tmdbPrimaryRuntime(values []int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
