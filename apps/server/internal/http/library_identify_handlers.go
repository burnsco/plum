package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"plum/internal/db"
	"plum/internal/metadata"
)

type scanResult = db.ScanResult

type identifyResult struct {
	Identified int `json:"identified"`
	Failed     int `json:"failed"`
}

var (
	identifyInitialTimeout   = 8 * time.Second
	identifyRetryTimeout     = 45 * time.Second
	identifyMovieWorkers     = 2
	identifyMovieRateLimit   = 250 * time.Millisecond
	identifyMovieRateBurst   = 2
	identifyEpisodeWorkers   = 4
	identifyEpisodeRateLimit = 100 * time.Millisecond
	identifyEpisodeRateBurst = 4
)

type identifyJob struct {
	row     db.IdentificationRow
	attempt int
}

type identifyJobStatus string

const (
	identifyJobSucceeded identifyJobStatus = "succeeded"
	identifyJobRetry     identifyJobStatus = "retry"
	identifyJobFailed    identifyJobStatus = "failed"
)

type identifyJobResult struct {
	status           identifyJobStatus
	job              identifyJob
	fallbackEligible bool
}

type identifyRunConfig struct {
	workers      int
	rateInterval time.Duration
	rateBurst    int
}

type episodeIdentifyGroup struct {
	key             string
	kind            string
	groupQuery      string
	fallbackQueries []string
	explicitTMDBID  int
	explicitTVDBID  string
	attempt         int
	representative  db.EpisodeIdentifyRow
	rows            []db.EpisodeIdentifyRow
}

type episodeGroupJob struct {
	group   episodeIdentifyGroup
	attempt int
}

type episodeGroupResult struct {
	group      episodeIdentifyGroup
	identified int
	retry      bool
	failed     []identifyJobResult
}

type episodeSearchCache struct {
	mu    sync.Mutex
	calls map[string]*episodeSearchCall
}

type episodeSearchCall struct {
	done    chan struct{}
	results []metadata.MatchResult
	err     error
}

type episodeSeriesDetailsCache struct {
	mu    sync.Mutex
	calls map[int]*episodeSeriesDetailsCall
}

type episodeSeriesDetailsCall struct {
	done    chan struct{}
	details *metadata.SeriesDetails
	err     error
}

type episodeLookupCache struct {
	mu    sync.Mutex
	calls map[string]*episodeLookupCall
}

type episodeLookupCall struct {
	done  chan struct{}
	match *metadata.MatchResult
	err   error
}

type episodicIdentifyCache struct {
	handler      *LibraryHandler
	searchCache  *episodeSearchCache
	detailsCache *episodeSeriesDetailsCache
	episodeCache *episodeLookupCache
}

type movieIdentifyCall struct {
	done chan struct{}
	res  *metadata.MatchResult
	err  error
}

type movieIdentifyCache struct {
	mu    sync.Mutex
	calls map[string]*movieIdentifyCall
}

func newMovieIdentifyCache() *movieIdentifyCache {
	return &movieIdentifyCache{calls: make(map[string]*movieIdentifyCall)}
}

func (c *movieIdentifyCache) lookupOrRun(key string, fn func() (*metadata.MatchResult, error)) (*metadata.MatchResult, error) {
	if c == nil || key == "" {
		return fn()
	}

	c.mu.Lock()
	if call, ok := c.calls[key]; ok {
		c.mu.Unlock()
		<-call.done
		return call.res, call.err
	}
	call := &movieIdentifyCall{done: make(chan struct{})}
	c.calls[key] = call
	c.mu.Unlock()

	call.res, call.err = fn()
	close(call.done)

	c.mu.Lock()
	if call.res == nil || call.err != nil {
		delete(c.calls, key)
	}
	c.mu.Unlock()
	return call.res, call.err
}

func newEpisodicIdentifyCache(handler *LibraryHandler) *episodicIdentifyCache {
	return &episodicIdentifyCache{
		handler: handler,
		searchCache: &episodeSearchCache{
			calls: make(map[string]*episodeSearchCall),
		},
		detailsCache: &episodeSeriesDetailsCache{
			calls: make(map[int]*episodeSeriesDetailsCall),
		},
		episodeCache: &episodeLookupCache{
			calls: make(map[string]*episodeLookupCall),
		},
	}
}

func (c *episodeSearchCache) lookupOrRun(key string, fn func() ([]metadata.MatchResult, error)) ([]metadata.MatchResult, error) {
	if c == nil || key == "" {
		return fn()
	}
	c.mu.Lock()
	if call, ok := c.calls[key]; ok {
		c.mu.Unlock()
		<-call.done
		return append([]metadata.MatchResult(nil), call.results...), call.err
	}
	call := &episodeSearchCall{done: make(chan struct{})}
	c.calls[key] = call
	c.mu.Unlock()

	call.results, call.err = fn()
	close(call.done)

	c.mu.Lock()
	if call.err != nil || len(call.results) == 0 {
		delete(c.calls, key)
	}
	c.mu.Unlock()

	return append([]metadata.MatchResult(nil), call.results...), call.err
}

func (c *episodeSeriesDetailsCache) lookupOrRun(key int, fn func() (*metadata.SeriesDetails, error)) (*metadata.SeriesDetails, error) {
	if c == nil || key <= 0 {
		return fn()
	}
	c.mu.Lock()
	if call, ok := c.calls[key]; ok {
		c.mu.Unlock()
		<-call.done
		return call.details, call.err
	}
	call := &episodeSeriesDetailsCall{done: make(chan struct{})}
	c.calls[key] = call
	c.mu.Unlock()

	call.details, call.err = fn()
	close(call.done)

	c.mu.Lock()
	if call.err != nil || call.details == nil {
		delete(c.calls, key)
	}
	c.mu.Unlock()

	return call.details, call.err
}

func (c *episodeLookupCache) lookupOrRun(key string, fn func() (*metadata.MatchResult, error)) (*metadata.MatchResult, error) {
	if c == nil || key == "" {
		return fn()
	}
	c.mu.Lock()
	if call, ok := c.calls[key]; ok {
		c.mu.Unlock()
		<-call.done
		return call.match, call.err
	}
	call := &episodeLookupCall{done: make(chan struct{})}
	c.calls[key] = call
	c.mu.Unlock()

	call.match, call.err = fn()
	close(call.done)

	c.mu.Lock()
	if call.err != nil || call.match == nil {
		delete(c.calls, key)
	}
	c.mu.Unlock()

	return call.match, call.err
}

func (c *episodicIdentifyCache) SearchTV(ctx context.Context, query string) ([]metadata.MatchResult, error) {
	if c == nil || c.handler == nil || c.handler.SeriesQuery == nil {
		return nil, nil
	}
	return c.searchCache.lookupOrRun(strings.ToLower(strings.TrimSpace(query)), func() ([]metadata.MatchResult, error) {
		return c.handler.SeriesQuery.SearchTV(ctx, query)
	})
}

func (c *episodicIdentifyCache) GetSeriesDetails(ctx context.Context, tmdbID int) (*metadata.SeriesDetails, error) {
	if c == nil || c.handler == nil || c.handler.Series == nil {
		return nil, nil
	}
	return c.detailsCache.lookupOrRun(tmdbID, func() (*metadata.SeriesDetails, error) {
		return c.handler.Series.GetSeriesDetails(ctx, tmdbID)
	})
}

func (c *episodicIdentifyCache) GetEpisode(ctx context.Context, provider, seriesID string, season, episode int) (*metadata.MatchResult, error) {
	if c == nil || c.handler == nil || c.handler.SeriesQuery == nil {
		return nil, nil
	}
	key := provider + ":" + seriesID + ":" + strconv.Itoa(season) + ":" + strconv.Itoa(episode)
	return c.episodeCache.lookupOrRun(key, func() (*metadata.MatchResult, error) {
		return c.handler.SeriesQuery.GetEpisode(ctx, provider, seriesID, season, episode)
	})
}

