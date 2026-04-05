package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	tmdbDiscoverShelfTTL  = 6 * time.Hour
	tmdbDiscoverDetailTTL = 24 * time.Hour
	tmdbDiscoverGenreTTL  = 24 * time.Hour
	tmdbDiscoverHubLimit  = 20
)

type tmdbDiscoverListItem struct {
	ID           int     `json:"id"`
	MediaType    string  `json:"media_type,omitempty"`
	Name         string  `json:"name,omitempty"`
	Title        string  `json:"title,omitempty"`
	Overview     string  `json:"overview"`
	PosterPath   string  `json:"poster_path"`
	BackdropPath string  `json:"backdrop_path"`
	ReleaseDate  string  `json:"release_date,omitempty"`
	FirstAirDate string  `json:"first_air_date,omitempty"`
	VoteAverage  float64 `json:"vote_average"`
}

type tmdbDiscoverListResponse struct {
	Page         int                    `json:"page"`
	Results      []tmdbDiscoverListItem `json:"results"`
	TotalPages   int                    `json:"total_pages"`
	TotalResults int                    `json:"total_results"`
}

type tmdbGenre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type tmdbGenreListResponse struct {
	Genres []tmdbGenre `json:"genres"`
}

type tmdbVideo struct {
	Name     string `json:"name"`
	Site     string `json:"site"`
	Key      string `json:"key"`
	Type     string `json:"type"`
	Official bool   `json:"official"`
}

type tmdbVideosResponse struct {
	Results []tmdbVideo `json:"results"`
}

type tmdbMovieDetailResponse struct {
	ID           int                `json:"id"`
	Title        string             `json:"title"`
	Overview     string             `json:"overview"`
	PosterPath   string             `json:"poster_path"`
	BackdropPath string             `json:"backdrop_path"`
	ReleaseDate  string             `json:"release_date"`
	VoteAverage  float64            `json:"vote_average"`
	Status       string             `json:"status"`
	Runtime      int                `json:"runtime"`
	Genres       []tmdbGenre        `json:"genres"`
	Videos       tmdbVideosResponse `json:"videos"`
}

type tmdbTVDetailResponse struct {
	ID               int                `json:"id"`
	Name             string             `json:"name"`
	Overview         string             `json:"overview"`
	PosterPath       string             `json:"poster_path"`
	BackdropPath     string             `json:"backdrop_path"`
	FirstAirDate     string             `json:"first_air_date"`
	VoteAverage      float64            `json:"vote_average"`
	Status           string             `json:"status"`
	EpisodeRunTime   []int              `json:"episode_run_time"`
	NumberOfSeasons  int                `json:"number_of_seasons"`
	NumberOfEpisodes int                `json:"number_of_episodes"`
	Genres           []tmdbGenre        `json:"genres"`
	Videos           tmdbVideosResponse `json:"videos"`
}

func isTMDBDiscoverHTTPStatus(err error, statusCode int) bool {
	var providerErr *ProviderError
	return errors.As(err, &providerErr) && providerErr.Provider == "tmdb" && providerErr.StatusCode == statusCode
}

// normalizeDiscoverOrigin accepts only 2 ASCII letters (uppercased). Mirrors httpapi.parseDiscoverOriginCountry
// and web normalizeDiscoverOriginKey; change all three together.
func normalizeDiscoverOrigin(raw string) string {
	s := strings.TrimSpace(strings.ToUpper(raw))
	if len(s) != 2 {
		return ""
	}
	for i := 0; i < 2; i++ {
		if s[i] < 'A' || s[i] > 'Z' {
			return ""
		}
	}
	return s
}

func cloneStringMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m)+8)
	for k, v := range m {
		out[k] = v
	}
	return out
}

func (c *TMDBClient) GetDiscover(ctx context.Context, originCountry string) (*DiscoverResponse, error) {
	if err := c.requireTMDB(); err != nil {
		return nil, err
	}
	origin := normalizeDiscoverOrigin(originCountry)
	if origin == "" {
		return c.getDiscoverGlobalHub(ctx)
	}
	return c.getDiscoverOriginHub(ctx, origin)
}

