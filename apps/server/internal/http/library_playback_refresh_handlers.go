package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"

	"plum/internal/db"
)

type refreshShowRequest struct {
	ShowKey string `json:"showKey"`
}

type confirmShowRequest struct {
	ShowKey string `json:"showKey"`
}

type identifyShowRequest struct {
	ShowKey    string `json:"showKey"`
	Provider   string `json:"provider"`
	ExternalID string `json:"externalId"`
	TmdbID     int    `json:"tmdbId"`
}

type identifyMovieRequest struct {
	MediaID    int    `json:"mediaId"`
	Provider   string `json:"provider"`
	ExternalID string `json:"externalId"`
	TmdbID     int    `json:"tmdbId"`
}

type showActionResult struct {
	Updated int `json:"updated"`
}

type updateMediaProgressRequest struct {
	PositionSeconds float64 `json:"position_seconds"`
	DurationSeconds float64 `json:"duration_seconds"`
	Completed       bool    `json:"completed"`
}

type setContinueWatchingVisibilityRequest struct {
	Hidden bool `json:"hidden"`
}

type markShowWatchedRequest struct {
	Mode    string `json:"mode"`
	Season  *int   `json:"season,omitempty"`
	Episode *int   `json:"episode,omitempty"`
}

type playbackRefreshProgress struct {
	mu          sync.Mutex
	Total       int    `json:"total"`
	Processed   int    `json:"processed"`
	CurrentPath string `json:"current_path"`
}

func (p *playbackRefreshProgress) update(processed int, currentPath string) {
	p.mu.Lock()
	p.Processed = processed
	p.CurrentPath = currentPath
	p.mu.Unlock()
}

func (p *playbackRefreshProgress) itemFinished(currentPath string) {
	p.mu.Lock()
	p.Processed++
	if currentPath != "" {
		p.CurrentPath = currentPath
	}
	p.mu.Unlock()
}

func playbackRefreshWorkerCount(itemCount int) int {
	if itemCount <= 1 {
		return 1
	}
	n := runtime.NumCPU()
	if n < 2 {
		n = 2
	}
	if n > 8 {
		n = 8
	}
	if n > itemCount {
		n = itemCount
	}
	return n
}

func (p *playbackRefreshProgress) snapshot() (total, processed int, currentPath string) {
	p.mu.Lock()
	total, processed, currentPath = p.Total, p.Processed, p.CurrentPath
	p.mu.Unlock()
	return
}

// librarySideJobOccupied reports whether a playback, intro-only, or chromaprint job is in flight
// for libraryID. Caller must hold h.librarySideJobsMu.
func (h *LibraryHandler) librarySideJobOccupied(libraryID int) bool {
	if h.playbackRefreshStatus != nil {
		if _, ok := h.playbackRefreshStatus[libraryID]; ok {
			return true
		}
	}
	if h.introRefreshStatus != nil {
		if _, ok := h.introRefreshStatus[libraryID]; ok {
			return true
		}
	}
	if h.chromaprintRefreshStatus != nil {
		if _, ok := h.chromaprintRefreshStatus[libraryID]; ok {
			return true
		}
	}
	return false
}

func (h *LibraryHandler) tryStartLibraryPlaybackRefresh(libraryID int, total int) *playbackRefreshProgress {
	h.librarySideJobsMu.Lock()
	defer h.librarySideJobsMu.Unlock()
	if h.librarySideJobOccupied(libraryID) {
		return nil
	}
	if h.playbackRefreshStatus == nil {
		h.playbackRefreshStatus = make(map[int]*playbackRefreshProgress)
	}
	p := &playbackRefreshProgress{Total: total}
	h.playbackRefreshStatus[libraryID] = p
	return p
}

func (h *LibraryHandler) finishLibraryPlaybackRefresh(libraryID int) {
	h.librarySideJobsMu.Lock()
	defer h.librarySideJobsMu.Unlock()
	delete(h.playbackRefreshStatus, libraryID)
}

func (h *LibraryHandler) getPlaybackRefreshStatuses() map[int]*playbackRefreshProgress {
	h.librarySideJobsMu.Lock()
	defer h.librarySideJobsMu.Unlock()
	out := make(map[int]*playbackRefreshProgress, len(h.playbackRefreshStatus))
	for k, v := range h.playbackRefreshStatus {
		out[k] = v
	}
	return out
}

func (h *LibraryHandler) tryStartLibraryIntroRefresh(libraryID int, total int) *playbackRefreshProgress {
	h.librarySideJobsMu.Lock()
	defer h.librarySideJobsMu.Unlock()
	if h.librarySideJobOccupied(libraryID) {
		return nil
	}
	if h.introRefreshStatus == nil {
		h.introRefreshStatus = make(map[int]*playbackRefreshProgress)
	}
	p := &playbackRefreshProgress{Total: total}
	h.introRefreshStatus[libraryID] = p
	return p
}

