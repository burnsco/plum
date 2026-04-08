package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"plum/internal/db"
	"plum/internal/metadata"
)

func (h *LibraryHandler) GetDiscover(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.Discover == nil {
		http.Error(w, metadata.ErrTMDBNotConfigured.Error(), http.StatusServiceUnavailable)
		return
	}
	originCountry, err := parseDiscoverOriginCountry(r.URL.Query().Get("origin_country"))
	if err != nil {
		http.Error(w, "invalid origin_country", http.StatusBadRequest)
		return
	}

	payload, err := h.Discover.GetDiscover(r.Context(), originCountry)
	if err != nil {
		status, message := discoverHTTPStatus(err)
		http.Error(w, message, status)
		return
	}
	if payload == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	for i := range payload.Shelves {
		if err := db.AttachDiscoverLibraryMatches(h.DB, u.ID, payload.Shelves[i].Items); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	h.enrichDiscoverShelvesAcquisition(r.Context(), payload)
	writeJSON(w, http.StatusOK, payload)
}

func (h *LibraryHandler) GetDiscoverGenres(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.Discover == nil {
		http.Error(w, metadata.ErrTMDBNotConfigured.Error(), http.StatusServiceUnavailable)
		return
	}

	payload, err := h.Discover.GetDiscoverGenres(r.Context())
	if err != nil {
		status, message := discoverHTTPStatus(err)
		http.Error(w, message, status)
		return
	}
	if payload == nil {
		payload = &metadata.DiscoverGenresResponse{
			MovieGenres: []metadata.DiscoverGenre{},
			TVGenres:    []metadata.DiscoverGenre{},
		}
	}

	writeJSON(w, http.StatusOK, payload)
}

func parseDiscoverBrowseCategory(raw string) (metadata.DiscoverBrowseCategory, bool) {
	switch metadata.DiscoverBrowseCategory(strings.TrimSpace(raw)) {
	case "":
		return "", true
	case metadata.DiscoverBrowseCategoryTrending,
		metadata.DiscoverBrowseCategoryPopularMovies,
		metadata.DiscoverBrowseCategoryPopularTV,
		metadata.DiscoverBrowseCategoryNowPlaying,
		metadata.DiscoverBrowseCategoryUpcoming,
		metadata.DiscoverBrowseCategoryOnTheAir,
		metadata.DiscoverBrowseCategoryTopRated:
		return metadata.DiscoverBrowseCategory(strings.TrimSpace(raw)), true
	default:
		return "", false
	}
}

func parseDiscoverBrowseMediaType(raw string) (metadata.DiscoverMediaType, bool) {
	switch metadata.DiscoverMediaType(strings.TrimSpace(raw)) {
	case "":
		return "", true
	case metadata.DiscoverMediaTypeMovie, metadata.DiscoverMediaTypeTV:
		return metadata.DiscoverMediaType(strings.TrimSpace(raw)), true
	default:
		return "", false
	}
}

// parseDiscoverOriginCountry validates TMDB ISO 3166-1 alpha-2 origin filters. Rules must stay in
// sync with metadata.normalizeDiscoverOrigin, TS normalizeDiscoverOriginKey (packages/shared/src/discover.ts),
// and Kotlin DiscoverOrigin.normalizeKey.
func parseDiscoverOriginCountry(raw string) (string, error) {
	s := strings.TrimSpace(strings.ToUpper(raw))
	if s == "" {
		return "", nil
	}
	if len(s) != 2 {
		return "", errors.New("invalid origin_country")
	}
	for i := 0; i < 2; i++ {
		if s[i] < 'A' || s[i] > 'Z' {
			return "", errors.New("invalid origin_country")
		}
	}
	return s, nil
}

func (h *LibraryHandler) BrowseDiscover(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.Discover == nil {
		http.Error(w, metadata.ErrTMDBNotConfigured.Error(), http.StatusServiceUnavailable)
		return
	}

	category, ok := parseDiscoverBrowseCategory(r.URL.Query().Get("category"))
	if !ok {
		http.Error(w, "invalid discover category", http.StatusBadRequest)
		return
	}
	mediaType, ok := parseDiscoverBrowseMediaType(r.URL.Query().Get("media_type"))
	if !ok {
		http.Error(w, "invalid discover media type", http.StatusBadRequest)
		return
	}

	genreID := 0
	if rawGenre := strings.TrimSpace(r.URL.Query().Get("genre")); rawGenre != "" {
		parsedGenre, err := strconv.Atoi(rawGenre)
		if err != nil || parsedGenre <= 0 {
			http.Error(w, "invalid discover genre", http.StatusBadRequest)
			return
		}
		genreID = parsedGenre
	}

	page := 1
	if rawPage := strings.TrimSpace(r.URL.Query().Get("page")); rawPage != "" {
		parsedPage, err := strconv.Atoi(rawPage)
		if err != nil || parsedPage <= 0 {
			http.Error(w, "invalid discover page", http.StatusBadRequest)
			return
		}
		page = parsedPage
	}

	originCountry, err := parseDiscoverOriginCountry(r.URL.Query().Get("origin_country"))
	if err != nil {
		http.Error(w, "invalid origin_country", http.StatusBadRequest)
		return
	}

	payload, err := h.Discover.BrowseDiscover(r.Context(), category, mediaType, genreID, page, originCountry)
	if err != nil {
		status, message := discoverHTTPStatus(err)
		http.Error(w, message, status)
		return
	}
	if payload == nil {
		payload = &metadata.DiscoverBrowseResponse{
			Items:        []metadata.DiscoverItem{},
			Page:         page,
			TotalPages:   1,
			TotalResults: 0,
			Category:     category,
			MediaType:    mediaType,
		}
	}
	if err := db.AttachDiscoverLibraryMatches(h.DB, u.ID, payload.Items); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.enrichDiscoverItemsAcquisition(r.Context(), payload.Items)

	writeJSON(w, http.StatusOK, payload)
}

func (h *LibraryHandler) SearchDiscover(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeJSON(w, http.StatusOK, &metadata.DiscoverSearchResponse{
			Movies: []metadata.DiscoverItem{},
			TV:     []metadata.DiscoverItem{},
		})
		return
	}
	if h.Discover == nil {
		http.Error(w, metadata.ErrTMDBNotConfigured.Error(), http.StatusServiceUnavailable)
		return
	}
	payload, err := h.Discover.SearchDiscover(r.Context(), query)
	if err != nil {
		status, message := discoverHTTPStatus(err)
		http.Error(w, message, status)
		return
	}
	if payload == nil {
		payload = &metadata.DiscoverSearchResponse{
			Movies: []metadata.DiscoverItem{},
			TV:     []metadata.DiscoverItem{},
		}
	}
	if err := db.AttachDiscoverLibraryMatches(h.DB, u.ID, payload.Movies); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := db.AttachDiscoverLibraryMatches(h.DB, u.ID, payload.TV); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.enrichDiscoverSearchAcquisition(r.Context(), payload)
	writeJSON(w, http.StatusOK, payload)
}