func (c *TMDBClient) getDiscoverGlobalHub(ctx context.Context) (*DiscoverResponse, error) {
	trending, err := c.fetchDiscoverList(ctx, "/trending/all/day", "", tmdbDiscoverShelfTTL, tmdbDiscoverHubLimit, 1)
	if err != nil {
		return nil, err
	}
	popularMovies, err := c.fetchDiscoverList(ctx, "/movie/popular", string(DiscoverMediaTypeMovie), tmdbDiscoverShelfTTL, tmdbDiscoverHubLimit, 1)
	if err != nil {
		return nil, err
	}
	nowPlaying, err := c.fetchDiscoverList(ctx, "/movie/now_playing", string(DiscoverMediaTypeMovie), tmdbDiscoverShelfTTL, tmdbDiscoverHubLimit, 1)
	if err != nil {
		return nil, err
	}
	upcoming, err := c.fetchDiscoverList(ctx, "/movie/upcoming", string(DiscoverMediaTypeMovie), tmdbDiscoverShelfTTL, tmdbDiscoverHubLimit, 1)
	if err != nil {
		return nil, err
	}
	popularTV, err := c.fetchDiscoverList(ctx, "/tv/popular", string(DiscoverMediaTypeTV), tmdbDiscoverShelfTTL, tmdbDiscoverHubLimit, 1)
	if err != nil {
		return nil, err
	}
	onTheAir, err := c.fetchDiscoverList(ctx, "/tv/on_the_air", string(DiscoverMediaTypeTV), tmdbDiscoverShelfTTL, tmdbDiscoverHubLimit, 1)
	if err != nil {
		return nil, err
	}
	topRatedMovies, err := c.fetchDiscoverList(ctx, "/movie/top_rated", string(DiscoverMediaTypeMovie), tmdbDiscoverShelfTTL, tmdbDiscoverHubLimit, 1)
	if err != nil {
		return nil, err
	}
	topRatedTV, err := c.fetchDiscoverList(ctx, "/tv/top_rated", string(DiscoverMediaTypeTV), tmdbDiscoverShelfTTL, tmdbDiscoverHubLimit, 1)
	if err != nil {
		return nil, err
	}

	return &DiscoverResponse{
		Shelves: []DiscoverShelf{
			{ID: "trending", Title: "Trending Now", Items: trending},
			{ID: "popular-movies", Title: "Popular Movies", Items: popularMovies},
			{ID: "now-playing", Title: "Now Playing", Items: nowPlaying},
			{ID: "upcoming", Title: "Upcoming Movies", Items: upcoming},
			{ID: "popular-tv", Title: "Popular TV", Items: popularTV},
			{ID: "on-the-air", Title: "On The Air", Items: onTheAir},
			{ID: "top-rated", Title: "Top Rated Picks", Items: interleaveDiscoverItems(topRatedMovies, topRatedTV, tmdbDiscoverHubLimit)},
		},
	}, nil
}

