package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const fanartBaseURL = "https://webservice.fanart.tv/v3"

type FanartClient struct {
	APIKey string
	cache  ProviderCache
}

func NewFanartClient(apiKey string) *FanartClient {
	if apiKey == "" {
		return nil
	}
	return &FanartClient{APIKey: apiKey}
}

func (c *FanartClient) SetCache(cache ProviderCache) {
	if c == nil {
		return
	}
	c.cache = cache
}

type fanartImage struct {
	URL    string `json:"url"`
	Season string `json:"season"`
}

type fanartMovieResponse struct {
	MoviePoster []fanartImage `json:"movieposter"`
}

type fanartTVResponse struct {
	TVPoster     []fanartImage `json:"tvposter"`
	SeasonPoster []fanartImage `json:"seasonposter"`
}

func fanartImageURLs(images []fanartImage) []string {
	urls := make([]string, 0, len(images))
	for _, image := range images {
		if image.URL != "" {
			urls = append(urls, image.URL)
		}
	}
	return urls
}

func (c *FanartClient) moviePosters(ctx context.Context, tmdbID int) ([]string, error) {
	if c == nil || c.APIKey == "" || tmdbID <= 0 {
		return nil, nil
	}
	rawURL := fmt.Sprintf("%s/movies/%d?api_key=%s&client_key=%s", fanartBaseURL, tmdbID, c.APIKey, c.APIKey)
	resp, err := doCachedJSONRequest(ctx, providerHTTPClient, c.cache, "fanart", http.MethodGet, rawURL, nil, nil, 7*24*time.Hour, 1)
	if err != nil {
		return nil, err
	}
	var payload fanartMovieResponse
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return nil, err
	}
	return fanartImageURLs(payload.MoviePoster), nil
}

func (c *FanartClient) moviePoster(ctx context.Context, tmdbID int) (string, error) {
	posters, err := c.moviePosters(ctx, tmdbID)
	if err != nil {
		return "", err
	}
	for _, poster := range posters {
		return poster, nil
	}
	return "", nil
}

func (c *FanartClient) showPosters(ctx context.Context, tvdbID string) ([]string, error) {
	if c == nil || c.APIKey == "" || tvdbID == "" {
		return nil, nil
	}
	rawURL := fmt.Sprintf("%s/tv/%s?api_key=%s&client_key=%s", fanartBaseURL, tvdbID, c.APIKey, c.APIKey)
	resp, err := doCachedJSONRequest(ctx, providerHTTPClient, c.cache, "fanart", http.MethodGet, rawURL, nil, nil, 7*24*time.Hour, 1)
	if err != nil {
		return nil, err
	}
	var payload fanartTVResponse
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return nil, err
	}
	return fanartImageURLs(payload.TVPoster), nil
}

func (c *FanartClient) showPoster(ctx context.Context, tvdbID string) (string, error) {
	posters, err := c.showPosters(ctx, tvdbID)
	if err != nil {
		return "", err
	}
	for _, poster := range posters {
		return poster, nil
	}
	return "", nil
}

func (c *FanartClient) seasonPoster(ctx context.Context, tvdbID string, season int) (string, error) {
	if c == nil || c.APIKey == "" || tvdbID == "" {
		return "", nil
	}
	rawURL := fmt.Sprintf("%s/tv/%s?api_key=%s&client_key=%s", fanartBaseURL, tvdbID, c.APIKey, c.APIKey)
	resp, err := doCachedJSONRequest(ctx, providerHTTPClient, c.cache, "fanart", http.MethodGet, rawURL, nil, nil, 7*24*time.Hour, 1)
	if err != nil {
		return "", err
	}
	var payload fanartTVResponse
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return "", err
	}
	target := fmt.Sprintf("%d", season)
	for _, image := range payload.SeasonPoster {
		if image.URL == "" {
			continue
		}
		if image.Season == target {
			return image.URL, nil
		}
	}
	for _, image := range payload.SeasonPoster {
		if image.URL != "" {
			return image.URL, nil
		}
	}
	return "", nil
}
