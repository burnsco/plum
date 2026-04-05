package arr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"plum/internal/db"
	"plum/internal/metadata"
)

const (
	defaultRequestTimeout = 10 * time.Second
	snapshotTTL           = 30 * time.Second
	queuePageSize         = 250
)

type RootFolderOption struct {
	Path string `json:"path"`
}

type QualityProfileOption struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type ServiceValidationResult struct {
	Configured      bool                   `json:"configured"`
	Reachable       bool                   `json:"reachable"`
	ErrorMessage    string                 `json:"errorMessage,omitempty"`
	RootFolders     []RootFolderOption     `json:"rootFolders"`
	QualityProfiles []QualityProfileOption `json:"qualityProfiles"`
}

type ValidationResult struct {
	Radarr   ServiceValidationResult `json:"radarr"`
	SonarrTV ServiceValidationResult `json:"sonarrTv"`
}

type DownloadItem struct {
	ID            string                         `json:"id"`
	Title         string                         `json:"title"`
	MediaType     metadata.DiscoverMediaType     `json:"media_type"`
	Source        metadata.MediaStackServiceKind `json:"source"`
	StatusText    string                         `json:"status_text"`
	Progress      float64                        `json:"progress,omitempty"`
	SizeLeftBytes int64                          `json:"size_left_bytes,omitempty"`
	ETASeconds    int64                          `json:"eta_seconds,omitempty"`
	ErrorMessage  string                         `json:"error_message,omitempty"`
}

type DownloadsResponse struct {
	Configured bool           `json:"configured"`
	Items      []DownloadItem `json:"items"`
}

type Service struct {
	HTTPClient *http.Client
	Now        func() time.Time

	mu     sync.RWMutex
	cached *cachedSnapshot
}

type cachedSnapshot struct {
	key       string
	fetchedAt time.Time
	snapshot  *Snapshot
}

type Snapshot struct {
	RadarrConfigured   bool
	SonarrTVConfigured bool

	radarrMoviesByTMDB    map[int]catalogItem
	sonarrSeriesByTMDB    map[int]catalogItem
	radarrDownloadsByTMDB map[int]DownloadItem
	sonarrDownloadsByTMDB map[int]DownloadItem
	downloads             []DownloadItem
}

type catalogItem struct {
	ID     int
	TMDBID int
	Title  string
}

type serviceOptions struct {
	rootFolders     []RootFolderOption
	qualityProfiles []QualityProfileOption
}

type movieRecord struct {
	ID     int    `json:"id"`
	TMDBID int    `json:"tmdbId"`
	Title  string `json:"title"`
}

type seriesRecord struct {
	ID     int    `json:"id"`
	TMDBID int    `json:"tmdbId"`
	Title  string `json:"title"`
}

type queueMessage struct {
	Title    string   `json:"title"`
	Messages []string `json:"messages"`
}

type queueRecord struct {
	ID                      int            `json:"id"`
	MovieID                 int            `json:"movieId"`
	SeriesID                int            `json:"seriesId"`
	Title                   string         `json:"title"`
	Status                  string         `json:"status"`
	TrackedDownloadStatus   string         `json:"trackedDownloadStatus"`
	TrackedDownloadState    string         `json:"trackedDownloadState"`
	StatusMessages          []queueMessage `json:"statusMessages"`
	Size                    float64        `json:"size"`
	SizeLeft                float64        `json:"sizeleft"`
	EstimatedCompletionTime string         `json:"estimatedCompletionTime"`
	TimeLeft                string         `json:"timeleft"`
}

type queuePage struct {
	Page         int           `json:"page"`
	PageSize     int           `json:"pageSize"`
	TotalRecords int           `json:"totalRecords"`
	Records      []queueRecord `json:"records"`
}

func NewService() *Service {
	return &Service{
		HTTPClient: &http.Client{Timeout: defaultRequestTimeout},
		Now:        time.Now,
	}
}

func IsConfigured(settings db.MediaStackServiceSettings) bool {
	settings = normalizeServiceSettings(settings)
	return settings.BaseURL != "" && settings.APIKey != ""
}

func normalizeServiceSettings(settings db.MediaStackServiceSettings) db.MediaStackServiceSettings {
	settings.BaseURL = strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/")
	settings.APIKey = strings.TrimSpace(settings.APIKey)
	settings.RootFolderPath = strings.TrimSpace(settings.RootFolderPath)
	return settings
}

func (s *Service) client() *http.Client {
	if s != nil && s.HTTPClient != nil {
		return s.HTTPClient
	}
	return &http.Client{Timeout: defaultRequestTimeout}
}