func (c *TMDBClient) getDiscoverOriginHub(ctx context.Context, origin string) (*DiscoverResponse, error) {
	base := map[string]string{"with_origin_country": origin}

	trendingMovies, err := c.fetchDiscoverListWithParams(ctx, "/discover/movie", withDiscoverSort(base, "popularity.desc"), string(DiscoverMediaTypeMovie), tmdbDiscoverShelfTTL, tmdbDiscoverHubLimit, 1)
	if err != nil {
		return nil, err
	}
	trendingTV, err := c.fetchDiscoverListWithParams(ctx, "/discover/tv", withDiscoverSort(base, "popularity.desc"), string(DiscoverMediaTypeTV), tmdbDiscoverShelfTTL, tmdbDiscoverHubLimit, 1)
	if err != nil {
		return nil, err
	}
	trending := interleaveDiscoverItems(trendingMovies, trendingTV, tmdbDiscoverHubLimit)

	popularMovies, err := c.fetchDiscoverListWithParams(ctx, "/discover/movie", withDiscoverSort(base, "popularity.desc"), string(DiscoverMediaTypeMovie), tmdbDiscoverShelfTTL, tmdbDiscoverHubLimit, 1)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	today := now.Format("2006-01-02")
	nowPlayingParams := cloneStringMap(base)
	nowPlayingParams["sort_by"] = "primary_release_date.desc"
	nowPlayingParams["primary_release_date.gte"] = now.AddDate(0, 0, -75).Format("2006-01-02")
	nowPlayingParams["primary_release_date.lte"] = today
	nowPlaying, err := c.fetchDiscoverListWithParams(ctx, "/discover/movie", nowPlayingParams, string(DiscoverMediaTypeMovie), tmdbDiscoverShelfTTL, tmdbDiscoverHubLimit, 1)
	if err != nil {
		return nil, err
	}

	upcomingParams := cloneStringMap(base)
	upcomingParams["sort_by"] = "primary_release_date.asc"
	upcomingParams["primary_release_date.gte"] = today
	upcoming, err := c.fetchDiscoverListWithParams(ctx, "/discover/movie", upcomingParams, string(DiscoverMediaTypeMovie), tmdbDiscoverShelfTTL, tmdbDiscoverHubLimit, 1)
	if err != nil {
		return nil, err
	}

	popularTV, err := c.fetchDiscoverListWithParams(ctx, "/discover/tv", withDiscoverSort(base, "popularity.desc"), string(DiscoverMediaTypeTV), tmdbDiscoverShelfTTL, tmdbDiscoverHubLimit, 1)
	if err != nil {
		return nil, err
	}
	onTheAir, err := c.fetchDiscoverListWithParams(ctx, "/discover/tv", withDiscoverSort(base, "popularity.desc"), string(DiscoverMediaTypeTV), tmdbDiscoverShelfTTL, tmdbDiscoverHubLimit, 1)
	if err != nil {
		return nil, err
	}

	topRatedMovieParams := cloneStringMap(base)
	topRatedMovieParams["sort_by"] = "vote_average.desc"
	topRatedMovieParams["vote_count.gte"] = "200"
	topRatedMovies, err := c.fetchDiscoverListWithParams(ctx, "/discover/movie", topRatedMovieParams, string(DiscoverMediaTypeMovie), tmdbDiscoverShelfTTL, tmdbDiscoverHubLimit, 1)
	if err != nil {
		return nil, err
	}
	topRatedTVParams := cloneStringMap(base)
	topRatedTVParams["sort_by"] = "vote_average.desc"
	topRatedTVParams["vote_count.gte"] = "200"
	topRatedTV, err := c.fetchDiscoverListWithParams(ctx, "/discover/tv", topRatedTVParams, string(DiscoverMediaTypeTV), tmdbDiscoverShelfTTL, tmdbDiscoverHubLimit, 1)
	if err != nil {
		return nil, err
	}

	return &DiscoverResponse{
		Shelves: []DiscoverShelf{
			{ID: "trending", Title: "Trending Now", Items: trending},
			{ID: "popular-movies", Title: "Popular Movies", Items: popularMovies},
			{ID: "now-playing", Title: "Now Playing", Items: nowPlaying},
			{ID: "upcoming", Title: "Upcoming Movies", Items: upcoming},
			{ID: "popular-tv", Title: "Popular TV", Items: popularTV},
			{ID: "on-the-air", Title: "On The Air", Items: onTheAir},
			{ID: "top-rated", Title: "Top Rated Picks", Items: interleaveDiscoverItems(topRatedMovies, topRatedTV, tmdbDiscoverHubLimit)},
		},
	}, nil
}

func withDiscoverSort(base map[string]string, sortBy string) map[string]string {
	out := cloneStringMap(base)
	out["sort_by"] = sortBy
	return out
}

func (c *TMDBClient) fetchDiscoverListWithParams(
	ctx context.Context,
	path string,
	query map[string]string,
	fallbackType string,
	ttl time.Duration,
	limit int,
	page int,
) ([]DiscoverItem, error) {
	params := cloneStringMap(query)
	params["page"] = fmt.Sprintf("%d", page)
	var payload tmdbDiscoverListResponse
	if err := c.fetchJSON(ctx, c.discoverURL(path, params), ttl, &payload); err != nil {
		return nil, err
	}
	return mapTMDBDiscoverItems(payload.Results, fallbackType, limit), nil
}

func (c *TMDBClient) GetDiscoverGenres(ctx context.Context) (*DiscoverGenresResponse, error) {
	if err := c.requireTMDB(); err != nil {
		return nil, err
	}

	var moviePayload tmdbGenreListResponse
	if err := c.fetchJSON(ctx, c.discoverURL("/genre/movie/list", nil), tmdbDiscoverGenreTTL, &moviePayload); err != nil {
		return nil, err
	}
	var tvPayload tmdbGenreListResponse
	if err := c.fetchJSON(ctx, c.discoverURL("/genre/tv/list", nil), tmdbDiscoverGenreTTL, &tvPayload); err != nil {
		return nil, err
	}

	return &DiscoverGenresResponse{
		MovieGenres: mapTMDBGenres(moviePayload.Genres),
		TVGenres:    mapTMDBGenres(tvPayload.Genres),
	}, nil
}