func identifyConfigForKind(kind string) identifyRunConfig {
	if kind == db.LibraryTypeMovie {
		return identifyRunConfig{
			workers:      identifyMovieWorkers,
			rateInterval: identifyMovieRateLimit,
			rateBurst:    identifyMovieRateBurst,
		}
	}
	return identifyRunConfig{
		workers:      identifyEpisodeWorkers,
		rateInterval: identifyEpisodeRateLimit,
		rateBurst:    identifyEpisodeRateBurst,
	}
}

func identificationRowsFromEpisodeRows(rows []db.EpisodeIdentifyRow) []db.IdentificationRow {
	out := make([]db.IdentificationRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.IdentificationRow)
	}
	return out
}

func (h *LibraryHandler) IdentifyLibrary(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	idStr := chi.URLParam(r, "id")
	var libraryID, ownerID int
	err := h.DB.QueryRow(`SELECT id, user_id FROM libraries WHERE id = ?`, idStr).Scan(&libraryID, &ownerID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if ownerID != u.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	result, err := h.identifyLibrary(r.Context(), libraryID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *LibraryHandler) identifyLibrary(ctx context.Context, libraryID int) (identifyResult, error) {
	if h.Meta == nil {
		return identifyResult{Identified: 0, Failed: 0}, nil
	}
	var libraryPath string
	_ = h.DB.QueryRow(`SELECT path FROM libraries WHERE id = ?`, libraryID).Scan(&libraryPath)
	var libraryType string
	if err := h.DB.QueryRow(`SELECT type FROM libraries WHERE id = ?`, libraryID).Scan(&libraryType); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.getIdentifyRun().clearLibrary(libraryID)
			return identifyResult{Identified: 0, Failed: 0}, nil
		}
		return identifyResult{}, err
	}

	if libraryType == db.LibraryTypeTV || libraryType == db.LibraryTypeAnime {
		rows, err := db.ListEpisodeIdentifyRowsByLibrary(h.DB, libraryID)
		if err != nil {
			return identifyResult{}, err
		}
		if len(rows) == 0 {
			h.getIdentifyRun().clearLibrary(libraryID)
			return identifyResult{Identified: 0, Failed: 0}, nil
		}
		trackedRows, refreshOnlyRows := splitEpisodeIdentifyRows(rows)
		if len(trackedRows) > 0 {
			t := h.ensureIdentifyRun()
			t.startLibrary(libraryID, identificationRowsFromEpisodeRows(trackedRows))
			defer t.finishLibrary(libraryID)
		} else {
			h.getIdentifyRun().clearLibrary(libraryID)
		}
		slog.Info("identify episode rows",
			"library_id", libraryID,
			"type", libraryType,
			"tracked_rows", len(trackedRows),
			"refresh_rows", len(refreshOnlyRows),
		)
		identified, failed, err := h.identifyEpisodeRowsWithRefresh(ctx, libraryID, libraryPath, libraryType, trackedRows, refreshOnlyRows)
		if err != nil {
			return identifyResult{}, err
		}
		if identified > 0 && h.SearchIndex != nil {
			h.SearchIndex.Queue(libraryID, false)
		}
		return identifyResult{Identified: identified, Failed: failed}, nil
	}

	rows, err := db.ListIdentifiableByLibrary(h.DB, libraryID)
	if err != nil {
		return identifyResult{}, err
	}
	if len(rows) == 0 {
		h.getIdentifyRun().clearLibrary(libraryID)
		return identifyResult{Identified: 0, Failed: 0}, nil
	}
	trackedRows, refreshOnlyRows := splitIdentifyRows(rows)
	if len(trackedRows) > 0 {
		t := h.ensureIdentifyRun()
		t.startLibrary(libraryID, trackedRows)
		defer t.finishLibrary(libraryID)
	} else {
		h.getIdentifyRun().clearLibrary(libraryID)
	}
	slog.Info("identify rows",
		"library_id", libraryID,
		"type", libraryType,
		"tracked_rows", len(trackedRows),
		"refresh_rows", len(refreshOnlyRows),
	)

	identified, failed := 0, 0
	initialJobs := make([]identifyJob, 0, len(trackedRows))
	for _, row := range trackedRows {
		initialJobs = append(initialJobs, identifyJob{row: row})
	}
	sortIdentifyJobs(initialJobs, libraryPath)
	var movieCache *movieIdentifyCache
	if len(rows) > 0 && rows[0].Kind == db.LibraryTypeMovie {
		movieCache = newMovieIdentifyCache()
	}
	initialIdentified, retryJobs, initialFailed := h.runIdentifyJobs(ctx, libraryID, libraryPath, initialJobs, movieCache)
	retryIdentified, _, retryFailed := h.runIdentifyJobs(ctx, libraryID, libraryPath, retryJobs, movieCache)
	identified += initialIdentified + retryIdentified

	fallbackIdentified, fallbackFailed := h.identifyShowFallbacks(ctx, libraryID, libraryPath, append(initialFailed, retryFailed...), nil, false)
	identified += fallbackIdentified
	failed += fallbackFailed
	if len(refreshOnlyRows) > 0 {
		refreshIdentified, refreshErr := h.refreshMatchedRows(ctx, libraryID, libraryPath, refreshOnlyRows, movieCache)
		if refreshErr != nil {
			slog.Warn("identify refresh-only rows failed", "library_id", libraryID, "type", libraryType, "error", refreshErr)
		}
		identified += refreshIdentified
	}

	if identified > 0 && h.SearchIndex != nil {
		h.SearchIndex.Queue(libraryID, false)
	}
	return identifyResult{Identified: identified, Failed: failed}, nil
}

func hasProviderMatch(row db.IdentificationRow) bool {
	return row.TMDBID > 0 || strings.TrimSpace(row.TVDBID) != ""
}

func rowNeedsTrackedIdentify(row db.IdentificationRow) bool {
	return row.MatchStatus != db.MatchStatusIdentified || !hasProviderMatch(row)
}

func splitIdentifyRows(rows []db.IdentificationRow) (tracked []db.IdentificationRow, refreshOnly []db.IdentificationRow) {
	tracked = make([]db.IdentificationRow, 0, len(rows))
	refreshOnly = make([]db.IdentificationRow, 0, len(rows))
	for _, row := range rows {
		if rowNeedsTrackedIdentify(row) {
			tracked = append(tracked, row)
			continue
		}
		refreshOnly = append(refreshOnly, row)
	}
	return tracked, refreshOnly
}

func splitEpisodeIdentifyRows(rows []db.EpisodeIdentifyRow) (tracked []db.EpisodeIdentifyRow, refreshOnly []db.EpisodeIdentifyRow) {
	tracked = make([]db.EpisodeIdentifyRow, 0, len(rows))
	refreshOnly = make([]db.EpisodeIdentifyRow, 0, len(rows))
	for _, row := range rows {
		if rowNeedsTrackedIdentify(row.IdentificationRow) {
			tracked = append(tracked, row)
			continue
		}
		refreshOnly = append(refreshOnly, row)
	}
	return tracked, refreshOnly
}