func (s *Service) now() time.Time {
	if s != nil && s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) Invalidate() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.cached = nil
	s.mu.Unlock()
}

func (s *Service) Validate(ctx context.Context, settings db.MediaStackSettings) (ValidationResult, error) {
	settings = db.NormalizeMediaStackSettings(settings)
	return ValidationResult{
		Radarr:   s.validateService(ctx, settings.Radarr),
		SonarrTV: s.validateService(ctx, settings.SonarrTV),
	}, nil
}

func (s *Service) validateService(ctx context.Context, settings db.MediaStackServiceSettings) ServiceValidationResult {
	settings = normalizeServiceSettings(settings)
	if !IsConfigured(settings) {
		return ServiceValidationResult{
			Configured:      false,
			Reachable:       false,
			RootFolders:     []RootFolderOption{},
			QualityProfiles: []QualityProfileOption{},
		}
	}

	options, err := s.fetchServiceOptions(ctx, settings)
	if err != nil {
		return ServiceValidationResult{
			Configured:      true,
			Reachable:       false,
			ErrorMessage:    err.Error(),
			RootFolders:     []RootFolderOption{},
			QualityProfiles: []QualityProfileOption{},
		}
	}
	return ServiceValidationResult{
		Configured:      true,
		Reachable:       true,
		RootFolders:     options.rootFolders,
		QualityProfiles: options.qualityProfiles,
	}
}

func (s *Service) fetchServiceOptions(ctx context.Context, settings db.MediaStackServiceSettings) (serviceOptions, error) {
	var rootFolders []RootFolderOption
	if err := s.getJSON(ctx, settings, "/api/v3/rootfolder", &rootFolders); err != nil {
		return serviceOptions{}, err
	}
	var qualityProfiles []QualityProfileOption
	if err := s.getJSON(ctx, settings, "/api/v3/qualityprofile", &qualityProfiles); err != nil {
		return serviceOptions{}, err
	}
	sort.Slice(rootFolders, func(i, j int) bool {
		return rootFolders[i].Path < rootFolders[j].Path
	})
	sort.Slice(qualityProfiles, func(i, j int) bool {
		if qualityProfiles[i].Name == qualityProfiles[j].Name {
			return qualityProfiles[i].ID < qualityProfiles[j].ID
		}
		return qualityProfiles[i].Name < qualityProfiles[j].Name
	})
	return serviceOptions{
		rootFolders:     rootFolders,
		qualityProfiles: qualityProfiles,
	}, nil
}

func (s *Service) LoadSnapshot(ctx context.Context, settings db.MediaStackSettings) (*Snapshot, error) {
	settings = db.NormalizeMediaStackSettings(settings)
	cacheKey := snapshotKey(settings)
	now := s.now()

	s.mu.RLock()
	cached := s.cached
	s.mu.RUnlock()
	if cached != nil && cached.key == cacheKey && now.Sub(cached.fetchedAt) < snapshotTTL {
		return cached.snapshot, nil
	}

	snapshot := &Snapshot{
		RadarrConfigured:      IsConfigured(settings.Radarr),
		SonarrTVConfigured:    IsConfigured(settings.SonarrTV),
		radarrMoviesByTMDB:    make(map[int]catalogItem),
		sonarrSeriesByTMDB:    make(map[int]catalogItem),
		radarrDownloadsByTMDB: make(map[int]DownloadItem),
		sonarrDownloadsByTMDB: make(map[int]DownloadItem),
	}

	var errs []string
	if snapshot.RadarrConfigured {
		if err := s.loadRadarrSnapshot(ctx, settings.Radarr, snapshot); err != nil {
			errs = append(errs, "radarr: "+err.Error())
		}
	}
	if snapshot.SonarrTVConfigured {
		if err := s.loadSonarrSnapshot(ctx, settings.SonarrTV, snapshot); err != nil {
			errs = append(errs, "sonarr-tv: "+err.Error())
		}
	}

	sort.Slice(snapshot.downloads, func(i, j int) bool {
		if snapshot.downloads[i].Source == snapshot.downloads[j].Source {
			return snapshot.downloads[i].Title < snapshot.downloads[j].Title
		}
		return snapshot.downloads[i].Source < snapshot.downloads[j].Source
	})

	if len(errs) == 0 {
		s.mu.Lock()
		s.cached = &cachedSnapshot{
			key:       cacheKey,
			fetchedAt: now,
			snapshot:  snapshot,
		}
		s.mu.Unlock()
		return snapshot, nil
	}
	return snapshot, errors.New(strings.Join(errs, "; "))
}

