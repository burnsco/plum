package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"plum/internal/db"
	"plum/internal/metadata"
)

type MetadataArtworkSettingsHandler struct {
	DB      *sql.DB
	Artwork metadata.MetadataArtworkProvider
}

type metadataArtworkSettingsResponse struct {
	Settings             db.MetadataArtworkSettings       `json:"settings"`
	ProviderAvailability []metadata.ArtworkProviderStatus `json:"provider_availability"`
}

type posterCandidateResponse struct {
	ID        string `json:"id"`
	Provider  string `json:"provider"`
	Label     string `json:"label"`
	ImageURL  string `json:"image_url"`
	SourceURL string `json:"source_url"`
	Selected  bool   `json:"selected"`
}

type posterCandidatesResponse struct {
	Candidates           []posterCandidateResponse        `json:"candidates"`
	ProviderAvailability []metadata.ArtworkProviderStatus `json:"provider_availability"`
	HasCustomSelection   bool                             `json:"has_custom_selection"`
}

type setPosterSelectionRequest struct {
	SourceURL string `json:"source_url"`
}

func defaultArtworkProviderStatuses() []metadata.ArtworkProviderStatus {
	return []metadata.ArtworkProviderStatus{
		{Provider: "fanart", Enabled: false, Available: false, Reason: "Metadata artwork provider unavailable"},
		{Provider: "tmdb", Enabled: false, Available: false, Reason: "Metadata artwork provider unavailable"},
		{Provider: "tvdb", Enabled: false, Available: false, Reason: "Metadata artwork provider unavailable"},
		{Provider: "omdb", Enabled: false, Available: false, Reason: "Metadata artwork provider unavailable"},
	}
}

func artworkProviderStatuses(artwork metadata.MetadataArtworkProvider) []metadata.ArtworkProviderStatus {
	if artwork == nil {
		return defaultArtworkProviderStatuses()
	}
	return artwork.ProviderStatuses()
}

func allowedShowArtworkProvider(fetchers db.ShowMetadataArtworkFetchers, provider string) bool {
	switch provider {
	case "fanart":
		return fetchers.Fanart
	case "tmdb":
		return fetchers.TMDB
	case "tvdb":
		return fetchers.TVDB
	default:
		return false
	}
}

func allowedMovieArtworkProvider(fetchers db.ShowMetadataArtworkFetchers, provider string) bool {
	return allowedShowArtworkProvider(fetchers, provider)
}

func allowedEpisodeArtworkProvider(fetchers db.EpisodeMetadataArtworkFetchers, provider string) bool {
	switch provider {
	case "tmdb":
		return fetchers.TMDB
	case "tvdb":
		return fetchers.TVDB
	case "omdb":
		return fetchers.OMDB
	default:
		return false
	}
}

func providerEnabledForSettings(settings db.MetadataArtworkSettings, provider string) bool {
	switch provider {
	case "fanart":
		return settings.Movies.Fanart || settings.Shows.Fanart || settings.Seasons.Fanart
	case "tmdb":
		return settings.Movies.TMDB || settings.Shows.TMDB || settings.Seasons.TMDB || settings.Episodes.TMDB
	case "tvdb":
		return settings.Movies.TVDB || settings.Shows.TVDB || settings.Seasons.TVDB || settings.Episodes.TVDB
	case "omdb":
		return settings.Episodes.OMDB
	default:
		return false
	}
}

func decorateArtworkProviderStatuses(
	statuses []metadata.ArtworkProviderStatus,
	settings db.MetadataArtworkSettings,
) []metadata.ArtworkProviderStatus {
	out := make([]metadata.ArtworkProviderStatus, len(statuses))
	for i, status := range statuses {
		out[i] = status
		out[i].Enabled = providerEnabledForSettings(settings, status.Provider)
	}
	return out
}

func firstPosterCandidateSource(candidates []metadata.PosterCandidate) string {
	for _, candidate := range candidates {
		sourceURL := strings.TrimSpace(candidate.SourceURL)
		if sourceURL == "" {
			sourceURL = strings.TrimSpace(candidate.ImageURL)
		}
		if sourceURL != "" {
			return sourceURL
		}
	}
	return ""
}

func firstAllowedPosterCandidate(
	candidates []metadata.PosterCandidate,
	allowed func(provider string) bool,
	fallbackSource string,
	fallbackProvider string,
) string {
	for _, candidate := range candidates {
		if !allowed(candidate.Provider) {
			continue
		}
		sourceURL := strings.TrimSpace(candidate.SourceURL)
		if sourceURL == "" {
			sourceURL = strings.TrimSpace(candidate.ImageURL)
		}
		if sourceURL != "" {
			return sourceURL
		}
	}
	if strings.TrimSpace(fallbackSource) != "" && allowed(fallbackProvider) {
		return strings.TrimSpace(fallbackSource)
	}
	return ""
}