func (h *LibraryHandler) finishLibraryIntroRefresh(libraryID int) {
	h.librarySideJobsMu.Lock()
	defer h.librarySideJobsMu.Unlock()
	delete(h.introRefreshStatus, libraryID)
}

func (h *LibraryHandler) getIntroRefreshStatuses() map[int]*playbackRefreshProgress {
	h.librarySideJobsMu.Lock()
	defer h.librarySideJobsMu.Unlock()
	out := make(map[int]*playbackRefreshProgress, len(h.introRefreshStatus))
	for k, v := range h.introRefreshStatus {
		out[k] = v
	}
	return out
}

func (h *LibraryHandler) tryStartLibraryChromaprintRefresh(libraryID int) *playbackRefreshProgress {
	h.librarySideJobsMu.Lock()
	defer h.librarySideJobsMu.Unlock()
	if h.librarySideJobOccupied(libraryID) {
		return nil
	}
	if h.chromaprintRefreshStatus == nil {
		h.chromaprintRefreshStatus = make(map[int]*playbackRefreshProgress)
	}
	p := &playbackRefreshProgress{Total: 1}
	h.chromaprintRefreshStatus[libraryID] = p
	return p
}

func (h *LibraryHandler) finishLibraryChromaprintRefresh(libraryID int) {
	h.librarySideJobsMu.Lock()
	defer h.librarySideJobsMu.Unlock()
	delete(h.chromaprintRefreshStatus, libraryID)
}

func (h *LibraryHandler) getChromaprintRefreshStatuses() map[int]*playbackRefreshProgress {
	h.librarySideJobsMu.Lock()
	defer h.librarySideJobsMu.Unlock()
	out := make(map[int]*playbackRefreshProgress, len(h.chromaprintRefreshStatus))
	for k, v := range h.chromaprintRefreshStatus {
		out[k] = v
	}
	return out
}

func countNonMissingMedia(items []db.MediaItem) int {
	n := 0
	for i := range items {
		if !items[i].Missing {
			n++
		}
	}
	return n
}

// startLibraryIntroRefreshAsync re-probes intro bounds only (chapters + silence), without a full embedded-track refresh.
func (h *LibraryHandler) startLibraryIntroRefreshAsync(libraryID int) bool {
	items, itemsErr := db.GetMediaByLibraryID(h.DB, libraryID)
	if itemsErr != nil {
		return false
	}
	total := countNonMissingMedia(items)
	progress := h.tryStartLibraryIntroRefresh(libraryID, total)
	if progress == nil {
		return false
	}
	go func(libID int, mediaItems []db.MediaItem, prog *playbackRefreshProgress) {
		defer h.finishLibraryIntroRefresh(libID)
		ctx := context.Background()
		processed := 0
		for i := range mediaItems {
			it := mediaItems[i]
			if it.Missing {
				continue
			}
			prog.update(processed, it.Path)
			if err := db.RefreshIntroProbeOnly(ctx, h.DB, &it); err != nil {
				log.Printf("intro-only refresh library=%d media=%d: %v", libID, it.ID, err)
			}
			processed++
		}
		prog.update(total, "")
		log.Printf("library intro-only refresh library=%d done items=%d", libID, total)
	}(libraryID, items, progress)
	return true
}