func (s *Service) GetDownloads(ctx context.Context, settings db.MediaStackSettings) (DownloadsResponse, error) {
	snapshot, err := s.LoadSnapshot(ctx, settings)
	if snapshot == nil {
		snapshot = &Snapshot{
			RadarrConfigured:   IsConfigured(settings.Radarr),
			SonarrTVConfigured: IsConfigured(settings.SonarrTV),
		}
	}
	return DownloadsResponse{
		Configured: snapshot.RadarrConfigured || snapshot.SonarrTVConfigured,
		Items:      append([]DownloadItem(nil), snapshot.downloads...),
	}, err
}

func (s *Service) loadRadarrSnapshot(ctx context.Context, settings db.MediaStackServiceSettings, snapshot *Snapshot) error {
	var movies []movieRecord
	if err := s.getJSON(ctx, settings, "/api/v3/movie", &movies); err != nil {
		return err
	}
	moviesByID := make(map[int]catalogItem, len(movies))
	for _, movie := range movies {
		if movie.TMDBID <= 0 {
			continue
		}
		item := catalogItem{ID: movie.ID, TMDBID: movie.TMDBID, Title: strings.TrimSpace(movie.Title)}
		snapshot.radarrMoviesByTMDB[movie.TMDBID] = item
		moviesByID[movie.ID] = item
	}

	queue, err := s.fetchQueue(ctx, settings)
	if err != nil {
		return err
	}
	for _, record := range queue {
		item, ok := moviesByID[record.MovieID]
		if !ok || item.TMDBID <= 0 {
			continue
		}
		download := queueRecordToDownloadItem(s.now(), metadata.DiscoverMediaTypeMovie, metadata.MediaStackServiceRadarr, record)
		if download.Title == "" {
			download.Title = item.Title
		}
		snapshot.radarrDownloadsByTMDB[item.TMDBID] = download
		snapshot.downloads = append(snapshot.downloads, download)
	}
	return nil
}

func (s *Service) loadSonarrSnapshot(ctx context.Context, settings db.MediaStackServiceSettings, snapshot *Snapshot) error {
	var series []seriesRecord
	if err := s.getJSON(ctx, settings, "/api/v3/series", &series); err != nil {
		return err
	}
	seriesByID := make(map[int]catalogItem, len(series))
	for _, show := range series {
		if show.TMDBID <= 0 {
			continue
		}
		item := catalogItem{ID: show.ID, TMDBID: show.TMDBID, Title: strings.TrimSpace(show.Title)}
		snapshot.sonarrSeriesByTMDB[show.TMDBID] = item
		seriesByID[show.ID] = item
	}

	queue, err := s.fetchQueue(ctx, settings)
	if err != nil {
		return err
	}
	for _, record := range queue {
		item, ok := seriesByID[record.SeriesID]
		if !ok || item.TMDBID <= 0 {
			continue
		}
		download := queueRecordToDownloadItem(s.now(), metadata.DiscoverMediaTypeTV, metadata.MediaStackServiceSonarrTV, record)
		if download.Title == "" {
			download.Title = item.Title
		}
		snapshot.sonarrDownloadsByTMDB[item.TMDBID] = download
		snapshot.downloads = append(snapshot.downloads, download)
	}
	return nil
}

func (s *Service) fetchQueue(ctx context.Context, settings db.MediaStackServiceSettings) ([]queueRecord, error) {
	var all []queueRecord
	page := 1
	for {
		values := url.Values{}
		values.Set("page", strconv.Itoa(page))
		values.Set("pageSize", strconv.Itoa(queuePageSize))
		values.Set("includeUnknownSeriesItems", "true")
		values.Set("includeMovie", "true")
		values.Set("includeSeries", "true")

		var payload queuePage
		path := "/api/v3/queue?" + values.Encode()
		if err := s.getJSON(ctx, settings, path, &payload); err != nil {
			return nil, err
		}
		all = append(all, payload.Records...)
		if payload.PageSize <= 0 || len(all) >= payload.TotalRecords || len(payload.Records) == 0 {
			break
		}
		page++
	}
	return all, nil
}