func (c *TMDBClient) BrowseDiscover(
	ctx context.Context,
	category DiscoverBrowseCategory,
	mediaType DiscoverMediaType,
	genreID int,
	page int,
	originCountry string,
) (*DiscoverBrowseResponse, error) {
	if err := c.requireTMDB(); err != nil {
		return nil, err
	}
	if page <= 0 {
		page = 1
	}

	path, fallbackType, params, resolvedMediaType := tmdbBrowseRequest(category, mediaType, genreID, page, originCountry)
	var payload tmdbDiscoverListResponse
	if err := c.fetchJSON(ctx, c.discoverURL(path, params), tmdbDiscoverShelfTTL, &payload); err != nil {
		return nil, err
	}

	response := &DiscoverBrowseResponse{
		Items:        mapTMDBDiscoverItems(payload.Results, fallbackType, 0),
		Page:         normalizePositive(payload.Page, page),
		TotalPages:   normalizePositive(payload.TotalPages, 1),
		TotalResults: maxInt(payload.TotalResults, len(payload.Results)),
		Category:     category,
	}
	if resolvedMediaType != "" {
		response.MediaType = resolvedMediaType
	}
	if genreID > 0 {
		if genre, err := c.lookupGenre(ctx, resolvedMediaType, genreID); err != nil {
			return nil, err
		} else {
			response.Genre = genre
		}
	}

	return response, nil
}

func (c *TMDBClient) SearchDiscover(ctx context.Context, query string) (*DiscoverSearchResponse, error) {
	if err := c.requireTMDB(); err != nil {
		return nil, err
	}
	if len(query) < 2 {
		return &DiscoverSearchResponse{Movies: []DiscoverItem{}, TV: []DiscoverItem{}}, nil
	}

	movies, err := c.fetchSearchDiscoverList(ctx, "/search/movie", query, string(DiscoverMediaTypeMovie))
	if err != nil {
		return nil, err
	}
	tv, err := c.fetchSearchDiscoverList(ctx, "/search/tv", query, string(DiscoverMediaTypeTV))
	if err != nil {
		return nil, err
	}

	return &DiscoverSearchResponse{
		Movies: movies,
		TV:     tv,
	}, nil
}

func (c *TMDBClient) GetDiscoverTitleDetails(ctx context.Context, mediaType DiscoverMediaType, tmdbID int) (*DiscoverTitleDetails, error) {
	if err := c.requireTMDB(); err != nil {
		return nil, err
	}
	if tmdbID <= 0 {
		return nil, nil
	}

	switch mediaType {
	case DiscoverMediaTypeMovie:
		var payload tmdbMovieDetailResponse
		if err := c.fetchJSON(ctx, c.discoverURL(fmt.Sprintf("/movie/%d", tmdbID), map[string]string{
			"append_to_response": "videos",
		}), tmdbDiscoverDetailTTL, &payload); err != nil {
			if isTMDBDiscoverHTTPStatus(err, http.StatusNotFound) {
				return nil, nil
			}
			return nil, err
		}
		imdbID, _ := c.getMovieIMDbID(ctx, tmdbID)
		return &DiscoverTitleDetails{
			MediaType:    DiscoverMediaTypeMovie,
			TMDBID:       payload.ID,
			Title:        payload.Title,
			Overview:     payload.Overview,
			PosterPath:   payload.PosterPath,
			BackdropPath: payload.BackdropPath,
			ReleaseDate:  payload.ReleaseDate,
			VoteAverage:  payload.VoteAverage,
			IMDbID:       imdbID,
			Status:       payload.Status,
			Genres:       tmdbGenresToNames(payload.Genres),
			Runtime:      payload.Runtime,
			Videos:       tmdbVideosToDiscover(payload.Videos.Results),
		}, nil
	case DiscoverMediaTypeTV:
		var payload tmdbTVDetailResponse
		if err := c.fetchJSON(ctx, c.discoverURL(fmt.Sprintf("/tv/%d", tmdbID), map[string]string{
			"append_to_response": "videos",
		}), tmdbDiscoverDetailTTL, &payload); err != nil {
			if isTMDBDiscoverHTTPStatus(err, http.StatusNotFound) {
				return nil, nil
			}
			return nil, err
		}
		imdbID, _ := c.getTVIMDbID(ctx, tmdbID)
		return &DiscoverTitleDetails{
			MediaType:        DiscoverMediaTypeTV,
			TMDBID:           payload.ID,
			Title:            payload.Name,
			Overview:         payload.Overview,
			PosterPath:       payload.PosterPath,
			BackdropPath:     payload.BackdropPath,
			FirstAirDate:     payload.FirstAirDate,
			VoteAverage:      payload.VoteAverage,
			IMDbID:           imdbID,
			Status:           payload.Status,
			Genres:           tmdbGenresToNames(payload.Genres),
			Runtime:          firstInt(payload.EpisodeRunTime),
			NumberOfSeasons:  payload.NumberOfSeasons,
			NumberOfEpisodes: payload.NumberOfEpisodes,
			Videos:           tmdbVideosToDiscover(payload.Videos.Results),
		}, nil
	default:
		return nil, nil
	}
}