func (h *LibraryHandler) refreshMatchedRows(
	ctx context.Context,
	libraryID int,
	libraryPath string,
	rows []db.IdentificationRow,
	movieCache *movieIdentifyCache,
) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	jobs := make([]identifyJob, 0, len(rows))
	for _, row := range rows {
		jobs = append(jobs, identifyJob{row: row})
	}
	sortIdentifyJobs(jobs, libraryPath)
	identified, retryJobs, initialFailed := h.runIdentifyJobs(ctx, libraryID, libraryPath, jobs, movieCache)
	retryIdentified, _, retryFailed := h.runIdentifyJobs(ctx, libraryID, libraryPath, retryJobs, movieCache)
	identified += retryIdentified
	refreshIdentified, _ := h.identifyShowFallbacks(ctx, libraryID, libraryPath, append(initialFailed, retryFailed...), nil, false)
	return identified + refreshIdentified, nil
}

func (h *LibraryHandler) identifyEpisodeRowsWithRefresh(
	ctx context.Context,
	libraryID int,
	libraryPath string,
	libraryType string,
	trackedRows []db.EpisodeIdentifyRow,
	refreshOnlyRows []db.EpisodeIdentifyRow,
) (identified int, failed int, err error) {
	if len(trackedRows) > 0 {
		trackedResult, trackErr := h.identifyEpisodesByGroup(ctx, libraryID, libraryPath, trackedRows)
		if trackErr != nil {
			return 0, 0, trackErr
		}
		identified += trackedResult.Identified
		failed += trackedResult.Failed
	}
	if len(refreshOnlyRows) > 0 {
		refreshResult, refreshErr := h.identifyEpisodesByGroup(ctx, libraryID, libraryPath, refreshOnlyRows)
		if refreshErr != nil {
			slog.Warn("identify refresh-only episode rows failed", "library_id", libraryID, "type", libraryType, "error", refreshErr)
		} else {
			identified += refreshResult.Identified
		}
	}
	return identified, failed, nil
}

func (h *LibraryHandler) runIdentifyJobs(
	ctx context.Context,
	libraryID int,
	libraryPath string,
	jobsToRun []identifyJob,
	movieCache *movieIdentifyCache,
) (identified int, retryJobs []identifyJob, failed []identifyJobResult) {
	if len(jobsToRun) == 0 {
		return 0, nil, nil
	}

	results := make(chan identifyJobResult, len(jobsToRun))
	jobs := make(chan identifyJob)
	config := identifyConfigForKind(jobsToRun[0].row.Kind)
	workerCount := config.workers
	if workerCount > len(jobsToRun) {
		workerCount = len(jobsToRun)
	}
	if workerCount < 1 {
		workerCount = 1
	}
	rateLimiter := newIdentifyRateLimiter(ctx, config.rateInterval, config.rateBurst)

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case job, ok := <-jobs:
					if !ok {
						return
					}
					results <- h.identifyLibraryJob(ctx, libraryID, job, libraryPath, rateLimiter, movieCache)
				}
			}
		}()
	}

	go func() {
		defer close(results)
		wg.Wait()
	}()

enqueueLoop:
	for _, job := range jobsToRun {
		select {
		case <-ctx.Done():
			break enqueueLoop
		case jobs <- job:
		}
	}
	close(jobs)

	for result := range results {
		switch result.status {
		case identifyJobSucceeded:
			identified++
		case identifyJobRetry:
			retryJobs = append(retryJobs, identifyJob{
				row:     result.job.row,
				attempt: result.job.attempt + 1,
			})
		case identifyJobFailed:
			failed = append(failed, result)
		}
	}

	return identified, retryJobs, failed
}

type showFallbackGroup struct {
	queries []string
	rows    []db.IdentificationRow
}

func (h *LibraryHandler) identifyShowFallbacks(
	ctx context.Context,
	libraryID int,
	libraryPath string,
	failedResults []identifyJobResult,
	cache *episodicIdentifyCache,
	queueSearch bool,
) (identified int, failed int) {
	if len(failedResults) == 0 {
		return 0, 0
	}

	groups := make(map[string]*showFallbackGroup)
	for _, result := range failedResults {
		if !result.fallbackEligible {
			h.ensureIdentifyRun().setState(libraryID, result.job.row.Kind, result.job.row.Path, "failed")
			failed++
			continue
		}
		queries := showFallbackQueries(result.job.row, libraryPath)
		if len(queries) == 0 {
			h.ensureIdentifyRun().setState(libraryID, result.job.row.Kind, result.job.row.Path, "failed")
			failed++
			continue
		}
		groupKey := strings.ToLower(queries[0])
		group, ok := groups[groupKey]
		if !ok {
			group = &showFallbackGroup{queries: queries}
			groups[groupKey] = group
		}
		group.rows = append(group.rows, result.job.row)
	}

	for _, group := range groups {
		updated, err := h.identifyShowFallbackGroup(ctx, libraryPath, group.queries, group.rows, cache, queueSearch)
		if err != nil || updated != len(group.rows) {
			h.ensureIdentifyRun().failRows(libraryID, group.rows[updated:])
			identified += updated
			failed += len(group.rows) - updated
			continue
		}
		for _, row := range group.rows {
			h.ensureIdentifyRun().setState(libraryID, row.Kind, row.Path, "")
		}
		identified += updated
	}

	return identified, failed
}

func episodeGroupKey(row db.EpisodeIdentifyRow, libraryPath string) (string, string, []string) {
	if row.Season <= 0 || row.Episode <= 0 {
		return "", "", nil
	}
	info := identifyMediaInfo(row.IdentificationRow, libraryPath)
	title := strings.TrimSpace(info.Title)
	queries := showFallbackQueries(row.IdentificationRow, libraryPath)
	if row.TMDBID > 0 {
		return "tmdb:" + strconv.Itoa(row.TMDBID), "", nil
	}
	if row.TVDBID != "" {
		return "tvdb:" + row.TVDBID, title, queries
	}
	if title != "" {
		return "title:" + metadata.NormalizeSeriesTitle(title), title, queries
	}
	if len(queries) > 0 {
		return "fallback:" + strings.ToLower(queries[0]), queries[0], queries
	}
	return "", "", nil
}

func buildEpisodeIdentifyGroups(rows []db.EpisodeIdentifyRow, libraryPath string) ([]episodeIdentifyGroup, []identifyJob) {
	groupsByKey := make(map[string]*episodeIdentifyGroup)
	order := make([]string, 0, len(rows))
	residual := make([]identifyJob, 0)
	for _, row := range rows {
		key, query, fallbackQueries := episodeGroupKey(row, libraryPath)
		if key == "" {
			residual = append(residual, identifyJob{row: row.IdentificationRow})
			continue
		}
		group, ok := groupsByKey[key]
		if !ok {
			group = &episodeIdentifyGroup{
				key:             key,
				kind:            row.Kind,
				groupQuery:      strings.TrimSpace(query),
				fallbackQueries: append([]string(nil), fallbackQueries...),
				explicitTMDBID:  row.TMDBID,
				explicitTVDBID:  row.TVDBID,
				representative:  row,
			}
			groupsByKey[key] = group
			order = append(order, key)
		}
		group.rows = append(group.rows, row)
		if row.TMDBID > 0 && group.explicitTMDBID == 0 {
			group.explicitTMDBID = row.TMDBID
		}
		if row.TVDBID != "" && group.explicitTVDBID == "" {
			group.explicitTVDBID = row.TVDBID
		}
		if group.groupQuery == "" && query != "" {
			group.groupQuery = query
		}
		if len(group.fallbackQueries) == 0 && len(fallbackQueries) > 0 {
			group.fallbackQueries = append([]string(nil), fallbackQueries...)
		}
	}

	groups := make([]episodeIdentifyGroup, 0, len(order))
	for _, key := range order {
		group := groupsByKey[key]
		if len(group.rows) < 2 && group.explicitTMDBID == 0 && group.explicitTVDBID == "" {
			residual = append(residual, identifyJob{row: group.rows[0].IdentificationRow})
			continue
		}
		sort.SliceStable(group.rows, func(i, j int) bool {
			if group.rows[i].Season != group.rows[j].Season {
				return group.rows[i].Season < group.rows[j].Season
			}
			if group.rows[i].Episode != group.rows[j].Episode {
				return group.rows[i].Episode < group.rows[j].Episode
			}
			return group.rows[i].Path < group.rows[j].Path
		})
		group.representative = group.rows[0]
		groups = append(groups, *group)
	}
	sortIdentifyJobs(residual, libraryPath)
	return groups, residual
}