func (s *Service) ResolveDiscoverAcquisition(
	mediaType metadata.DiscoverMediaType,
	tmdbID int,
	inLibrary bool,
	settings db.MediaStackSettings,
	snapshot *Snapshot,
) *metadata.DiscoverAcquisition {
	source, configured := sourceForMediaType(mediaType, settings)
	acquisition := &metadata.DiscoverAcquisition{
		State:        metadata.DiscoverAcquisitionStateNotAdded,
		Source:       source,
		IsConfigured: configured,
	}

	if inLibrary {
		acquisition.State = metadata.DiscoverAcquisitionStateAvailable
		return acquisition
	}

	if snapshot != nil {
		switch mediaType {
		case metadata.DiscoverMediaTypeMovie:
			if _, ok := snapshot.radarrDownloadsByTMDB[tmdbID]; ok {
				acquisition.State = metadata.DiscoverAcquisitionStateDownloading
			} else if _, ok := snapshot.radarrMoviesByTMDB[tmdbID]; ok {
				acquisition.State = metadata.DiscoverAcquisitionStateAdded
			}
		case metadata.DiscoverMediaTypeTV:
			if _, ok := snapshot.sonarrDownloadsByTMDB[tmdbID]; ok {
				acquisition.State = metadata.DiscoverAcquisitionStateDownloading
			} else if _, ok := snapshot.sonarrSeriesByTMDB[tmdbID]; ok {
				acquisition.State = metadata.DiscoverAcquisitionStateAdded
			}
		}
	}

	acquisition.CanAdd = configured && acquisition.State == metadata.DiscoverAcquisitionStateNotAdded
	return acquisition
}

func sourceForMediaType(
	mediaType metadata.DiscoverMediaType,
	settings db.MediaStackSettings,
) (metadata.MediaStackServiceKind, bool) {
	switch mediaType {
	case metadata.DiscoverMediaTypeMovie:
		return metadata.MediaStackServiceRadarr, IsConfigured(settings.Radarr)
	case metadata.DiscoverMediaTypeTV:
		return metadata.MediaStackServiceSonarrTV, IsConfigured(settings.SonarrTV)
	default:
		return "", false
	}
}

func (s *Service) AddMovie(ctx context.Context, settings db.MediaStackServiceSettings, tmdbID int) error {
	settings = normalizeServiceSettings(settings)
	if !IsConfigured(settings) {
		return errors.New("radarr is not configured")
	}

	var payload map[string]any
	if err := s.getJSON(ctx, settings, fmt.Sprintf("/api/v3/movie/lookup/tmdb?tmdbId=%d", tmdbID), &payload); err != nil {
		return err
	}
	payload["qualityProfileId"] = settings.QualityProfileID
	payload["rootFolderPath"] = settings.RootFolderPath
	payload["monitored"] = true
	payload["addOptions"] = map[string]any{
		"searchForMovie": true,
	}
	return s.postJSON(ctx, settings, "/api/v3/movie", payload, nil)
}

func (s *Service) AddSeries(ctx context.Context, settings db.MediaStackServiceSettings, tvdbID string) error {
	settings = normalizeServiceSettings(settings)
	if !IsConfigured(settings) {
		return errors.New("sonarr-tv is not configured")
	}
	if strings.TrimSpace(tvdbID) == "" {
		return errors.New("tvdb id is required to add a series")
	}

	var payload []map[string]any
	if err := s.getJSON(ctx, settings, "/api/v3/series/lookup?term="+url.QueryEscape("tvdb:"+tvdbID), &payload); err != nil {
		return err
	}
	if len(payload) == 0 {
		return errors.New("series lookup returned no matches")
	}
	request := payload[0]
	request["qualityProfileId"] = settings.QualityProfileID
	request["rootFolderPath"] = settings.RootFolderPath
	request["seasonFolder"] = true
	request["monitored"] = true
	request["addOptions"] = map[string]any{
		"monitor":                      "all",
		"searchForMissingEpisodes":     true,
		"searchForCutoffUnmetEpisodes": true,
	}
	return s.postJSON(ctx, settings, "/api/v3/series", request, nil)
}

func (s *Service) getJSON(ctx context.Context, settings db.MediaStackServiceSettings, path string, out any) error {
	return s.doJSON(ctx, http.MethodGet, settings, path, nil, out)
}

func (s *Service) postJSON(ctx context.Context, settings db.MediaStackServiceSettings, path string, body any, out any) error {
	return s.doJSON(ctx, http.MethodPost, settings, path, body, out)
}

func (s *Service) deleteJSON(ctx context.Context, settings db.MediaStackServiceSettings, path string) error {
	return s.doJSON(ctx, http.MethodDelete, settings, path, nil, nil)
}