func (c *TMDBClient) requireTMDB() error {
	if c == nil || c.APIKey == "" {
		return ErrTMDBNotConfigured
	}
	return nil
}

func (c *TMDBClient) discoverURL(path string, params map[string]string) string {
	values := url.Values{}
	values.Set("api_key", c.APIKey)
	for key, value := range params {
		if value != "" {
			values.Set(key, value)
		}
	}
	return fmt.Sprintf("%s%s?%s", c.resolveBaseURL(), path, values.Encode())
}

func (c *TMDBClient) resolveBaseURL() string {
	if c != nil && c.baseURL != "" {
		return c.baseURL
	}
	return tmdbBaseURL
}

func (c *TMDBClient) fetchSearchDiscoverList(ctx context.Context, path string, query string, fallbackType string) ([]DiscoverItem, error) {
	var payload tmdbDiscoverListResponse
	if err := c.fetchJSON(ctx, c.discoverURL(path, map[string]string{
		"query": query,
	}), tmdbDiscoverShelfTTL, &payload); err != nil {
		return nil, err
	}
	return mapTMDBDiscoverItems(payload.Results, fallbackType, tmdbDiscoverHubLimit), nil
}

func (c *TMDBClient) fetchDiscoverList(
	ctx context.Context,
	path string,
	fallbackType string,
	ttl time.Duration,
	limit int,
	page int,
) ([]DiscoverItem, error) {
	var payload tmdbDiscoverListResponse
	params := map[string]string{}
	if page > 1 {
		params["page"] = fmt.Sprintf("%d", page)
	}
	if err := c.fetchJSON(ctx, c.discoverURL(path, params), ttl, &payload); err != nil {
		return nil, err
	}
	return mapTMDBDiscoverItems(payload.Results, fallbackType, limit), nil
}

func (c *TMDBClient) fetchJSON(ctx context.Context, rawURL string, ttl time.Duration, dest any) error {
	resp, err := doCachedJSONRequest(ctx, providerHTTPClient, c.cache, "tmdb", http.MethodGet, rawURL, nil, nil, ttl, 1)
	if err != nil {
		return err
	}
	return json.Unmarshal(resp.Body, dest)
}

func mapTMDBDiscoverItems(items []tmdbDiscoverListItem, fallbackType string, limit int) []DiscoverItem {
	out := make([]DiscoverItem, 0, len(items))
	for _, item := range items {
		mapped, ok := mapTMDBDiscoverItem(item, fallbackType)
		if !ok {
			continue
		}
		out = append(out, mapped)
		if limit > 0 && len(out) == limit {
			break
		}
	}
	return out
}

func mapTMDBDiscoverItem(item tmdbDiscoverListItem, fallbackType string) (DiscoverItem, bool) {
	mediaType := item.MediaType
	if mediaType == "" {
		mediaType = fallbackType
	}
	if mediaType != string(DiscoverMediaTypeMovie) && mediaType != string(DiscoverMediaTypeTV) {
		return DiscoverItem{}, false
	}

	title := item.Title
	if mediaType == string(DiscoverMediaTypeTV) && title == "" {
		title = item.Name
	}
	if title == "" {
		title = item.Name
	}
	if title == "" || item.ID <= 0 {
		return DiscoverItem{}, false
	}

	return DiscoverItem{
		MediaType:    DiscoverMediaType(mediaType),
		TMDBID:       item.ID,
		Title:        title,
		Overview:     item.Overview,
		PosterPath:   item.PosterPath,
		BackdropPath: item.BackdropPath,
		ReleaseDate:  item.ReleaseDate,
		FirstAirDate: item.FirstAirDate,
		VoteAverage:  item.VoteAverage,
	}, true
}

