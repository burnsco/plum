package httpapi

import (
	"context"
	"log"
	"net/http"
	"runtime"
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

// librarySideJobOccupied reports whether a playback job is in flight for libraryID.
// Caller must hold h.librarySideJobsMu.
func (h *LibraryHandler) librarySideJobOccupied(libraryID int) bool {
	if h.playbackRefreshStatus != nil {
		if _, ok := h.playbackRefreshStatus[libraryID]; ok {
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