func identifyGroupRowsAsQueued(tracker *identifyRunTracker, libraryID int, rows []db.EpisodeIdentifyRow) {
	if tracker == nil {
		return
	}
	for _, row := range rows {
		tracker.setState(libraryID, row.Kind, row.Path, "queued")
	}
}

func identifyGroupRowsAsIdentifying(tracker *identifyRunTracker, libraryID int, rows []db.EpisodeIdentifyRow) {
	if tracker == nil {
		return
	}
	for _, row := range rows {
		tracker.setState(libraryID, row.Kind, row.Path, "identifying")
	}
}

func identifyGroupRowsClear(tracker *identifyRunTracker, libraryID int, rows []db.EpisodeIdentifyRow) {
	if tracker == nil {
		return
	}
	for _, row := range rows {
		tracker.setState(libraryID, row.Kind, row.Path, "")
	}
}

func identifyGroupRowsFail(tracker *identifyRunTracker, libraryID int, rows []db.EpisodeIdentifyRow) {
	if tracker == nil {
		return
	}
	for _, row := range rows {
		tracker.setState(libraryID, row.Kind, row.Path, "failed")
	}
}

func episodeIdentifyFailedResults(group episodeIdentifyGroup) []identifyJobResult {
	out := make([]identifyJobResult, 0, len(group.rows))
	for _, row := range group.rows {
		out = append(out, identifyJobResult{
			status:           identifyJobFailed,
			job:              identifyJob{row: row.IdentificationRow},
			fallbackEligible: true,
		})
	}
	return out
}

func episodeIdentifyFallbackResultsFromJobs(results []identifyJobResult) []identifyJobResult {
	out := make([]identifyJobResult, 0, len(results))
	for _, result := range results {
		if result.status != identifyJobFailed {
			continue
		}
		out = append(out, result)
	}
	return out
}

func (h *LibraryHandler) identifyEpisodesByGroup(
	ctx context.Context,
	libraryID int,
	libraryPath string,
	rows []db.EpisodeIdentifyRow,
) (identifyResult, error) {
	groups, residualJobs := buildEpisodeIdentifyGroups(rows, libraryPath)
	cache := newEpisodicIdentifyCache(h)

	identified := 0
	failedResults := make([]identifyJobResult, 0)

	groupIdentified, retryGroups, groupFailed := h.runEpisodeIdentifyGroups(ctx, libraryID, libraryPath, groups, cache)
	identified += groupIdentified
	failedResults = append(failedResults, groupFailed...)

	retryIdentified, unresolvedGroups, retryFailed := h.runEpisodeIdentifyGroups(ctx, libraryID, libraryPath, retryGroups, cache)
	identified += retryIdentified
	failedResults = append(failedResults, retryFailed...)
	for _, group := range unresolvedGroups {
		failedResults = append(failedResults, episodeIdentifyFailedResults(group)...)
	}

	residualIdentified, residualRetryJobs, residualInitialFailed := h.runIdentifyJobs(ctx, libraryID, libraryPath, residualJobs, nil)
	identified += residualIdentified
	residualRetryIdentified, _, residualRetryFailed := h.runIdentifyJobs(ctx, libraryID, libraryPath, residualRetryJobs, nil)
	identified += residualRetryIdentified
	failedResults = append(failedResults, residualInitialFailed...)
	failedResults = append(failedResults, residualRetryFailed...)

	fallbackIdentified, fallbackFailed := h.identifyShowFallbacks(ctx, libraryID, libraryPath, failedResults, cache, false)
	identified += fallbackIdentified
	return identifyResult{Identified: identified, Failed: fallbackFailed}, nil
}

func (h *LibraryHandler) runEpisodeIdentifyGroups(
	ctx context.Context,
	libraryID int,
	libraryPath string,
	groups []episodeIdentifyGroup,
	cache *episodicIdentifyCache,
) (identified int, retryGroups []episodeIdentifyGroup, failed []identifyJobResult) {
	if len(groups) == 0 {
		return 0, nil, nil
	}

	results := make(chan episodeGroupResult, len(groups))
	groupJobs := make(chan episodeGroupJob)
	config := identifyConfigForKind(groups[0].kind)
	workerCount := config.workers
	if workerCount > len(groups) {
		workerCount = len(groups)
	}
	if workerCount < 1 {
		workerCount = 1
	}
	rateLimiter := newIdentifyRateLimiter(ctx, config.rateInterval, config.rateBurst)

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case job, ok := <-groupJobs:
					if !ok {
						return
					}
					groupIdentified, retry, groupFailed := h.identifyEpisodeGroup(ctx, libraryID, libraryPath, job, cache, rateLimiter)
					results <- episodeGroupResult{
						group:      job.group,
						identified: groupIdentified,
						retry:      retry,
						failed:     groupFailed,
					}
				}
			}
		}()
	}

	go func() {
		defer close(results)
		wg.Wait()
	}()

enqueueLoop:
	for _, group := range groups {
		select {
		case <-ctx.Done():
			break enqueueLoop
		case groupJobs <- episodeGroupJob{group: group, attempt: group.attempt}:
		}
	}
	close(groupJobs)

	for result := range results {
		identified += result.identified
		if result.retry {
			next := result.group
			next.attempt++
			retryGroups = append(retryGroups, next)
		}
		failed = append(failed, result.failed...)
	}

	return identified, retryGroups, failed
}

type tmdbSeriesSelection struct {
	tmdbID               int
	metadataReviewNeeded bool
}

func fallbackIdentifyInfo(row db.IdentificationRow, libraryPath string) metadata.MediaInfo {
	info := identifyMediaInfo(row, libraryPath)
	if info.Season == 0 {
		info.Season = row.Season
	}
	if info.Episode == 0 {
		info.Episode = row.Episode
	}
	if info.Title == "" {
		info.Title = row.Title
	}
	return info
}

func scoredTMDBSeriesMatch(
	candidates []metadata.MatchResult,
	info metadata.MediaInfo,
) (best *metadata.MatchResult, topScore int, secondScore int, hasSecond bool) {
	type scored struct {
		match *metadata.MatchResult
		score int
	}
	scores := make([]scored, 0, len(candidates))
	for i := range candidates {
		candidate := &candidates[i]
		if candidate.Provider != "tmdb" {
			continue
		}
		if tmdbID, err := strconv.Atoi(candidate.ExternalID); err != nil || tmdbID <= 0 {
			continue
		}
		scores = append(scores, scored{
			match: candidate,
			score: metadata.ScoreTV(candidate, info),
		})
	}
	if len(scores) == 0 {
		return nil, 0, 0, false
	}
	sort.SliceStable(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})
	best = scores[0].match
	topScore = scores[0].score
	if len(scores) > 1 {
		secondScore = scores[1].score
		hasSecond = true
	}
	return best, topScore, secondScore, hasSecond
}