// RemoveQueueItem deletes a Radarr or Sonarr queue record. compoundID is "radarr:<queueId>" or "sonarr-tv:<queueId>".
func (s *Service) RemoveQueueItem(ctx context.Context, settings db.MediaStackSettings, compoundID string) error {
	if s == nil {
		return errors.New("arr service unavailable")
	}
	settings = db.NormalizeMediaStackSettings(settings)
	parts := strings.SplitN(strings.TrimSpace(compoundID), ":", 2)
	if len(parts) != 2 {
		return errors.New("invalid download id")
	}
	source := metadata.MediaStackServiceKind(parts[0])
	queueID, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || queueID <= 0 {
		return errors.New("invalid download id")
	}
	var svc db.MediaStackServiceSettings
	switch source {
	case metadata.MediaStackServiceRadarr:
		if !IsConfigured(settings.Radarr) {
			return errors.New("radarr is not configured")
		}
		svc = settings.Radarr
	case metadata.MediaStackServiceSonarrTV:
		if !IsConfigured(settings.SonarrTV) {
			return errors.New("sonarr-tv is not configured")
		}
		svc = settings.SonarrTV
	default:
		return errors.New("unknown download source")
	}
	svc = normalizeServiceSettings(svc)
	q := url.Values{}
	q.Set("removeFromClient", "false")
	q.Set("blocklist", "false")
	q.Set("skipRedownload", "true")
	path := fmt.Sprintf("/api/v3/queue/%d?%s", queueID, q.Encode())
	return s.deleteJSON(ctx, svc, path)
}

func (s *Service) doJSON(
	ctx context.Context,
	method string,
	settings db.MediaStackServiceSettings,
	path string,
	body any,
	out any,
) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, settings.BaseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("X-Api-Key", settings.APIKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		message := strings.TrimSpace(string(data))
		if message == "" {
			message = resp.Status
		}
		return fmt.Errorf("%s %s: %s", method, path, message)
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func snapshotKey(settings db.MediaStackSettings) string {
	raw, _ := json.Marshal(settings)
	return string(raw)
}

func queueRecordToDownloadItem(
	now time.Time,
	mediaType metadata.DiscoverMediaType,
	source metadata.MediaStackServiceKind,
	record queueRecord,
) DownloadItem {
	item := DownloadItem{
		ID:            fmt.Sprintf("%s:%d", source, record.ID),
		Title:         strings.TrimSpace(record.Title),
		MediaType:     mediaType,
		Source:        source,
		StatusText:    normalizeStatusLabel(record.TrackedDownloadStatus, record.TrackedDownloadState, record.Status),
		Progress:      queueProgress(record.Size, record.SizeLeft),
		SizeLeftBytes: int64(record.SizeLeft),
		ETASeconds:    estimateETASeconds(now, record.EstimatedCompletionTime, record.TimeLeft),
		ErrorMessage:  queueErrorMessage(record.StatusMessages),
	}
	return item
}

func normalizeStatusLabel(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		normalized := strings.NewReplacer("_", " ", "-", " ").Replace(value)
		parts := strings.Fields(normalized)
		for i := range parts {
			if parts[i] == "" {
				continue
			}
			parts[i] = strings.ToUpper(parts[i][:1]) + strings.ToLower(parts[i][1:])
		}
		return strings.Join(parts, " ")
	}
	return "Queued"
}

func queueProgress(size, sizeLeft float64) float64 {
	if size <= 0 {
		return 0
	}
	progress := ((size - sizeLeft) / size) * 100
	if progress < 0 {
		return 0
	}
	if progress > 100 {
		return 100
	}
	return progress
}

func estimateETASeconds(now time.Time, estimatedCompletionTime, timeLeft string) int64 {
	if estimatedCompletionTime = strings.TrimSpace(estimatedCompletionTime); estimatedCompletionTime != "" {
		if parsed, err := time.Parse(time.RFC3339, estimatedCompletionTime); err == nil {
			seconds := int64(parsed.Sub(now).Seconds())
			if seconds > 0 {
				return seconds
			}
		}
	}
	if timeLeft = strings.TrimSpace(timeLeft); timeLeft != "" {
		parts := strings.Split(timeLeft, ":")
		if len(parts) == 3 {
			hours, errH := strconv.Atoi(parts[0])
			minutes, errM := strconv.Atoi(parts[1])
			seconds, errS := strconv.Atoi(parts[2])
			if errH == nil && errM == nil && errS == nil {
				total := int64(hours*3600 + minutes*60 + seconds)
				if total > 0 {
					return total
				}
			}
		}
	}
	return 0
}

func queueErrorMessage(messages []queueMessage) string {
	for _, message := range messages {
		if title := strings.TrimSpace(message.Title); title != "" {
			return title
		}
		for _, value := range message.Messages {
			if value = strings.TrimSpace(value); value != "" {
				return value
			}
		}
	}
	return ""
}