func interleaveDiscoverItems(primary []DiscoverItem, secondary []DiscoverItem, limit int) []DiscoverItem {
	if limit <= 0 {
		return []DiscoverItem{}
	}
	out := make([]DiscoverItem, 0, limit)
	for i := 0; len(out) < limit && (i < len(primary) || i < len(secondary)); i++ {
		if i < len(primary) {
			out = append(out, primary[i])
			if len(out) == limit {
				break
			}
		}
		if i < len(secondary) {
			out = append(out, secondary[i])
			if len(out) == limit {
				break
			}
		}
	}
	return out
}

func applyTMDBListPathToDiscoverSort(basePath string, filtered map[string]string) {
	now := time.Now().UTC()
	today := now.Format("2006-01-02")
	switch basePath {
	case "/movie/popular", "/tv/popular":
		filtered["sort_by"] = "popularity.desc"
	case "/movie/now_playing":
		filtered["sort_by"] = "primary_release_date.desc"
		filtered["primary_release_date.gte"] = now.AddDate(0, 0, -75).Format("2006-01-02")
		filtered["primary_release_date.lte"] = today
	case "/movie/upcoming":
		filtered["sort_by"] = "primary_release_date.asc"
		filtered["primary_release_date.gte"] = today
	case "/tv/on_the_air":
		filtered["sort_by"] = "popularity.desc"
	case "/movie/top_rated":
		filtered["sort_by"] = "vote_average.desc"
		filtered["vote_count.gte"] = "200"
	case "/tv/top_rated":
		filtered["sort_by"] = "vote_average.desc"
		filtered["vote_count.gte"] = "200"
	default:
		filtered["sort_by"] = "popularity.desc"
	}
}

func tmdbMediaBrowseRequest(
	mediaType DiscoverMediaType,
	basePath string,
	genreID int,
	params map[string]string,
	origin string,
) (string, string, map[string]string, DiscoverMediaType) {
	if genreID > 0 {
		filtered := cloneStringMap(params)
		filtered["with_genres"] = fmt.Sprintf("%d", genreID)
		filtered["sort_by"] = "popularity.desc"
		if o := normalizeDiscoverOrigin(origin); o != "" {
			filtered["with_origin_country"] = o
		}
		return fmt.Sprintf("/discover/%s", mediaType), string(mediaType), filtered, mediaType
	}
	oNorm := normalizeDiscoverOrigin(origin)
	if oNorm == "" {
		return basePath, string(mediaType), params, mediaType
	}
	filtered := cloneStringMap(params)
	filtered["with_origin_country"] = oNorm
	if strings.HasPrefix(basePath, "/discover/") {
		return basePath, string(mediaType), filtered, mediaType
	}
	applyTMDBListPathToDiscoverSort(basePath, filtered)
	return fmt.Sprintf("/discover/%s", mediaType), string(mediaType), filtered, mediaType
}