func (h *LibraryHandler) selectTMDBSeriesFallback(
	ctx context.Context,
	libraryPath string,
	representative db.IdentificationRow,
	queries []string,
	cache *episodicIdentifyCache,
) (tmdbSeriesSelection, error) {
	info := fallbackIdentifyInfo(representative, libraryPath)
	seenQueries := make(map[string]struct{}, len(queries))
	bestTentative := tmdbSeriesSelection{}
	bestTentativeScore := 0
	hasTentative := false

	for _, query := range queries {
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}
		key := strings.ToLower(query)
		if _, ok := seenQueries[key]; ok {
			continue
		}
		seenQueries[key] = struct{}{}

		var (
			results []metadata.MatchResult
			err     error
		)
		if cache != nil {
			results, err = cache.SearchTV(ctx, query)
		} else {
			results, err = h.SeriesQuery.SearchTV(ctx, query)
		}
		if err != nil {
			return tmdbSeriesSelection{}, err
		}

		best, topScore, secondScore, hasSecond := scoredTMDBSeriesMatch(results, info)
		if best == nil {
			continue
		}
		tmdbID, err := strconv.Atoi(best.ExternalID)
		if err != nil || tmdbID <= 0 {
			continue
		}
		if topScore >= metadata.ScoreAutoMatch &&
			(!hasSecond || (topScore-secondScore) >= metadata.ScoreMargin) {
			return tmdbSeriesSelection{tmdbID: tmdbID, metadataReviewNeeded: false}, nil
		}
		if !hasTentative || topScore > bestTentativeScore {
			bestTentative = tmdbSeriesSelection{
				tmdbID:               tmdbID,
				metadataReviewNeeded: true,
			}
			bestTentativeScore = topScore
			hasTentative = true
		}
	}

	if hasTentative {
		return bestTentative, nil
	}
	return tmdbSeriesSelection{}, nil
}

func (h *LibraryHandler) resolveTMDBSeriesSelectionForGroup(
	ctx context.Context,
	libraryPath string,
	group episodeIdentifyGroup,
	cache *episodicIdentifyCache,
) (tmdbSeriesSelection, error) {
	if group.explicitTMDBID > 0 {
		return tmdbSeriesSelection{tmdbID: group.explicitTMDBID}, nil
	}
	queries := make([]string, 0, len(group.fallbackQueries)+1)
	if group.groupQuery != "" {
		queries = append(queries, group.groupQuery)
	}
	for _, query := range group.fallbackQueries {
		if query == "" {
			continue
		}
		seen := false
		for _, existing := range queries {
			if strings.EqualFold(existing, query) {
				seen = true
				break
			}
		}
		if !seen {
			queries = append(queries, query)
		}
	}
	return h.selectTMDBSeriesFallback(ctx, libraryPath, group.representative.IdentificationRow, queries, cache)
}

func (h *LibraryHandler) identifyEpisodeGroup(
	ctx context.Context,
	libraryID int,
	libraryPath string,
	job episodeGroupJob,
	cache *episodicIdentifyCache,
	rateLimiter <-chan struct{},
) (identified int, retry bool, failed []identifyJobResult) {
	identifyGroupRowsAsIdentifying(h.ensureIdentifyRun(), libraryID, job.group.rows)
	if h.ScanJobs != nil {
		h.ScanJobs.RecordIdentifyActivity(libraryID, job.group.representative.Path)
	}
	select {
	case <-ctx.Done():
		return 0, false, episodeIdentifyFailedResults(job.group)
	case <-rateLimiter:
	}

	itemCtx, cancel := context.WithTimeout(ctx, identifyTimeoutForAttempt(job.attempt))
	defer cancel()

	selection, err := h.resolveTMDBSeriesSelectionForGroup(itemCtx, libraryPath, job.group, cache)
	if err != nil || selection.tmdbID <= 0 {
		if err == nil && job.attempt == 0 {
			identifyGroupRowsAsQueued(h.ensureIdentifyRun(), libraryID, job.group.rows)
			return 0, true, nil
		}
		identifyGroupRowsFail(h.ensureIdentifyRun(), libraryID, job.group.rows)
		return 0, false, episodeIdentifyFailedResults(job.group)
	}

	refs := make([]db.ShowEpisodeRef, 0, len(job.group.rows))
	for _, row := range job.group.rows {
		refs = append(refs, db.ShowEpisodeRef{
			RefID:   row.RefID,
			Kind:    row.Kind,
			Season:  row.Season,
			Episode: row.Episode,
		})
	}
	updatedRefIDs, err := h.applySeriesToRefs(
		itemCtx,
		selection.tmdbID,
		refs,
		selection.metadataReviewNeeded,
		false,
		cache,
		false,
	)
	if err != nil || len(updatedRefIDs) == 0 {
		if err == nil && job.attempt == 0 {
			identifyGroupRowsAsQueued(h.ensureIdentifyRun(), libraryID, job.group.rows)
			return 0, true, nil
		}
		identifyGroupRowsFail(h.ensureIdentifyRun(), libraryID, job.group.rows)
		return 0, false, episodeIdentifyFailedResults(job.group)
	}

	updatedSet := make(map[int]struct{}, len(updatedRefIDs))
	for _, refID := range updatedRefIDs {
		updatedSet[refID] = struct{}{}
	}
	updatedRows := make([]db.EpisodeIdentifyRow, 0, len(updatedSet))
	unresolved := make([]db.EpisodeIdentifyRow, 0)
	for _, row := range job.group.rows {
		if _, ok := updatedSet[row.RefID]; ok {
			updatedRows = append(updatedRows, row)
			continue
		}
		unresolved = append(unresolved, row)
	}
	identifyGroupRowsClear(h.ensureIdentifyRun(), libraryID, updatedRows)
	if len(unresolved) > 0 {
		identifyGroupRowsFail(h.ensureIdentifyRun(), libraryID, unresolved)
		failed = append(failed, episodeIdentifyFailedResults(episodeIdentifyGroup{
			key:             job.group.key,
			kind:            job.group.kind,
			groupQuery:      job.group.groupQuery,
			fallbackQueries: job.group.fallbackQueries,
			rows:            unresolved,
		})...)
	}
	return len(updatedRows), false, failed
}

func newIdentifyRateLimiter(ctx context.Context, interval time.Duration, burst int) <-chan struct{} {
	if burst < 1 {
		burst = 1
	}
	ch := make(chan struct{}, burst)
	for i := 0; i < burst; i++ {
		ch <- struct{}{}
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				select {
				case ch <- struct{}{}:
				default:
				}
			}
		}
	}()
	return ch
}

func identifyTimeoutForAttempt(attempt int) time.Duration {
	if attempt <= 0 {
		return identifyInitialTimeout
	}
	return identifyRetryTimeout
}

func movieIdentifyKey(info metadata.MediaInfo) string {
	title := metadata.NormalizeTitle(info.Title)
	if title == "" {
		return ""
	}
	if info.Year > 0 {
		return title + ":" + strconv.Itoa(info.Year)
	}
	return title
}

func (h *LibraryHandler) identifyMovieResult(
	ctx context.Context,
	info metadata.MediaInfo,
) (*metadata.MatchResult, error) {
	if withError, ok := h.Meta.(metadata.MovieIdentifierWithError); ok {
		return withError.IdentifyMovieResult(ctx, info)
	}
	return h.Meta.IdentifyMovie(ctx, info), nil
}

func logRetryableMovieIdentifyFailure(libraryID int, title string, err error) {
	var providerErr *metadata.ProviderError
	if errors.As(err, &providerErr) {
		slog.Warn("identify movie retryable failure",
			"library_id", libraryID,
			"provider", providerErr.Provider,
			"status", providerErr.StatusCode,
			"title", title,
			"error", err,
		)
		return
	}
	slog.Warn("identify movie retryable failure", "library_id", libraryID, "title", title, "error", err)
}

