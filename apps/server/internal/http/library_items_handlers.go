package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"plum/internal/db"
)

func (h *LibraryHandler) GetLibraryMovieDetails(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	libraryID, _, _, _, ok := h.authorizeLibraryRequest(w, r, u.ID)
	if !ok {
		return
	}
	mediaID, ok := chiURLIntParam(w, r, "mediaId", "media id")
	if !ok {
		return
	}
	details, err := db.GetLibraryMovieDetails(h.DB, libraryID, mediaID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if details == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err := db.AttachPlaybackProgressToLibraryMovieDetails(h.DB, u.ID, details.MediaID, details); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, details)
}

func (h *LibraryHandler) IdentifyMovie(w http.ResponseWriter, r *http.Request) {
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
	var payload identifyMovieRequest
	if !decodeRequestJSON(w, r, &payload) {
		return
	}
	payload.Provider = strings.ToLower(strings.TrimSpace(payload.Provider))
	payload.ExternalID = strings.TrimSpace(payload.ExternalID)
	if payload.Provider == "" && payload.TmdbID > 0 {
		payload.Provider = "tmdb"
	}
	if payload.ExternalID == "" && payload.TmdbID > 0 {
		payload.ExternalID = strconv.Itoa(payload.TmdbID)
	}
	if payload.MediaID <= 0 || payload.Provider == "" || payload.ExternalID == "" {
		http.Error(w, "mediaId, provider, and externalId are required", http.StatusBadRequest)
		return
	}
	if h.MovieLookup == nil {
		http.Error(w, "metadata not configured", http.StatusServiceUnavailable)
		return
	}

	var refID int
	// Browse/detail APIs expose media_global.id (g.id); movies row primary key is movies.id.
	err = h.DB.QueryRow(`
SELECT m.id FROM movies m
JOIN media_global g ON g.kind = 'movie' AND g.ref_id = m.id
WHERE m.library_id = ? AND g.id = ?`, libraryID, payload.MediaID).Scan(&refID)
	if err != nil {
		if err != sql.ErrNoRows {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		// Backwards compatibility if a client ever sent movies.id directly.
		err = h.DB.QueryRow(`SELECT id FROM movies WHERE library_id = ? AND id = ?`, libraryID, payload.MediaID).Scan(&refID)
	}
	if err != nil {
		if err == sql.ErrNoRows {
			w.Header().Set("Content-Type", "application/json")
			writeJSON(w, http.StatusOK, showActionResult{Updated: 0})
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	match, err := h.MovieLookup.GetMovie(r.Context(), payload.ExternalID)
	if err != nil {
		http.Error(w, "identify failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if match == nil {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, http.StatusOK, showActionResult{Updated: 0})
		return
	}

	tmdbID := 0
	if match.Provider == "tmdb" {
		tmdbID, _ = strconv.Atoi(match.ExternalID)
	}
	cast := make([]db.CastCredit, 0, len(match.Cast))
	for _, member := range match.Cast {
		cast = append(cast, db.CastCredit{
			Name:        member.Name,
			Character:   member.Character,
			Order:       member.Order,
			ProfilePath: member.ProfilePath,
			Provider:    member.Provider,
			ProviderID:  member.ProviderID,
		})
	}
	settings := loadMetadataArtworkSettings(h.DB)
	posterPath := automaticMoviePosterSource(
		r.Context(),
		h.Artwork,
		settings,
		tmdbID,
		match.IMDbID,
		match.PosterURL,
		match.Provider,
	)
	canonical := db.CanonicalMetadata{
		Title:        match.Title,
		Overview:     match.Overview,
		PosterPath:   posterPath,
		BackdropPath: match.BackdropURL,
		ReleaseDate:  match.ReleaseDate,
		VoteAverage:  match.VoteAverage,
		IMDbID:       match.IMDbID,
		IMDbRating:   match.IMDbRating,
		Genres:       match.Genres,
		Cast:         cast,
		Runtime:      match.Runtime,
	}
	if err := updateMetadataWithRetry(
		r.Context(),
		h.DB,
		db.MediaTableForKind(db.LibraryTypeMovie),
		refID,
		match.Title,
		match.Overview,
		posterPath,
		match.BackdropURL,
		match.ReleaseDate,
		match.VoteAverage,
		match.IMDbID,
		match.IMDbRating,
		tmdbID,
		"",
		0,
		0,
		canonical,
		false,
		true,
		true,
	); err != nil {
		http.Error(w, "identify failed", http.StatusInternalServerError)
		return
	}
	if h.SearchIndex != nil {
		h.SearchIndex.Queue(libraryID, false)
	}
	writeJSON(w, http.StatusOK, showActionResult{Updated: 1})
}

func (h *LibraryHandler) GetLibraryShowDetails(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	libraryID, _, _, _, ok := h.authorizeLibraryRequest(w, r, u.ID)
	if !ok {
		return
	}
	showKey := strings.TrimSpace(chi.URLParam(r, "showKey"))
	if showKey == "" {
		http.Error(w, "invalid show key", http.StatusBadRequest)
		return
	}
	details, err := db.GetLibraryShowDetails(h.DB, libraryID, showKey)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if details == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, details)
}

type showSeasonEpisodesResponse struct {
	SeasonNumber int                         `json:"seasonNumber"`
	Label        string                      `json:"label"`
	Episodes     []libraryBrowseItemResponse `json:"episodes"`
}

type showEpisodesResponse struct {
	Seasons []showSeasonEpisodesResponse `json:"seasons"`
}

func (h *LibraryHandler) GetLibraryShowEpisodes(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	libraryID, _, _, _, ok := h.authorizeLibraryRequest(w, r, u.ID)
	if !ok {
		return
	}
	showKey := strings.TrimSpace(chi.URLParam(r, "showKey"))
	if showKey == "" {
		http.Error(w, "invalid show key", http.StatusBadRequest)
		return
	}
	items, err := db.GetLibraryShowEpisodesForUser(h.DB, libraryID, u.ID, showKey)
	if err != nil {
		if errors.Is(err, db.ErrShowNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if identifyStates := h.identifyRun.stateForLibrary(libraryID); len(identifyStates) > 0 {
		for i := range items {
			if state, ok := identifyStates[identifyRowKey(items[i].Type, items[i].Path)]; ok {
				items[i].IdentifyState = state
			}
		}
	}
	bySeason := make(map[int][]libraryBrowseItemResponse)
	for _, item := range items {
		s := item.Season
		bySeason[s] = append(bySeason[s], buildLibraryBrowseItemResponse(item))
	}
	seasonNums := make([]int, 0, len(bySeason))
	for s := range bySeason {
		seasonNums = append(seasonNums, s)
	}
	sort.Ints(seasonNums)
	seasons := make([]showSeasonEpisodesResponse, 0, len(seasonNums))
	for _, s := range seasonNums {
		label := "Specials"
		if s != 0 {
			label = "Season " + strconv.Itoa(s)
		}
		seasons = append(seasons, showSeasonEpisodesResponse{
			SeasonNumber: s,
			Label:        label,
			Episodes:     bySeason[s],
		})
	}
	writeJSON(w, http.StatusOK, showEpisodesResponse{
		Seasons: seasons,
	})
}