func tmdbBrowseRequest(
	category DiscoverBrowseCategory,
	mediaType DiscoverMediaType,
	genreID int,
	page int,
	origin string,
) (string, string, map[string]string, DiscoverMediaType) {
	params := map[string]string{
		"page": fmt.Sprintf("%d", page),
	}
	resolvedMediaType := mediaType

	switch category {
	case DiscoverBrowseCategoryTrending:
		if normalizeDiscoverOrigin(origin) != "" {
			o := normalizeDiscoverOrigin(origin)
			filtered := cloneStringMap(params)
			filtered["with_origin_country"] = o
			filtered["sort_by"] = "popularity.desc"
			if resolvedMediaType == DiscoverMediaTypeTV {
				return "/discover/tv", string(DiscoverMediaTypeTV), filtered, DiscoverMediaTypeTV
			}
			if resolvedMediaType == DiscoverMediaTypeMovie {
				return "/discover/movie", string(DiscoverMediaTypeMovie), filtered, DiscoverMediaTypeMovie
			}
			// Trending "all" cannot honor country on the mixed trending feed; use movies for stable paging.
			return "/discover/movie", string(DiscoverMediaTypeMovie), filtered, DiscoverMediaTypeMovie
		}
		if resolvedMediaType == DiscoverMediaTypeMovie {
			return "/trending/movie/day", string(DiscoverMediaTypeMovie), params, DiscoverMediaTypeMovie
		}
		if resolvedMediaType == DiscoverMediaTypeTV {
			return "/trending/tv/day", string(DiscoverMediaTypeTV), params, DiscoverMediaTypeTV
		}
		return "/trending/all/day", "", params, ""
	case DiscoverBrowseCategoryPopularMovies:
		return tmdbMediaBrowseRequest(DiscoverMediaTypeMovie, "/movie/popular", genreID, params, origin)
	case DiscoverBrowseCategoryPopularTV:
		return tmdbMediaBrowseRequest(DiscoverMediaTypeTV, "/tv/popular", genreID, params, origin)
	case DiscoverBrowseCategoryNowPlaying:
		return tmdbMediaBrowseRequest(DiscoverMediaTypeMovie, "/movie/now_playing", genreID, params, origin)
	case DiscoverBrowseCategoryUpcoming:
		return tmdbMediaBrowseRequest(DiscoverMediaTypeMovie, "/movie/upcoming", genreID, params, origin)
	case DiscoverBrowseCategoryOnTheAir:
		return tmdbMediaBrowseRequest(DiscoverMediaTypeTV, "/tv/on_the_air", genreID, params, origin)
	case DiscoverBrowseCategoryTopRated:
		if resolvedMediaType == DiscoverMediaTypeTV {
			return tmdbMediaBrowseRequest(DiscoverMediaTypeTV, "/tv/top_rated", genreID, params, origin)
		}
		return tmdbMediaBrowseRequest(DiscoverMediaTypeMovie, "/movie/top_rated", genreID, params, origin)
	default:
		if resolvedMediaType == DiscoverMediaTypeTV {
			return tmdbMediaBrowseRequest(DiscoverMediaTypeTV, "/discover/tv", genreID, params, origin)
		}
		return tmdbMediaBrowseRequest(DiscoverMediaTypeMovie, "/discover/movie", genreID, params, origin)
	}
}

func (c *TMDBClient) lookupGenre(ctx context.Context, mediaType DiscoverMediaType, genreID int) (*DiscoverGenre, error) {
	genres, err := c.GetDiscoverGenres(ctx)
	if err != nil {
		return nil, err
	}
	var source []DiscoverGenre
	if mediaType == DiscoverMediaTypeTV {
		source = genres.TVGenres
	} else {
		source = genres.MovieGenres
	}
	for _, genre := range source {
		if genre.ID == genreID {
			copyGenre := genre
			return &copyGenre, nil
		}
	}
	return &DiscoverGenre{ID: genreID}, nil
}

func mapTMDBGenres(genres []tmdbGenre) []DiscoverGenre {
	if len(genres) == 0 {
		return []DiscoverGenre{}
	}
	out := make([]DiscoverGenre, 0, len(genres))
	for _, genre := range genres {
		if genre.ID <= 0 || genre.Name == "" {
			continue
		}
		out = append(out, DiscoverGenre{ID: genre.ID, Name: genre.Name})
	}
	return out
}

func normalizePositive(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func maxInt(value int, fallback int) int {
	if value > fallback {
		return value
	}
	return fallback
}

func tmdbGenresToNames(genres []tmdbGenre) []string {
	if len(genres) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(genres))
	for _, genre := range genres {
		if genre.Name == "" {
			continue
		}
		out = append(out, genre.Name)
	}
	return out
}

func tmdbVideosToDiscover(videos []tmdbVideo) []DiscoverTitleVideo {
	if len(videos) == 0 {
		return []DiscoverTitleVideo{}
	}
	out := make([]DiscoverTitleVideo, 0, len(videos))
	for _, video := range videos {
		if video.Key == "" || video.Site == "" {
			continue
		}
		out = append(out, DiscoverTitleVideo{
			Name:     video.Name,
			Site:     video.Site,
			Key:      video.Key,
			Type:     video.Type,
			Official: video.Official,
		})
	}
	return out
}

func firstInt(values []int) int {
	if len(values) == 0 {
		return 0
	}
	return values[0]
}