func updateMetadataWithRetry(
	ctx context.Context,
	dbConn *sql.DB,
	table string,
	refID int,
	title string,
	overview string,
	posterPath string,
	backdropPath string,
	releaseDate string,
	voteAvg float64,
	imdbID string,
	imdbRating float64,
	tmdbID int,
	tvdbID string,
	season int,
	episode int,
	canonical db.CanonicalMetadata,
	metadataReviewNeeded bool,
	metadataConfirmed bool,
	updateShowVoteAverage bool,
) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		lastErr = db.UpdateMediaMetadataWithCanonicalState(
			ctx,
			dbConn,
			table,
			refID,
			title,
			overview,
			posterPath,
			backdropPath,
			releaseDate,
			voteAvg,
			imdbID,
			imdbRating,
			tmdbID,
			tvdbID,
			season,
			episode,
			canonical,
			metadataReviewNeeded,
			metadataConfirmed,
			updateShowVoteAverage,
		)
		if lastErr == nil || !isSQLiteBusyError(lastErr) {
			return lastErr
		}
		time.Sleep(time.Duration(attempt+1) * 25 * time.Millisecond)
	}
	return lastErr
}

type existingEpisodeMetadata struct {
	Title        string
	Overview     string
	PosterPath   string
	BackdropPath string
	ReleaseDate  string
	VoteAverage  float64
	IMDbID       string
	IMDbRating   float64
}

func loadExistingEpisodeMetadata(dbConn *sql.DB, table string, refID int) (*existingEpisodeMetadata, error) {
	if table != "tv_episodes" && table != "anime_episodes" {
		return nil, nil
	}
	row := &existingEpisodeMetadata{}
	if err := dbConn.QueryRow(
		`SELECT title, COALESCE(overview, ''), COALESCE(poster_path, ''), COALESCE(backdrop_path, ''), COALESCE(release_date, ''), COALESCE(vote_average, 0), COALESCE(imdb_id, ''), COALESCE(imdb_rating, 0) FROM `+table+` WHERE id = ?`,
		refID,
	).Scan(
		&row.Title,
		&row.Overview,
		&row.PosterPath,
		&row.BackdropPath,
		&row.ReleaseDate,
		&row.VoteAverage,
		&row.IMDbID,
		&row.IMDbRating,
	); err != nil {
		return nil, err
	}
	return row, nil
}

func (h *LibraryHandler) identifyLibraryJob(
	ctx context.Context,
	libraryID int,
	job identifyJob,
	libraryPath string,
	rateLimiter <-chan struct{},
	movieCache *movieIdentifyCache,
) identifyJobResult {
	h.ensureIdentifyRun().setState(libraryID, job.row.Kind, job.row.Path, "identifying")
	if h.ScanJobs != nil {
		h.ScanJobs.RecordIdentifyActivity(libraryID, job.row.Path)
	}
	select {
	case <-ctx.Done():
		return identifyJobResult{job: job}
	case <-rateLimiter:
	}

	row := job.row
	info := identifyMediaInfo(row, libraryPath)
	if info.Season == 0 {
		info.Season = row.Season
	}
	if info.Episode == 0 {
		info.Episode = row.Episode
	}
	if info.Title == "" {
		info.Title = row.Title
	}

	itemCtx, cancel := context.WithTimeout(ctx, identifyTimeoutForAttempt(job.attempt))
	defer cancel()

	var (
		res      *metadata.MatchResult
		movieErr error
	)
	switch row.Kind {
	case db.LibraryTypeTV:
		res = h.Meta.IdentifyTV(itemCtx, info)
	case db.LibraryTypeAnime:
		res = h.Meta.IdentifyAnime(itemCtx, info)
	case db.LibraryTypeMovie:
		if movieCache != nil {
			res, movieErr = movieCache.lookupOrRun(movieIdentifyKey(info), func() (*metadata.MatchResult, error) {
				return h.identifyMovieResult(itemCtx, info)
			})
		} else {
			res, movieErr = h.identifyMovieResult(itemCtx, info)
		}
	default:
		return identifyJobResult{status: identifyJobFailed, job: job}
	}
	if row.Kind == db.LibraryTypeMovie && movieErr != nil {
		if metadata.IsRetryableProviderError(movieErr) && job.attempt == 0 {
			logRetryableMovieIdentifyFailure(libraryID, row.Title, movieErr)
			h.ensureIdentifyRun().setState(libraryID, row.Kind, row.Path, "queued")
			return identifyJobResult{status: identifyJobRetry, job: job}
		}
		if metadata.IsRetryableProviderError(movieErr) {
			logRetryableMovieIdentifyFailure(libraryID, row.Title, movieErr)
		}
		return identifyJobResult{status: identifyJobFailed, job: job}
	}
	if res == nil {
		if row.Kind == db.LibraryTypeMovie {
			return identifyJobResult{status: identifyJobFailed, job: job}
		}
		if job.attempt == 0 {
			h.ensureIdentifyRun().setState(libraryID, row.Kind, row.Path, "queued")
			return identifyJobResult{status: identifyJobRetry, job: job}
		}
		return identifyJobResult{
			status:           identifyJobFailed,
			job:              job,
			fallbackEligible: (row.Kind == db.LibraryTypeAnime || row.Kind == db.LibraryTypeTV) && itemCtx.Err() == nil,
		}
	}

	tmdbID, tvdbID := 0, ""
	switch res.Provider {
	case "tmdb":
		if id, err := strconv.Atoi(res.ExternalID); err == nil {
			tmdbID = id
		}
	case "tvdb":
		tvdbID = res.ExternalID
	}
	tbl := db.MediaTableForKind(row.Kind)
	cast := make([]db.CastCredit, 0, len(res.Cast))
	for _, member := range res.Cast {
		cast = append(cast, db.CastCredit{
			Name:        member.Name,
			Character:   member.Character,
			Order:       member.Order,
			ProfilePath: member.ProfilePath,
			Provider:    member.Provider,
			ProviderID:  member.ProviderID,
		})
	}
	seasonNumber := row.Season
	if seasonNumber == 0 {
		seasonNumber = info.Season
	}
	episodeNumber := row.Episode
	if episodeNumber == 0 {
		episodeNumber = info.Episode
	}
	posterPath := res.PosterURL
	settings := loadMetadataArtworkSettings(h.DB)
	canonical := db.CanonicalMetadata{
		Title:        res.Title,
		Overview:     res.Overview,
		BackdropPath: res.BackdropURL,
		ReleaseDate:  res.ReleaseDate,
		VoteAverage:  res.VoteAverage,
		IMDbID:       res.IMDbID,
		IMDbRating:   res.IMDbRating,
		Genres:       res.Genres,
		Cast:         cast,
		Runtime:      res.Runtime,
	}
	switch row.Kind {
	case db.LibraryTypeMovie:
		posterPath = automaticMoviePosterSource(
			ctx,
			h.Artwork,
			settings,
			tmdbID,
			res.IMDbID,
			res.PosterURL,
			res.Provider,
		)
	case db.LibraryTypeTV, db.LibraryTypeAnime:
		showTitle := showTitleFromEpisodeTitle(res.Title)
		canonical.PosterPath = automaticShowPosterSource(
			ctx,
			h.Artwork,
			settings,
			showTitle,
			tmdbID,
			tvdbID,
			res.PosterURL,
			res.Provider,
		)
		canonical.SeasonPosterPath = automaticSeasonPosterSource(
			ctx,
			h.Artwork,
			settings,
			showTitle,
			tmdbID,
			tvdbID,
			seasonNumber,
			canonical.PosterPath,
			"",
		)
		if canonical.SeasonPosterPath == "" {
			canonical.SeasonPosterPath = canonical.PosterPath
		}
		posterPath = automaticEpisodePosterSource(
			ctx,
			h.Artwork,
			settings,
			showTitle,
			tmdbID,
			tvdbID,
			res.IMDbID,
			seasonNumber,
			episodeNumber,
			res.PosterURL,
			res.Provider,
		)
	}
	if err := updateMetadataWithRetry(ctx, h.DB, tbl, row.RefID, res.Title, res.Overview, posterPath, res.BackdropURL, res.ReleaseDate, res.VoteAverage, res.IMDbID, res.IMDbRating, tmdbID, tvdbID, seasonNumber, episodeNumber, canonical, false, false, true); err != nil {
		return identifyJobResult{status: identifyJobFailed, job: job}
	}
	h.ensureIdentifyRun().setState(libraryID, row.Kind, row.Path, "")
	return identifyJobResult{status: identifyJobSucceeded, job: job}
}

