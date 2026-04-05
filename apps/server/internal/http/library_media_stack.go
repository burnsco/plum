package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"plum/internal/arr"
	"plum/internal/db"
	"plum/internal/metadata"
)

func loadMediaStackSettings(dbConn *sql.DB) db.MediaStackSettings {
	settings, err := db.GetEffectiveMediaStackSettings(dbConn)
	if err != nil {
		return db.DefaultMediaStackSettings()
	}
	return settings
}

func mediaTypeFromRoute(r *http.Request) (metadata.DiscoverMediaType, int, bool) {
	mediaType := metadata.DiscoverMediaType(strings.TrimSpace(chi.URLParam(r, "mediaType")))
	if mediaType != metadata.DiscoverMediaTypeMovie && mediaType != metadata.DiscoverMediaTypeTV {
		return "", 0, false
	}
	tmdbID, err := strconv.Atoi(chi.URLParam(r, "tmdbId"))
	if err != nil || tmdbID <= 0 {
		return "", 0, false
	}
	return mediaType, tmdbID, true
}

func (h *LibraryHandler) enrichDiscoverShelvesAcquisition(ctx context.Context, payload *metadata.DiscoverResponse) {
	if h == nil || h.Arr == nil || payload == nil {
		return
	}
	settings := loadMediaStackSettings(h.DB)
	snapshot, err := h.Arr.LoadSnapshot(ctx, settings)
	if err != nil {
		log.Printf("media stack snapshot: %v", err)
	}
	for shelfIndex := range payload.Shelves {
		for itemIndex := range payload.Shelves[shelfIndex].Items {
			item := &payload.Shelves[shelfIndex].Items[itemIndex]
			item.Acquisition = h.Arr.ResolveDiscoverAcquisition(
				item.MediaType,
				item.TMDBID,
				len(item.LibraryMatches) > 0,
				settings,
				snapshot,
			)
		}
	}
}

func (h *LibraryHandler) enrichDiscoverSearchAcquisition(ctx context.Context, payload *metadata.DiscoverSearchResponse) {
	if h == nil || h.Arr == nil || payload == nil {
		return
	}
	settings := loadMediaStackSettings(h.DB)
	snapshot, err := h.Arr.LoadSnapshot(ctx, settings)
	if err != nil {
		log.Printf("media stack snapshot: %v", err)
	}
	for index := range payload.Movies {
		item := &payload.Movies[index]
		item.Acquisition = h.Arr.ResolveDiscoverAcquisition(
			item.MediaType,
			item.TMDBID,
			len(item.LibraryMatches) > 0,
			settings,
			snapshot,
		)
	}
	for index := range payload.TV {
		item := &payload.TV[index]
		item.Acquisition = h.Arr.ResolveDiscoverAcquisition(
			item.MediaType,
			item.TMDBID,
			len(item.LibraryMatches) > 0,
			settings,
			snapshot,
		)
	}
}

func (h *LibraryHandler) enrichDiscoverItemsAcquisition(ctx context.Context, items []metadata.DiscoverItem) {
	if h == nil || h.Arr == nil || len(items) == 0 {
		return
	}
	settings := loadMediaStackSettings(h.DB)
	snapshot, err := h.Arr.LoadSnapshot(ctx, settings)
	if err != nil {
		log.Printf("media stack snapshot: %v", err)
	}
	for index := range items {
		item := &items[index]
		item.Acquisition = h.Arr.ResolveDiscoverAcquisition(
			item.MediaType,
			item.TMDBID,
			len(item.LibraryMatches) > 0,
			settings,
			snapshot,
		)
	}
}

func (h *LibraryHandler) enrichDiscoverTitleAcquisition(ctx context.Context, details *metadata.DiscoverTitleDetails) {
	if h == nil || h.Arr == nil || details == nil {
		return
	}
	settings := loadMediaStackSettings(h.DB)
	snapshot, err := h.Arr.LoadSnapshot(ctx, settings)
	if err != nil {
		log.Printf("media stack snapshot: %v", err)
	}
	details.Acquisition = h.Arr.ResolveDiscoverAcquisition(
		details.MediaType,
		details.TMDBID,
		len(details.LibraryMatches) > 0,
		settings,
		snapshot,
	)
}