func buildPosterCandidatesResponse(
	candidates []metadata.PosterCandidate,
	statuses []metadata.ArtworkProviderStatus,
	currentSource string,
	hasCustomSelection bool,
) posterCandidatesResponse {
	out := make([]posterCandidateResponse, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	currentSource = strings.TrimSpace(currentSource)
	for index, candidate := range candidates {
		sourceURL := strings.TrimSpace(candidate.SourceURL)
		if sourceURL == "" {
			sourceURL = strings.TrimSpace(candidate.ImageURL)
		}
		if sourceURL == "" {
			continue
		}
		key := candidate.Provider + ":" + sourceURL
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		imageURL := strings.TrimSpace(candidate.ImageURL)
		if imageURL == "" {
			imageURL = sourceURL
		}
		out = append(out, posterCandidateResponse{
			ID:        fmt.Sprintf("%s-%d", candidate.Provider, index+1),
			Provider:  candidate.Provider,
			Label:     candidate.Label,
			ImageURL:  imageURL,
			SourceURL: sourceURL,
			Selected:  currentSource != "" && currentSource == sourceURL,
		})
	}
	return posterCandidatesResponse{
		Candidates:           out,
		ProviderAvailability: statuses,
		HasCustomSelection:   hasCustomSelection,
	}
}

func loadMetadataArtworkSettings(dbConn *sql.DB) db.MetadataArtworkSettings {
	settings, err := db.GetMetadataArtworkSettings(dbConn)
	if err != nil {
		return db.DefaultMetadataArtworkSettings()
	}
	return settings
}

func automaticShowPosterSource(
	ctx context.Context,
	artwork metadata.MetadataArtworkProvider,
	settings db.MetadataArtworkSettings,
	title string,
	tmdbID int,
	tvdbID string,
	fallbackSource string,
	fallbackProvider string,
) string {
	if artwork == nil {
		if allowedShowArtworkProvider(settings.Shows, fallbackProvider) {
			return strings.TrimSpace(fallbackSource)
		}
		return ""
	}
	candidates, err := artwork.GetShowPosterCandidates(ctx, title, tmdbID, tvdbID)
	if err != nil {
		if allowedShowArtworkProvider(settings.Shows, fallbackProvider) {
			return strings.TrimSpace(fallbackSource)
		}
		return ""
	}
	return firstAllowedPosterCandidate(
		candidates,
		func(provider string) bool { return allowedShowArtworkProvider(settings.Shows, provider) },
		fallbackSource,
		fallbackProvider,
	)
}

func automaticMoviePosterSource(
	ctx context.Context,
	artwork metadata.MetadataArtworkProvider,
	settings db.MetadataArtworkSettings,
	tmdbID int,
	imdbID string,
	fallbackSource string,
	fallbackProvider string,
) string {
	if artwork == nil {
		if allowedMovieArtworkProvider(settings.Movies, fallbackProvider) {
			return strings.TrimSpace(fallbackSource)
		}
		return ""
	}
	candidates, err := artwork.GetMoviePosterCandidates(ctx, tmdbID, imdbID)
	if err != nil {
		if allowedMovieArtworkProvider(settings.Movies, fallbackProvider) {
			return strings.TrimSpace(fallbackSource)
		}
		return ""
	}
	return firstAllowedPosterCandidate(
		candidates,
		func(provider string) bool { return allowedMovieArtworkProvider(settings.Movies, provider) },
		fallbackSource,
		fallbackProvider,
	)
}

func automaticSeasonPosterSource(
	ctx context.Context,
	artwork metadata.MetadataArtworkProvider,
	settings db.MetadataArtworkSettings,
	title string,
	tmdbID int,
	tvdbID string,
	season int,
	fallbackSource string,
	fallbackProvider string,
) string {
	if artwork == nil {
		if allowedShowArtworkProvider(settings.Seasons, fallbackProvider) {
			return strings.TrimSpace(fallbackSource)
		}
		return ""
	}
	candidates, err := artwork.GetSeasonPosterCandidates(ctx, title, tmdbID, tvdbID, season)
	if err != nil {
		if allowedShowArtworkProvider(settings.Seasons, fallbackProvider) {
			return strings.TrimSpace(fallbackSource)
		}
		return ""
	}
	return firstAllowedPosterCandidate(
		candidates,
		func(provider string) bool { return allowedShowArtworkProvider(settings.Seasons, provider) },
		fallbackSource,
		fallbackProvider,
	)
}

func automaticEpisodePosterSource(
	ctx context.Context,
	artwork metadata.MetadataArtworkProvider,
	settings db.MetadataArtworkSettings,
	title string,
	tmdbID int,
	tvdbID string,
	imdbID string,
	season int,
	episode int,
	fallbackSource string,
	fallbackProvider string,
) string {
	if artwork == nil {
		if allowedEpisodeArtworkProvider(settings.Episodes, fallbackProvider) {
			return strings.TrimSpace(fallbackSource)
		}
		return ""
	}
	candidates, err := artwork.GetEpisodePosterCandidates(ctx, title, tmdbID, tvdbID, imdbID, season, episode)
	if err != nil {
		if allowedEpisodeArtworkProvider(settings.Episodes, fallbackProvider) {
			return strings.TrimSpace(fallbackSource)
		}
		return ""
	}
	return firstAllowedPosterCandidate(
		candidates,
		func(provider string) bool { return allowedEpisodeArtworkProvider(settings.Episodes, provider) },
		fallbackSource,
		fallbackProvider,
	)
}

func validatePosterSelection(sourceURL string, candidates []metadata.PosterCandidate) bool {
	sourceURL = strings.TrimSpace(sourceURL)
	if sourceURL == "" {
		return false
	}
	for _, candidate := range candidates {
		candidateSource := strings.TrimSpace(candidate.SourceURL)
		if candidateSource == "" {
			candidateSource = strings.TrimSpace(candidate.ImageURL)
		}
		if candidateSource == sourceURL {
			return true
		}
	}
	return false
}

func queuePosterRefresh(searchIndex *SearchIndexManager, libraryID int) {
	if searchIndex != nil {
		searchIndex.Queue(libraryID, false)
	}
}

func (h *MetadataArtworkSettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	settings, err := db.GetMetadataArtworkSettings(h.DB)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(metadataArtworkSettingsResponse{
		Settings:             settings,
		ProviderAvailability: decorateArtworkProviderStatuses(artworkProviderStatuses(h.Artwork), settings),
	})
}

func (h *MetadataArtworkSettingsHandler) Put(w http.ResponseWriter, r *http.Request) {
	var payload db.MetadataArtworkSettings
	if !decodeRequestJSON(w, r, &payload) {
		return
	}
	settings, err := db.SaveMetadataArtworkSettings(h.DB, payload)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(metadataArtworkSettingsResponse{
		Settings:             settings,
		ProviderAvailability: decorateArtworkProviderStatuses(artworkProviderStatuses(h.Artwork), settings),
	})
}

func (h *LibraryHandler) GetMoviePosterCandidates(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	libraryID, _, _, _, ok := h.authorizeLibraryRequest(w, r, user.ID)
	if !ok {
		return
	}
	mediaID, ok := parsePathInt(w, chi.URLParam(r, "mediaId"), "invalid media id")
	if !ok {
		return
	}
	target, err := db.GetMovieArtworkTarget(h.DB, libraryID, mediaID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if target == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	settings, err := db.GetMetadataArtworkSettings(h.DB)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	candidates := []metadata.PosterCandidate{}
	if h.Artwork != nil {
		candidates, err = h.Artwork.GetMoviePosterCandidates(r.Context(), target.TMDBID, target.IMDbID)
		if err != nil {
			http.Error(w, "artwork lookup failed", http.StatusBadGateway)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(buildPosterCandidatesResponse(
		candidates,
		decorateArtworkProviderStatuses(artworkProviderStatuses(h.Artwork), settings),
		target.PosterPath,
		target.PosterLocked,
	))
}

func (h *LibraryHandler) SetMoviePosterSelection(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	libraryID, _, _, _, ok := h.authorizeLibraryRequest(w, r, user.ID)
	if !ok {
		return
	}
	mediaID, ok := parsePathInt(w, chi.URLParam(r, "mediaId"), "invalid media id")
	if !ok {
		return
	}
	var payload setPosterSelectionRequest
	if !decodeRequestJSON(w, r, &payload) {
		return
	}
	if h.Artwork == nil {
		http.Error(w, "metadata artwork not configured", http.StatusServiceUnavailable)
		return
	}
	target, err := db.GetMovieArtworkTarget(h.DB, libraryID, mediaID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if target == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	candidates, err := h.Artwork.GetMoviePosterCandidates(r.Context(), target.TMDBID, target.IMDbID)
	if err != nil {
		http.Error(w, "artwork lookup failed", http.StatusBadGateway)
		return
	}
	if !validatePosterSelection(payload.SourceURL, candidates) {
		http.Error(w, "invalid poster selection", http.StatusBadRequest)
		return
	}
	if err := db.SetMoviePosterSelection(h.DB, target.RefID, payload.SourceURL, true); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	queuePosterRefresh(h.SearchIndex, libraryID)
	w.WriteHeader(http.StatusNoContent)
}

func (h *LibraryHandler) ResetMoviePosterSelection(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	libraryID, _, _, _, ok := h.authorizeLibraryRequest(w, r, user.ID)
	if !ok {
		return
	}
	mediaID, ok := parsePathInt(w, chi.URLParam(r, "mediaId"), "invalid media id")
	if !ok {
		return
	}
	target, err := db.GetMovieArtworkTarget(h.DB, libraryID, mediaID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if target == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	settings, err := db.GetMetadataArtworkSettings(h.DB)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	automaticSource := ""
	if h.Artwork != nil {
		candidates, err := h.Artwork.GetMoviePosterCandidates(r.Context(), target.TMDBID, target.IMDbID)
		if err != nil {
			http.Error(w, "artwork lookup failed", http.StatusBadGateway)
			return
		}
		automaticSource = firstAllowedPosterCandidate(candidates, func(provider string) bool {
			return allowedMovieArtworkProvider(settings.Movies, provider)
		}, "", "")
	}
	if err := db.SetMoviePosterSelection(h.DB, target.RefID, automaticSource, false); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	queuePosterRefresh(h.SearchIndex, libraryID)
	w.WriteHeader(http.StatusNoContent)
}

func (h *LibraryHandler) GetShowPosterCandidates(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	libraryID, _, _, _, ok := h.authorizeLibraryRequest(w, r, user.ID)
	if !ok {
		return
	}
	showKey := chi.URLParam(r, "showKey")
	target, err := db.GetShowArtworkTarget(h.DB, libraryID, showKey)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if target == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	settings, err := db.GetMetadataArtworkSettings(h.DB)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	candidates := []metadata.PosterCandidate{}
	if h.Artwork != nil {
		candidates, err = h.Artwork.GetShowPosterCandidates(r.Context(), target.Title, target.TMDBID, target.TVDBID)
		if err != nil {
			http.Error(w, "artwork lookup failed", http.StatusBadGateway)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(buildPosterCandidatesResponse(
		candidates,
		decorateArtworkProviderStatuses(artworkProviderStatuses(h.Artwork), settings),
		target.PosterPath,
		target.PosterLocked,
	))
}

func (h *LibraryHandler) SetShowPosterSelection(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	libraryID, _, _, _, ok := h.authorizeLibraryRequest(w, r, user.ID)
	if !ok {
		return
	}
	showKey := chi.URLParam(r, "showKey")
	var payload setPosterSelectionRequest
	if !decodeRequestJSON(w, r, &payload) {
		return
	}
	if h.Artwork == nil {
		http.Error(w, "metadata artwork not configured", http.StatusServiceUnavailable)
		return
	}
	target, err := db.GetShowArtworkTarget(h.DB, libraryID, showKey)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if target == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	candidates, err := h.Artwork.GetShowPosterCandidates(r.Context(), target.Title, target.TMDBID, target.TVDBID)
	if err != nil {
		http.Error(w, "artwork lookup failed", http.StatusBadGateway)
		return
	}
	if !validatePosterSelection(payload.SourceURL, candidates) {
		http.Error(w, "invalid poster selection", http.StatusBadRequest)
		return
	}
	if err := db.SetShowPosterSelection(h.DB, target.ID, payload.SourceURL, true); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	queuePosterRefresh(h.SearchIndex, libraryID)
	w.WriteHeader(http.StatusNoContent)
}

func (h *LibraryHandler) ResetShowPosterSelection(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	libraryID, _, _, _, ok := h.authorizeLibraryRequest(w, r, user.ID)
	if !ok {
		return
	}
	showKey := chi.URLParam(r, "showKey")
	target, err := db.GetShowArtworkTarget(h.DB, libraryID, showKey)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if target == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	settings, err := db.GetMetadataArtworkSettings(h.DB)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	automaticSource := ""
	if h.Artwork != nil {
		candidates, err := h.Artwork.GetShowPosterCandidates(r.Context(), target.Title, target.TMDBID, target.TVDBID)
		if err != nil {
			http.Error(w, "artwork lookup failed", http.StatusBadGateway)
			return
		}
		automaticSource = firstAllowedPosterCandidate(candidates, func(provider string) bool {
			return allowedShowArtworkProvider(settings.Shows, provider)
		}, "", "")
	}
	if err := db.SetShowPosterSelection(h.DB, target.ID, automaticSource, false); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	queuePosterRefresh(h.SearchIndex, libraryID)
	w.WriteHeader(http.StatusNoContent)
}