func (h *LibraryHandler) identifyShowFallbackGroup(
	ctx context.Context,
	libraryPath string,
	queries []string,
	rows []db.IdentificationRow,
	cache *episodicIdentifyCache,
	queueSearch bool,
) (int, error) {
	if h.SeriesQuery == nil || len(rows) == 0 {
		return 0, nil
	}
	selection, err := h.selectTMDBSeriesFallback(ctx, libraryPath, rows[0], queries, cache)
	if err != nil {
		return 0, err
	}
	if selection.tmdbID <= 0 {
		return 0, nil
	}
	refs := make([]db.ShowEpisodeRef, 0, len(rows))
	for _, row := range rows {
		refs = append(refs, db.ShowEpisodeRef{
			RefID:   row.RefID,
			Kind:    row.Kind,
			Season:  row.Season,
			Episode: row.Episode,
		})
	}
	updatedRefIDs, err := h.applySeriesToRefs(
		ctx,
		selection.tmdbID,
		refs,
		selection.metadataReviewNeeded,
		false,
		cache,
		queueSearch,
	)
	return len(updatedRefIDs), err
}

func showFallbackQueries(row db.IdentificationRow, libraryPath string) []string {
	info := identifyMediaInfo(row, libraryPath)
	candidates := []string{
		showTitleFromEpisodeTitle(row.Title),
		strings.TrimSpace(info.Title),
	}
	seen := make(map[string]struct{}, len(candidates))
	queries := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		key := strings.ToLower(candidate)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		queries = append(queries, candidate)
	}
	return queries
}

func showTitleFromEpisodeTitle(title string) string {
	title = strings.TrimSpace(title)
	if i := strings.Index(strings.ToLower(title), " - s"); i > 0 {
		return strings.TrimSpace(title[:i])
	}
	if i := strings.Index(title, " - "); i > 0 {
		return strings.TrimSpace(title[:i])
	}
	return title
}

func (h *LibraryHandler) applySeriesToRefs(
	ctx context.Context,
	seriesTMDBID int,
	refs []db.ShowEpisodeRef,
	metadataReviewNeeded bool,
	metadataConfirmed bool,
	cache *episodicIdentifyCache,
	queueSearch bool,
) ([]int, error) {
	if h.SeriesQuery == nil || seriesTMDBID <= 0 || len(refs) == 0 {
		return nil, nil
	}
	table := db.MediaTableForKind(refs[0].Kind)
	seriesID := strconv.Itoa(seriesTMDBID)
	var canonical db.CanonicalMetadata
	seriesTVDBID := ""
	if h.Series != nil {
		var details *metadata.SeriesDetails
		var err error
		if cache != nil {
			details, err = cache.GetSeriesDetails(ctx, seriesTMDBID)
		} else {
			details, err = h.Series.GetSeriesDetails(ctx, seriesTMDBID)
		}
		if err == nil && details != nil {
			cast := make([]db.CastCredit, 0, len(details.Cast))
			for _, member := range details.Cast {
				cast = append(cast, db.CastCredit{
					Name:        member.Name,
					Character:   member.Character,
					Order:       member.Order,
					ProfilePath: member.ProfilePath,
					Provider:    member.Provider,
					ProviderID:  member.ProviderID,
				})
			}
			canonical = db.CanonicalMetadata{
				Title:        details.Name,
				Overview:     details.Overview,
				PosterPath:   details.PosterPath,
				BackdropPath: details.BackdropPath,
				ReleaseDate:  details.FirstAirDate,
				VoteAverage:  details.VoteAverage,
				IMDbID:       details.IMDbID,
				IMDbRating:   details.IMDbRating,
				Genres:       details.Genres,
				Cast:         cast,
				Runtime:      details.Runtime,
			}
			seriesTVDBID = details.TVDBID
		}
	}
	settings := loadMetadataArtworkSettings(h.DB)
	showTitle := strings.TrimSpace(canonical.Title)
	canonical.PosterPath = automaticShowPosterSource(
		ctx,
		h.Artwork,
		settings,
		showTitle,
		seriesTMDBID,
		seriesTVDBID,
		canonical.PosterPath,
		"tmdb",
	)
	updatedRefIDs := make([]int, 0, len(refs))
	for _, ref := range refs {
		var (
			ep  *metadata.MatchResult
			err error
		)
		if cache != nil {
			ep, err = cache.GetEpisode(ctx, "tmdb", seriesID, ref.Season, ref.Episode)
		} else {
			ep, err = h.SeriesQuery.GetEpisode(ctx, "tmdb", seriesID, ref.Season, ref.Episode)
		}
		if err != nil || ep == nil {
			if !metadataConfirmed {
				continue
			}
			if len(strings.TrimSpace(canonical.Title)) == 0 {
				continue
			}
			existing, loadErr := loadExistingEpisodeMetadata(h.DB, table, ref.RefID)
			if loadErr != nil || existing == nil {
				continue
			}
			fallbackCanonical := canonical
			fallbackCanonical.SeasonPosterPath = automaticSeasonPosterSource(
				ctx,
				h.Artwork,
				settings,
				showTitle,
				seriesTMDBID,
				seriesTVDBID,
				ref.Season,
				fallbackCanonical.PosterPath,
				"tmdb",
			)
			if fallbackCanonical.SeasonPosterPath == "" {
				fallbackCanonical.SeasonPosterPath = fallbackCanonical.PosterPath
			}
			if err := updateMetadataWithRetry(
				ctx,
				h.DB,
				table,
				ref.RefID,
				existing.Title,
				existing.Overview,
				existing.PosterPath,
				existing.BackdropPath,
				existing.ReleaseDate,
				existing.VoteAverage,
				existing.IMDbID,
				existing.IMDbRating,
				seriesTMDBID,
				seriesTVDBID,
				ref.Season,
				ref.Episode,
				fallbackCanonical,
				metadataReviewNeeded,
				metadataConfirmed,
				true,
			); err != nil {
				continue
			}
			updatedRefIDs = append(updatedRefIDs, ref.RefID)
			if cache == nil {
				time.Sleep(identifyEpisodeRateLimit)
			}
			continue
		}
		if showTitle == "" {
			showTitle = showTitleFromEpisodeTitle(ep.Title)
		}
		tvdbID := ""
		if ep.Provider == "tvdb" {
			tvdbID = ep.ExternalID
		}
		if tvdbID == "" {
			tvdbID = seriesTVDBID
		}
		episodeCanonical := canonical
		episodeCanonical.SeasonPosterPath = automaticSeasonPosterSource(
			ctx,
			h.Artwork,
			settings,
			showTitle,
			seriesTMDBID,
			tvdbID,
			ref.Season,
			episodeCanonical.PosterPath,
			"tmdb",
		)
		if episodeCanonical.SeasonPosterPath == "" {
			episodeCanonical.SeasonPosterPath = episodeCanonical.PosterPath
		}
		episodePosterPath := automaticEpisodePosterSource(
			ctx,
			h.Artwork,
			settings,
			showTitle,
			seriesTMDBID,
			tvdbID,
			ep.IMDbID,
			ref.Season,
			ref.Episode,
			ep.PosterURL,
			ep.Provider,
		)
		if err := updateMetadataWithRetry(ctx, h.DB, table, ref.RefID, ep.Title, ep.Overview, episodePosterPath, ep.BackdropURL, ep.ReleaseDate, ep.VoteAverage, ep.IMDbID, ep.IMDbRating, seriesTMDBID, tvdbID, ref.Season, ref.Episode, episodeCanonical, metadataReviewNeeded, metadataConfirmed, true); err != nil {
			continue
		}
		updatedRefIDs = append(updatedRefIDs, ref.RefID)
		if cache == nil {
			time.Sleep(identifyEpisodeRateLimit)
		}
	}
	if len(updatedRefIDs) > 0 && queueSearch && len(refs) > 0 && h.SearchIndex != nil {
		var libraryID int
		if err := h.DB.QueryRow(`SELECT library_id FROM `+table+` WHERE id = ?`, refs[0].RefID).Scan(&libraryID); err == nil {
			h.SearchIndex.Queue(libraryID, false)
		}
	}
	return updatedRefIDs, nil
}