func mediaStackServiceUnavailable(w http.ResponseWriter, message string) {
	http.Error(w, message, http.StatusServiceUnavailable)
}

func mediaStackUpstreamError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusBadGateway)
}

func (h *LibraryHandler) AddDiscoverTitle(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.Arr == nil {
		mediaStackServiceUnavailable(w, "media stack unavailable")
		return
	}

	mediaType, tmdbID, ok := mediaTypeFromRoute(r)
	if !ok {
		http.Error(w, "invalid media type or tmdb id", http.StatusBadRequest)
		return
	}

	settings, err := db.GetEffectiveMediaStackSettings(h.DB)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	snapshot, snapshotErr := h.Arr.LoadSnapshot(r.Context(), settings)
	if snapshotErr != nil {
		log.Printf("media stack snapshot before add: %v", snapshotErr)
	}
	acquisition := h.Arr.ResolveDiscoverAcquisition(mediaType, tmdbID, false, settings, snapshot)
	if acquisition != nil && acquisition.State != metadata.DiscoverAcquisitionStateNotAdded {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(acquisition)
		return
	}

	switch mediaType {
	case metadata.DiscoverMediaTypeMovie:
		if !arr.IsConfigured(settings.Radarr) {
			mediaStackServiceUnavailable(w, "radarr is not configured")
			return
		}
		if err := h.Arr.AddMovie(r.Context(), settings.Radarr, tmdbID); err != nil {
			mediaStackUpstreamError(w, err)
			return
		}
	case metadata.DiscoverMediaTypeTV:
		if !arr.IsConfigured(settings.SonarrTV) {
			mediaStackServiceUnavailable(w, "sonarr-tv is not configured")
			return
		}
		if h.Series == nil {
			http.Error(w, "series provider unavailable", http.StatusInternalServerError)
			return
		}
		details, err := h.Series.GetSeriesDetails(r.Context(), tmdbID)
		if err != nil {
			mediaStackUpstreamError(w, err)
			return
		}
		if details == nil || strings.TrimSpace(details.TVDBID) == "" {
			http.Error(w, "series details missing tvdb id", http.StatusBadGateway)
			return
		}
		if err := h.Arr.AddSeries(r.Context(), settings.SonarrTV, details.TVDBID); err != nil {
			mediaStackUpstreamError(w, err)
			return
		}
	default:
		http.Error(w, "invalid media type", http.StatusBadRequest)
		return
	}

	h.Arr.Invalidate()
	snapshot, snapshotErr = h.Arr.LoadSnapshot(r.Context(), settings)
	if snapshotErr != nil {
		log.Printf("media stack snapshot after add: %v", snapshotErr)
	}
	acquisition = h.Arr.ResolveDiscoverAcquisition(mediaType, tmdbID, false, settings, snapshot)
	if acquisition == nil {
		acquisition = &metadata.DiscoverAcquisition{
			State:        metadata.DiscoverAcquisitionStateAdded,
			IsConfigured: true,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(acquisition)
}

func (h *LibraryHandler) GetDownloads(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.Arr == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(arr.DownloadsResponse{
			Configured: false,
			Items:      []arr.DownloadItem{},
		})
		return
	}

	settings, err := db.GetEffectiveMediaStackSettings(h.DB)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	payload, err := h.Arr.GetDownloads(r.Context(), settings)
	if err != nil {
		log.Printf("media stack downloads partial response: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

type removeDownloadRequest struct {
	ID string `json:"id"`
}

func (h *LibraryHandler) RemoveDownload(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.Arr == nil {
		mediaStackServiceUnavailable(w, "media stack unavailable")
		return
	}
	var body removeDownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.ID) == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}
	settings, err := db.GetEffectiveMediaStackSettings(h.DB)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := h.Arr.RemoveQueueItem(r.Context(), settings, body.ID); err != nil {
		msg := err.Error()
		switch msg {
		case "invalid download id", "unknown download source":
			http.Error(w, msg, http.StatusBadRequest)
		case "radarr is not configured", "sonarr-tv is not configured":
			mediaStackServiceUnavailable(w, msg)
		default:
			mediaStackUpstreamError(w, err)
		}
		return
	}
	h.Arr.Invalidate()
	w.WriteHeader(http.StatusNoContent)
}