func (h *LibraryHandler) userOwnedLibrarySet(userID int) (map[int]struct{}, error) {
	rows, err := h.DB.Query(`SELECT id FROM libraries WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	owned := make(map[int]struct{})
	for rows.Next() {
		var libraryID int
		if scanErr := rows.Scan(&libraryID); scanErr != nil {
			return nil, scanErr
		}
		owned[libraryID] = struct{}{}
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	return owned, nil
}

// startLibraryPlaybackRefreshAsync starts a background ffprobe/chapter refresh for one library.
// Returns false if a refresh is already running for that library or media listing fails.
func (h *LibraryHandler) startLibraryPlaybackRefreshAsync(libraryID int) bool {
	items, itemsErr := db.GetMediaByLibraryID(h.DB, libraryID)
	if itemsErr != nil {
		return false
	}
	progress := h.tryStartLibraryPlaybackRefresh(libraryID, len(items))
	if progress == nil {
		return false
	}
	go func(libID int, mediaItems []db.MediaItem, prog *playbackRefreshProgress) {
		defer h.finishLibraryPlaybackRefresh(libID)
		ctx := context.Background()
		n := len(mediaItems)
		if n == 0 {
			prog.update(0, "")
			log.Printf("library playback refresh library=%d done refreshed=0 failed=0 (empty)", libID)
			return
		}
		workers := playbackRefreshWorkerCount(n)
		if workers <= 1 {
			var refreshed, failed int
			for i := range mediaItems {
				it := mediaItems[i]
				prog.update(i, it.Path)
				_, rerr := db.RefreshPlaybackTrackMetadata(ctx, h.DB, &it)
				if rerr != nil {
					log.Printf("refresh playback tracks library=%d media=%d: %v", libID, it.ID, rerr)
					failed++
				} else {
					refreshed++
				}
			}
			prog.update(n, "")
			log.Printf("library playback refresh library=%d done refreshed=%d failed=%d", libID, refreshed, failed)
			return
		}
		jobs := make(chan int, workers)
		var wg sync.WaitGroup
		var failMu sync.Mutex
		var failed int
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for idx := range jobs {
					it := mediaItems[idx]
					_, rerr := db.RefreshPlaybackTrackMetadata(ctx, h.DB, &it)
					if rerr != nil {
						log.Printf("refresh playback tracks library=%d media=%d: %v", libID, it.ID, rerr)
						failMu.Lock()
						failed++
						failMu.Unlock()
					}
					prog.itemFinished(it.Path)
				}
			}()
		}
		for i := range mediaItems {
			jobs <- i
		}
		close(jobs)
		wg.Wait()
		prog.update(n, "")
		refreshed := n - failed
		log.Printf("library playback refresh library=%d done refreshed=%d failed=%d workers=%d", libID, refreshed, failed, workers)
	}(libraryID, items, progress)
	return true
}

func (h *LibraryHandler) RefreshLibraryPlaybackTracks(w http.ResponseWriter, r *http.Request) {
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
	if !h.startLibraryPlaybackRefreshAsync(libraryID) {
		writeJSON(w, http.StatusConflict, map[string]any{
			"accepted":  false,
			"libraryId": libraryID,
			"error":     "playback track refresh already running for this library",
		})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"accepted":  true,
		"libraryId": libraryID,
	})
}

func (h *LibraryHandler) RefreshLibraryIntroOnly(w http.ResponseWriter, r *http.Request) {
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
	if !h.startLibraryIntroRefreshAsync(libraryID) {
		writeJSON(w, http.StatusConflict, map[string]any{
			"accepted":  false,
			"libraryId": libraryID,
			"error":     "a library media job is already running for this library",
		})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"accepted":  true,
		"libraryId": libraryID,
	})
}

func (h *LibraryHandler) PostLibraryIntroChromaprintScan(w http.ResponseWriter, r *http.Request) {
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
	var body struct {
		ShowKey string `json:"show_key"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if !db.ChromaprintMuxersAvailable() {
		http.Error(w, db.ErrChromaprintUnavailable.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(db.IntroFingerprintCacheDir()) == "" {
		http.Error(w, "intro fingerprint cache directory unset (set PLUM_INTRO_FINGERPRINT_DIR or use a file-backed database path)", http.StatusBadRequest)
		return
	}
	prog := h.tryStartLibraryChromaprintRefresh(libraryID)
	if prog == nil {
		writeJSON(w, http.StatusConflict, map[string]any{
			"accepted":  false,
			"libraryId": libraryID,
			"error":     "a library media job is already running for this library",
		})
		return
	}
	showKey := strings.TrimSpace(body.ShowKey)
	go func(libID int, sk string, p *playbackRefreshProgress) {
		defer h.finishLibraryChromaprintRefresh(libID)
		ctx := context.Background()
		p.update(0, "chromaprint")
		cacheRoot := db.IntroFingerprintCacheDir()
		_, _, err := db.RunChromaprintIntroScanForLibrary(ctx, h.DB, libID, sk, cacheRoot)
		if err != nil {
			log.Printf("chromaprint intro scan library=%d: %v", libID, err)
		}
		p.update(1, "")
	}(libraryID, showKey, prog)
	writeJSON(w, http.StatusAccepted, map[string]any{
		"accepted":  true,
		"libraryId": libraryID,
	})
}

func (h *LibraryHandler) RefreshShow(w http.ResponseWriter, r *http.Request) {
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
	var payload refreshShowRequest
	if !decodeRequestJSON(w, r, &payload) {
		return
	}
	if payload.ShowKey == "" {
		http.Error(w, "showKey is required", http.StatusBadRequest)
		return
	}
	refs, err := db.ListShowEpisodeRefs(h.DB, libraryID, payload.ShowKey)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if len(refs) == 0 {
		writeJSON(w, http.StatusOK, showActionResult{Updated: 0})
		return
	}
	// Use first episode's TMDB ID (series id) for the show.
	seriesTMDBID := refs[0].TMDBID
	if h.SeriesQuery == nil || seriesTMDBID <= 0 {
		writeJSON(w, http.StatusOK, showActionResult{Updated: 0})
		return
	}
	updated, _ := h.applyTMDBSeriesToRefs(r.Context(), seriesTMDBID, refs, false, true)
	writeJSON(w, http.StatusOK, showActionResult{Updated: updated})
}

func (h *LibraryHandler) IdentifyShow(w http.ResponseWriter, r *http.Request) {
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
	var payload identifyShowRequest
	if !decodeRequestJSON(w, r, &payload) {
		return
	}
	payload.ShowKey = strings.TrimSpace(payload.ShowKey)
	payload.Provider = strings.ToLower(strings.TrimSpace(payload.Provider))
	payload.ExternalID = strings.TrimSpace(payload.ExternalID)
	if payload.Provider == "" && payload.TmdbID > 0 {
		payload.Provider = "tmdb"
	}
	if payload.ExternalID == "" && payload.TmdbID > 0 {
		payload.ExternalID = strconv.Itoa(payload.TmdbID)
	}
	if payload.ShowKey == "" || payload.Provider == "" || payload.ExternalID == "" {
		http.Error(w, "showKey, provider, and externalId are required", http.StatusBadRequest)
		return
	}
	refs, err := db.ListShowEpisodeRefs(h.DB, libraryID, payload.ShowKey)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if len(refs) == 0 {
		writeJSON(w, http.StatusOK, showActionResult{Updated: 0})
		return
	}
	if h.SeriesQuery == nil {
		http.Error(w, "metadata not configured", http.StatusServiceUnavailable)
		return
	}
	updated, _ := h.applySeriesMatchToRefs(r.Context(), payload.Provider, payload.ExternalID, refs, false, true)
	writeJSON(w, http.StatusOK, showActionResult{Updated: updated})
}

func (h *LibraryHandler) ConfirmShow(w http.ResponseWriter, r *http.Request) {
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
	var payload confirmShowRequest
	if !decodeRequestJSON(w, r, &payload) {
		return
	}
	if payload.ShowKey == "" {
		http.Error(w, "showKey is required", http.StatusBadRequest)
		return
	}
	refs, err := db.ListShowEpisodeRefs(h.DB, libraryID, payload.ShowKey)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if len(refs) == 0 {
		writeJSON(w, http.StatusOK, showActionResult{Updated: 0})
		return
	}
	refIDs := make([]int, 0, len(refs))
	for _, ref := range refs {
		refIDs = append(refIDs, ref.RefID)
	}
	updated, err := db.UpdateShowMetadataState(h.DB, db.MediaTableForKind(refs[0].Kind), refIDs, false, true)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, showActionResult{Updated: updated})
}

func (h *LibraryHandler) GetIntroScanSummary(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	summaries, err := db.ListIntroScanSummaries(h.DB, u.ID)
	if err != nil {
		log.Printf("list intro scan summaries: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if summaries == nil {
		summaries = []db.IntroScanLibrarySummary{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"libraries": summaries})
}

func (h *LibraryHandler) GetIntroScanShowSummary(w http.ResponseWriter, r *http.Request) {
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
	shows, err := db.ListIntroScanShowSummaries(h.DB, libraryID)
	if err != nil {
		log.Printf("list intro scan show summaries library=%d: %v", libraryID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if shows == nil {
		shows = []db.IntroScanShowSummary{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"shows": shows})
}

type introRefreshStatusEntry struct {
	LibraryID   int    `json:"library_id"`
	Total       int    `json:"total"`
	Processed   int    `json:"processed"`
	CurrentPath string `json:"current_path,omitempty"`
}

func (h *LibraryHandler) GetIntroRefreshStatus(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	ownedLibraryIDs, err := h.userOwnedLibrarySet(u.ID)
	if err != nil {
		log.Printf("list owned libraries user=%d: %v", u.ID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	build := func(statuses map[int]*playbackRefreshProgress) []introRefreshStatusEntry {
		entries := make([]introRefreshStatusEntry, 0, len(statuses))
		for libID, prog := range statuses {
			if _, ok := ownedLibraryIDs[libID]; !ok {
				continue
			}
			total, processed, currentPath := prog.snapshot()
			entries = append(entries, introRefreshStatusEntry{
				LibraryID:   libID,
				Total:       total,
				Processed:   processed,
				CurrentPath: currentPath,
			})
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].LibraryID < entries[j].LibraryID })
		return entries
	}
	playback := build(h.getPlaybackRefreshStatuses())
	introOnly := build(h.getIntroRefreshStatuses())
	chroma := build(h.getChromaprintRefreshStatuses())
	writeJSON(w, http.StatusOK, map[string]any{
		"libraries":             playback,
		"intro_only_libraries":  introOnly,
		"chromaprint_libraries": chroma,
	})
}