func (h *LibraryHandler) applyTMDBSeriesToRefs(
	ctx context.Context,
	seriesTMDBID int,
	refs []db.ShowEpisodeRef,
	metadataReviewNeeded bool,
	metadataConfirmed bool,
) (int, error) {
	updatedRefIDs, err := h.applySeriesToRefs(ctx, seriesTMDBID, refs, metadataReviewNeeded, metadataConfirmed, nil, true)
	return len(updatedRefIDs), err
}

func (h *LibraryHandler) applySeriesMatchToRefs(
	ctx context.Context,
	provider string,
	externalID string,
	refs []db.ShowEpisodeRef,
	metadataReviewNeeded bool,
	metadataConfirmed bool,
) (int, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	externalID = strings.TrimSpace(externalID)
	if provider == "" || externalID == "" || len(refs) == 0 || h.SeriesQuery == nil {
		return 0, nil
	}
	if provider == "tmdb" {
		seriesTMDBID, err := strconv.Atoi(externalID)
		if err != nil || seriesTMDBID <= 0 {
			return 0, nil
		}
		return h.applyTMDBSeriesToRefs(ctx, seriesTMDBID, refs, metadataReviewNeeded, metadataConfirmed)
	}

	table := db.MediaTableForKind(refs[0].Kind)
	updatedRefIDs := make([]int, 0, len(refs))
	for _, ref := range refs {
		ep, err := h.SeriesQuery.GetEpisode(ctx, provider, externalID, ref.Season, ref.Episode)
		if err != nil || ep == nil {
			continue
		}
		tmdbID := 0
		tvdbID := ""
		switch ep.Provider {
		case "tmdb":
			tmdbID, _ = strconv.Atoi(ep.ExternalID)
		case "tvdb":
			tvdbID = ep.ExternalID
		}
		if tvdbID == "" && provider == "tvdb" {
			tvdbID = externalID
		}
		showTitle := showTitleFromEpisodeTitle(ep.Title)
		settings := loadMetadataArtworkSettings(h.DB)
		posterPath := automaticEpisodePosterSource(
			ctx,
			h.Artwork,
			settings,
			showTitle,
			tmdbID,
			tvdbID,
			ep.IMDbID,
			ref.Season,
			ref.Episode,
			ep.PosterURL,
			ep.Provider,
		)
		canonical := db.CanonicalMetadata{
			Title:            showTitle,
			Overview:         ep.Overview,
			PosterPath:       posterPath,
			SeasonPosterPath: posterPath,
			BackdropPath:     ep.BackdropURL,
			ReleaseDate:      ep.ReleaseDate,
			// Show vote_average must come from provider series metadata (see migration 23), not per-episode scores.
			VoteAverage: 0,
			IMDbID:      ep.IMDbID,
			IMDbRating:  ep.IMDbRating,
			Genres:      ep.Genres,
			Runtime:     ep.Runtime,
		}
		if err := updateMetadataWithRetry(ctx, h.DB, table, ref.RefID, ep.Title, ep.Overview, posterPath, ep.BackdropURL, ep.ReleaseDate, ep.VoteAverage, ep.IMDbID, ep.IMDbRating, tmdbID, tvdbID, ref.Season, ref.Episode, canonical, metadataReviewNeeded, metadataConfirmed, false); err != nil {
			continue
		}
		updatedRefIDs = append(updatedRefIDs, ref.RefID)
	}
	if len(updatedRefIDs) > 0 && h.SearchIndex != nil {
		var libraryID int
		if err := h.DB.QueryRow(`SELECT library_id FROM `+table+` WHERE id = ?`, refs[0].RefID).Scan(&libraryID); err == nil {
			h.SearchIndex.Queue(libraryID, false)
		}
	}
	return len(updatedRefIDs), nil
}

func identifyMediaInfo(row db.IdentificationRow, libraryPath string) metadata.MediaInfo {
	base := filepath.Base(row.Path)
	relPath, _ := filepath.Rel(libraryPath, row.Path)
	applyProviderHints := func(info metadata.MediaInfo) metadata.MediaInfo {
		if info.TMDBID <= 0 && row.TMDBID > 0 {
			info.TMDBID = row.TMDBID
		}
		if info.TVDBID == "" && row.TVDBID != "" {
			info.TVDBID = row.TVDBID
		}
		return info
	}
	switch row.Kind {
	case db.LibraryTypeMovie:
		return applyProviderHints(metadata.MovieMediaInfo(metadata.ParseMovie(relPath, base)))
	case db.LibraryTypeTV, db.LibraryTypeAnime:
		info := metadata.ParseFilename(base)
		pathInfo := metadata.ParsePathForTV(relPath, base)
		info = metadata.MergePathInfo(pathInfo, info)
		showRoot := metadata.ShowRootPath(libraryPath, row.Path)
		metadata.ApplyShowNFO(&info, showRoot)
		if row.Kind == db.LibraryTypeAnime && info.IsSpecial && info.Episode > 0 {
			info.Season = 0
		}
		return applyProviderHints(info)
	default:
		return applyProviderHints(metadata.ParseFilename(base))
	}
}

func sortIdentifyJobs(jobs []identifyJob, libraryPath string) {
	sort.SliceStable(jobs, func(i, j int) bool {
		a := identifyJobPriority(jobs[i], libraryPath)
		b := identifyJobPriority(jobs[j], libraryPath)
		if a != b {
			return a < b
		}
		if jobs[i].row.Kind != jobs[j].row.Kind {
			return jobs[i].row.Kind < jobs[j].row.Kind
		}
		if jobs[i].row.Season != jobs[j].row.Season {
			return jobs[i].row.Season < jobs[j].row.Season
		}
		if jobs[i].row.Episode != jobs[j].row.Episode {
			return jobs[i].row.Episode < jobs[j].row.Episode
		}
		return jobs[i].row.Path < jobs[j].row.Path
	})
}

func identifyJobPriority(job identifyJob, libraryPath string) int {
	info := identifyMediaInfo(job.row, libraryPath)
	switch job.row.Kind {
	case db.LibraryTypeMovie:
		if info.TMDBID > 0 || info.TVDBID != "" {
			return 0
		}
		if info.Year > 0 {
			return 1
		}
		return 2
	case db.LibraryTypeTV, db.LibraryTypeAnime:
		season := info.Season
		if season == 0 {
			season = job.row.Season
		}
		episode := info.Episode
		if episode == 0 {
			episode = job.row.Episode
		}
		if (season == 1 || season == 0) && episode == 1 {
			return 0
		}
		if episode > 0 && episode <= 3 {
			return 1
		}
		return 2
	default:
		return 3
	}
}