func (h *LibraryHandler) GetDiscoverTitleDetails(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.Discover == nil {
		http.Error(w, metadata.ErrTMDBNotConfigured.Error(), http.StatusServiceUnavailable)
		return
	}

	mediaType := metadata.DiscoverMediaType(strings.TrimSpace(chi.URLParam(r, "mediaType")))
	if mediaType != metadata.DiscoverMediaTypeMovie && mediaType != metadata.DiscoverMediaTypeTV {
		http.Error(w, "invalid media type", http.StatusBadRequest)
		return
	}
	tmdbID, ok := chiURLIntParam(w, r, "tmdbId", "tmdb id")
	if !ok {
		return
	}

	details, err := h.Discover.GetDiscoverTitleDetails(r.Context(), mediaType, tmdbID)
	if err != nil {
		status, message := discoverHTTPStatus(err)
		http.Error(w, message, status)
		return
	}
	if details == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err := db.AttachDiscoverTitleLibraryMatches(h.DB, u.ID, details); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.enrichDiscoverTitleAcquisition(r.Context(), details)

	writeJSON(w, http.StatusOK, details)
}

func (h *LibraryHandler) UpdateMediaProgress(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	mediaID, ok := chiURLIntParamInvalidID(w, r, "id")
	if !ok {
		return
	}
	item, err := db.GetMediaByID(h.DB, mediaID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if item == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	var ownerID int
	if err := h.DB.QueryRow(`SELECT user_id FROM libraries WHERE id = ?`, item.LibraryID).Scan(&ownerID); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if ownerID != u.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var payload updateMediaProgressRequest
	if !decodeRequestJSON(w, r, &payload) {
		return
	}
	if err := db.UpsertPlaybackProgress(h.DB, u.ID, mediaID, payload.PositionSeconds, payload.DurationSeconds, payload.Completed); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *LibraryHandler) SetContinueWatchingVisibility(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	mediaID, ok := chiURLIntParamInvalidID(w, r, "id")
	if !ok {
		return
	}
	item, err := db.GetMediaByID(h.DB, mediaID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if item == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	var ownerID int
	if err := h.DB.QueryRow(`SELECT user_id FROM libraries WHERE id = ?`, item.LibraryID).Scan(&ownerID); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if ownerID != u.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var payload setContinueWatchingVisibilityRequest
	if !decodeRequestJSON(w, r, &payload) {
		return
	}
	if err := db.SetPlaybackProgressContinueWatchingHidden(h.DB, u.ID, mediaID, payload.Hidden); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *LibraryHandler) ClearMediaProgress(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	mediaID, ok := chiURLIntParamInvalidID(w, r, "id")
	if !ok {
		return
	}
	item, err := db.GetMediaByID(h.DB, mediaID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if item == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	var ownerID int
	if err := h.DB.QueryRow(`SELECT user_id FROM libraries WHERE id = ?`, item.LibraryID).Scan(&ownerID); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if ownerID != u.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := db.ResetPlaybackProgressForUser(h.DB, u.ID, mediaID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *LibraryHandler) ClearShowProgress(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	libraryID, ok := chiURLIntParamInvalidID(w, r, "id")
	if !ok {
		return
	}
	showKey := chi.URLParam(r, "showKey")
	if strings.TrimSpace(showKey) == "" {
		http.Error(w, "invalid show key", http.StatusBadRequest)
		return
	}
	var ownerID int
	var libraryType string
	if err := h.DB.QueryRow(`SELECT user_id, type FROM libraries WHERE id = ?`, libraryID).Scan(&ownerID, &libraryType); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if ownerID != u.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if libraryType != db.LibraryTypeTV && libraryType != db.LibraryTypeAnime {
		http.Error(w, "invalid media type", http.StatusBadRequest)
		return
	}
	if err := db.ClearShowPlaybackProgressForUser(h.DB, u.ID, libraryID, libraryType, showKey); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *LibraryHandler) MarkShowWatched(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	libraryID, ok := chiURLIntParamInvalidID(w, r, "id")
	if !ok {
		return
	}
	showKey := chi.URLParam(r, "showKey")
	if strings.TrimSpace(showKey) == "" {
		http.Error(w, "invalid show key", http.StatusBadRequest)
		return
	}
	var ownerID int
	var libraryType string
	if err := h.DB.QueryRow(`SELECT user_id, type FROM libraries WHERE id = ?`, libraryID).Scan(&ownerID, &libraryType); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if ownerID != u.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if libraryType != db.LibraryTypeTV && libraryType != db.LibraryTypeAnime {
		http.Error(w, "invalid media type", http.StatusBadRequest)
		return
	}
	var payload markShowWatchedRequest
	if !decodeRequestJSON(w, r, &payload) {
		return
	}
	mode := strings.TrimSpace(payload.Mode)
	switch mode {
	case "all":
		if err := db.MarkShowPlaybackWatched(h.DB, u.ID, libraryID, libraryType, showKey, mode, 0, 0); err != nil {
			if errors.Is(err, db.ErrShowNotFound) {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	case "season":
		if payload.Season == nil {
			http.Error(w, "season required", http.StatusBadRequest)
			return
		}
		if err := db.MarkShowPlaybackWatched(h.DB, u.ID, libraryID, libraryType, showKey, mode, *payload.Season, 0); err != nil {
			if errors.Is(err, db.ErrShowNotFound) {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	case "up_to":
		if payload.Season == nil || payload.Episode == nil {
			http.Error(w, "season and episode required", http.StatusBadRequest)
			return
		}
		if err := db.MarkShowPlaybackWatched(h.DB, u.ID, libraryID, libraryType, showKey, mode, *payload.Season, *payload.Episode); err != nil {
			if errors.Is(err, db.ErrShowNotFound) {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "invalid mode", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *LibraryHandler) GetMovieSearch(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, []metadata.MatchResult{})
		return
	}
	if h.MovieQuery == nil {
		writeJSON(w, http.StatusOK, []metadata.MatchResult{})
		return
	}
	results, err := h.MovieQuery.SearchMovie(r.Context(), q)
	if err != nil {
		http.Error(w, "search failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if results == nil {
		results = []metadata.MatchResult{}
	}
	writeJSON(w, http.StatusOK, results)
}

func (h *LibraryHandler) GetSeriesSearch(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSON(w, http.StatusOK, []metadata.MatchResult{})
		return
	}
	if h.SeriesQuery == nil {
		writeJSON(w, http.StatusOK, []metadata.MatchResult{})
		return
	}
	results, err := h.SeriesQuery.SearchTV(r.Context(), q)
	if err != nil {
		http.Error(w, "search failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if results == nil {
		results = []metadata.MatchResult{}
	}
	writeJSON(w, http.StatusOK, results)
}

func (h *LibraryHandler) SearchLibraryMedia(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	libraryID := 0
	if rawLibraryID := strings.TrimSpace(r.URL.Query().Get("library_id")); rawLibraryID != "" {
		parsed, err := strconv.Atoi(rawLibraryID)
		if err != nil || parsed <= 0 {
			http.Error(w, "invalid library_id", http.StatusBadRequest)
			return
		}
		libraryID = parsed
	}
	limit := 30
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		limit = parsed
	}
	searchType := strings.TrimSpace(r.URL.Query().Get("type"))
	if searchType != "" && searchType != "movie" && searchType != "show" {
		http.Error(w, "invalid type", http.StatusBadRequest)
		return
	}
	results, err := db.SearchLibraryMedia(h.DB, db.SearchQuery{
		UserID:    u.ID,
		Query:     strings.TrimSpace(r.URL.Query().Get("q")),
		LibraryID: libraryID,
		Type:      searchType,
		Genre:     strings.TrimSpace(r.URL.Query().Get("genre")),
		Limit:     limit,
	})
	if err != nil {
		http.Error(w, "search failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, results)
}
