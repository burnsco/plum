package db

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

type LibraryMovieDetails struct {
	MediaID      int               `json:"media_id"`
	LibraryID    int               `json:"library_id"`
	Title        string            `json:"title"`
	Overview     string            `json:"overview"`
	PosterPath   string            `json:"poster_path,omitempty"`
	PosterURL    string            `json:"poster_url,omitempty"`
	BackdropPath string            `json:"backdrop_path,omitempty"`
	BackdropURL  string            `json:"backdrop_url,omitempty"`
	ReleaseDate  string            `json:"release_date,omitempty"`
	IMDbID       string            `json:"imdb_id,omitempty"`
	IMDbRating   float64           `json:"imdb_rating,omitempty"`
	Runtime      int               `json:"runtime,omitempty"`
	Genres       []string          `json:"genres"`
	Cast         []TitleCastMember `json:"cast"`
}

type LibraryShowDetails struct {
	LibraryID        int               `json:"library_id"`
	ShowKey          string            `json:"show_key"`
	Name             string            `json:"name"`
	Overview         string            `json:"overview"`
	PosterPath       string            `json:"poster_path,omitempty"`
	PosterURL        string            `json:"poster_url,omitempty"`
	BackdropPath     string            `json:"backdrop_path,omitempty"`
	BackdropURL      string            `json:"backdrop_url,omitempty"`
	FirstAirDate     string            `json:"first_air_date,omitempty"`
	IMDbID           string            `json:"imdb_id,omitempty"`
	IMDbRating       float64           `json:"imdb_rating,omitempty"`
	Runtime          int               `json:"runtime,omitempty"`
	NumberOfSeasons  int               `json:"number_of_seasons"`
	NumberOfEpisodes int               `json:"number_of_episodes"`
	Genres           []string          `json:"genres"`
	Cast             []TitleCastMember `json:"cast"`
}

type SearchResult struct {
	Kind         string   `json:"kind"`
	LibraryID    int      `json:"library_id"`
	LibraryName  string   `json:"library_name"`
	LibraryType  string   `json:"library_type"`
	Title        string   `json:"title"`
	Subtitle     string   `json:"subtitle,omitempty"`
	PosterPath   string   `json:"poster_path,omitempty"`
	PosterURL    string   `json:"poster_url,omitempty"`
	IMDbRating   float64  `json:"imdb_rating,omitempty"`
	MatchReason  string   `json:"match_reason"`
	MatchedActor string   `json:"matched_actor,omitempty"`
	Href         string   `json:"href"`
	Genres       []string `json:"genres,omitempty"`
}

type SearchFacetValue struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

type SearchFacets struct {
	Libraries []SearchFacetValue `json:"libraries"`
	Types     []SearchFacetValue `json:"types"`
	Genres    []SearchFacetValue `json:"genres"`
}

type SearchResponse struct {
	Query   string         `json:"query"`
	Results []SearchResult `json:"results"`
	Total   int            `json:"total"`
	Facets  SearchFacets   `json:"facets"`
}

type SearchQuery struct {
	UserID    int
	Query     string
	LibraryID int
	Type      string
	Genre     string
	Limit     int
}

type searchDocument struct {
	DocKey      string
	Kind        string
	LibraryID   int
	LibraryName string
	LibraryType string
	Title       string
	Normalized  string
	Subtitle    string
	PosterPath  string
	PosterURL   string
	IMDbRating  float64
	Href        string
	Genres      []string
	Cast        []TitleCastMember
}

func normalizeSearchText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastSpace := true
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			b.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

func buildFTSQuery(query string) string {
	tokens := strings.Fields(normalizeSearchText(query))
	if len(tokens) == 0 {
		return ""
	}
	parts := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if token == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf(`"%s"*`, token))
	}
	return strings.Join(parts, " AND ")
}

func appendSearchDocumentFilters(clauses []string, args []interface{}, query SearchQuery, docAlias string) ([]string, []interface{}) {
	if query.UserID > 0 {
		clauses = append(clauses, "l.user_id = ?")
		args = append(args, query.UserID)
	}
	if query.LibraryID > 0 {
		clauses = append(clauses, docAlias+`.library_id = ?`)
		args = append(args, query.LibraryID)
	}
	if query.Type != "" {
		clauses = append(clauses, docAlias+`.kind = ?`)
		args = append(args, query.Type)
	}
	if genre := genreSlug(query.Genre); genre != "" {
		clauses = append(clauses, `EXISTS (
SELECT 1
FROM search_document_genres sdg
WHERE sdg.doc_key = `+docAlias+`.doc_key AND sdg.genre_slug = ?
)`)
		args = append(args, genre)
	}
	return clauses, args
}

func queryTitleMatchDocKeys(db *sql.DB, searchQuery SearchQuery, ftsQuery string, limit int) ([]string, error) {
	query := `SELECT search_documents_fts.doc_key
FROM search_documents_fts
JOIN search_documents sd ON sd.doc_key = search_documents_fts.doc_key
JOIN libraries l ON l.id = sd.library_id`
	clauses, args := appendSearchDocumentFilters([]string{`search_documents_fts MATCH ?`}, []interface{}{ftsQuery}, searchQuery, "sd")
	query += " WHERE " + strings.Join(clauses, " AND ") + " LIMIT ?"
	args = append(args, limit)
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	docKeys := make([]string, 0, limit)
	for rows.Next() {
		var docKey string
		if err := rows.Scan(&docKey); err != nil {
			return nil, err
		}
		docKeys = append(docKeys, docKey)
	}
	return docKeys, rows.Err()
}

type searchActorMatch struct {
	DocKey     string
	PersonName string
}

func queryActorMatches(db *sql.DB, searchQuery SearchQuery, ftsQuery string, limit int) ([]searchActorMatch, error) {
	query := `SELECT search_people_fts.doc_key, search_people_fts.person_name
FROM search_people_fts
JOIN search_documents sd ON sd.doc_key = search_people_fts.doc_key
JOIN libraries l ON l.id = sd.library_id`
	clauses, args := appendSearchDocumentFilters([]string{`search_people_fts MATCH ?`}, []interface{}{ftsQuery}, searchQuery, "sd")
	query += " WHERE " + strings.Join(clauses, " AND ") + " LIMIT ?"
	args = append(args, limit)
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	matches := make([]searchActorMatch, 0, limit)
	for rows.Next() {
		var match searchActorMatch
		if err := rows.Scan(&match.DocKey, &match.PersonName); err != nil {
			return nil, err
		}
		matches = append(matches, match)
	}
	return matches, rows.Err()
}

func RefreshLibrarySearchIndex(ctx context.Context, db *sql.DB, libraryID int) error {
	var libraryName, libraryType string
	if err := db.QueryRowContext(ctx, `SELECT name, type FROM libraries WHERE id = ?`, libraryID).Scan(&libraryName, &libraryType); err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM search_document_genres WHERE doc_key IN (SELECT doc_key FROM search_documents WHERE library_id = ?)`, libraryID); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM search_document_cast WHERE doc_key IN (SELECT doc_key FROM search_documents WHERE library_id = ?)`, libraryID); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM search_documents_fts WHERE doc_key IN (SELECT doc_key FROM search_documents WHERE library_id = ?)`, libraryID); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM search_people_fts WHERE doc_key IN (SELECT doc_key FROM search_documents WHERE library_id = ?)`, libraryID); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM search_documents WHERE library_id = ?`, libraryID); err != nil {
		_ = tx.Rollback()
		return err
	}
	if libraryType == LibraryTypeMusic {
		return tx.Commit()
	}

	docs, err := buildSearchDocuments(db, libraryID, libraryName, libraryType)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, doc := range docs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO search_documents (
doc_key, kind, library_id, library_name, library_type, title, normalized_title, subtitle,
poster_path, poster_url, imdb_rating, href, show_key, media_id, title_ref_id, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			doc.DocKey,
			doc.Kind,
			doc.LibraryID,
			doc.LibraryName,
			doc.LibraryType,
			doc.Title,
			doc.Normalized,
			doc.Subtitle,
			doc.PosterPath,
			doc.PosterURL,
			doc.IMDbRating,
			doc.Href,
			extractDocShowKey(doc),
			extractDocMediaID(doc),
			extractDocTitleRefID(doc),
			now,
		); err != nil {
			_ = tx.Rollback()
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO search_documents_fts (doc_key, title, normalized_title) VALUES (?, ?, ?)`,
			doc.DocKey, doc.Title, doc.Normalized); err != nil {
			_ = tx.Rollback()
			return err
		}
		for _, genre := range doc.Genres {
			slug := genreSlug(genre)
			if slug == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO search_document_genres (doc_key, genre_slug, genre_name) VALUES (?, ?, ?)`, doc.DocKey, slug, genre); err != nil {
				_ = tx.Rollback()
				return err
			}
		}
		for _, castMember := range doc.Cast {
			nameKey := personNameKey(castMember.Name)
			if nameKey == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO search_document_cast (doc_key, person_name, person_name_key, billing_order, character_name) VALUES (?, ?, ?, ?, ?)`,
				doc.DocKey, castMember.Name, nameKey, castMember.Order, nullStr(castMember.Character)); err != nil {
				_ = tx.Rollback()
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO search_people_fts (doc_key, person_name, person_name_key) VALUES (?, ?, ?)`,
				doc.DocKey, castMember.Name, nameKey); err != nil {
				_ = tx.Rollback()
				return err
			}
		}
	}
	return tx.Commit()
}

func extractDocMediaID(doc searchDocument) int {
	if strings.HasPrefix(doc.DocKey, "movie:") {
		parts := strings.Split(doc.DocKey, ":")
		if len(parts) >= 4 {
			id, _ := strconv.Atoi(parts[2])
			return id
		}
	}
	return 0
}

func extractDocTitleRefID(doc searchDocument) int {
	if strings.HasPrefix(doc.DocKey, "movie:") {
		parts := strings.Split(doc.DocKey, ":")
		if len(parts) >= 4 {
			id, _ := strconv.Atoi(parts[3])
			return id
		}
	}
	if strings.HasPrefix(doc.DocKey, "show:") {
		parts := strings.Split(doc.DocKey, ":")
		if len(parts) >= 3 {
			id, _ := strconv.Atoi(parts[2])
			return id
		}
	}
	return 0
}

func extractDocShowKey(doc searchDocument) string {
	if !strings.HasPrefix(doc.DocKey, "show:") {
		return ""
	}
	parts := strings.SplitN(doc.DocKey, ":", 4)
	if len(parts) < 4 {
		return ""
	}
	return parts[3]
}

func buildSearchDocuments(db *sql.DB, libraryID int, libraryName, libraryType string) ([]searchDocument, error) {
	items, err := queryMediaByLibraryID(db, libraryID, libraryType)
	if err != nil {
		return nil, err
	}
	if libraryType == LibraryTypeMovie {
		docs := make([]searchDocument, 0, len(items))
		for _, item := range items {
			refID, err := refIDForGlobalMediaID(db, item.ID)
			if err != nil {
				return nil, err
			}
			genres, err := loadTitleGenres(db, "movie", refID)
			if err != nil {
				return nil, err
			}
			cast, err := loadTitleCast(db, "movie", refID)
			if err != nil {
				return nil, err
			}
			doc := searchDocument{
				DocKey:      fmt.Sprintf("movie:%d:%d:%d", libraryID, item.ID, refID),
				Kind:        "movie",
				LibraryID:   libraryID,
				LibraryName: libraryName,
				LibraryType: libraryType,
				Title:       item.Title,
				Normalized:  normalizeSearchText(item.Title),
				Subtitle:    titleSubtitle(releaseYearFromDate(item.ReleaseDate), ""),
				PosterPath:  item.PosterPath,
				PosterURL:   item.PosterURL,
				IMDbRating:  item.IMDbRating,
				Href:        fmt.Sprintf("/library/%d/movie/%d", libraryID, item.ID),
				Genres:      titleGenreNames(genres),
				Cast:        cast,
			}
			docs = append(docs, doc)
		}
		return docs, nil
	}

	type showAggregate struct {
		showKey   string
		title     string
		poster    string
		backdrop  string
		release   string
		imdbID    string
		imdb      float64
		runtime   int
		episodeCt int
		seasonSet map[int]struct{}
		tmdbID    int
	}
	showMap := make(map[string]*showAggregate)
	for _, item := range items {
		showKey := showKeyFromItem(item.TMDBID, item.Title)
		agg := showMap[showKey]
		if agg == nil {
			agg = &showAggregate{
				showKey:   showKey,
				title:     showNameFromTitle(item.Title),
				seasonSet: map[int]struct{}{},
				tmdbID:    item.TMDBID,
			}
			showMap[showKey] = agg
		}
		if agg.poster == "" {
			if item.ShowPosterPath != "" {
				agg.poster = item.ShowPosterPath
			} else {
				agg.poster = item.PosterPath
			}
		}
		if agg.backdrop == "" {
			agg.backdrop = item.BackdropPath
		}
		if agg.release == "" {
			agg.release = item.ReleaseDate
		}
		if agg.imdbID == "" {
			agg.imdbID = item.IMDbID
		}
		if agg.imdb == 0 {
			agg.imdb = item.IMDbRating
		}
		if agg.runtime == 0 && item.Duration > 0 {
			agg.runtime = item.Duration / 60
		}
		agg.episodeCt++
		agg.seasonSet[item.Season] = struct{}{}
	}

	keys := make([]string, 0, len(showMap))
	for key := range showMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	docs := make([]searchDocument, 0, len(keys))
	for _, showKey := range keys {
		agg := showMap[showKey]
		showID, showTitle, overview, posterPath, backdropPath, airDate, imdbID, imdbRating, genres, cast, err := getShowCanonicalMetadata(db, libraryID, libraryType, showKey)
		if err != nil {
			return nil, err
		}
		if showTitle != "" {
			agg.title = showTitle
		}
		if posterPath != "" {
			agg.poster = posterPath
		}
		if backdropPath != "" {
			agg.backdrop = backdropPath
		}
		if airDate != "" {
			agg.release = airDate
		}
		if imdbID != "" {
			agg.imdbID = imdbID
		}
		if imdbRating > 0 {
			agg.imdb = imdbRating
		}
		doc := searchDocument{
			DocKey:      fmt.Sprintf("show:%d:%d:%s", libraryID, showID, showKey),
			Kind:        "show",
			LibraryID:   libraryID,
			LibraryName: libraryName,
			LibraryType: libraryType,
			Title:       agg.title,
			Normalized:  normalizeSearchText(agg.title),
			Subtitle:    titleSubtitle(releaseYearFromDate(agg.release), fmt.Sprintf("%d episodes", agg.episodeCt)),
			PosterPath:  agg.poster,
			PosterURL:   showPosterDisplayURL(libraryID, showKey, agg.poster),
			IMDbRating:  agg.imdb,
			Href:        fmt.Sprintf("/library/%d/show/%s", libraryID, showKey),
			Genres:      genres,
			Cast:        cast,
		}
		if strings.TrimSpace(overview) == "" {
			_ = overview
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

func titleGenreNames(genres []TitleGenre) []string {
	out := make([]string, 0, len(genres))
	for _, genre := range genres {
		out = append(out, genre.Name)
	}
	return out
}

func releaseYearFromDate(value string) int {
	if len(value) < 4 {
		return 0
	}
	year, _ := strconv.Atoi(value[:4])
	return year
}

func getShowCanonicalMetadata(db *sql.DB, libraryID int, libraryType string, showKey string) (showID int, title string, overview string, posterPath string, backdropPath string, airDate string, imdbID string, imdbRating float64, genres []string, cast []TitleCastMember, err error) {
	query := `SELECT id, title, COALESCE(overview, ''), COALESCE(poster_path, ''), COALESCE(backdrop_path, ''), COALESCE(first_air_date, ''), COALESCE(imdb_id, ''), COALESCE(imdb_rating, 0)
FROM shows
WHERE library_id = ? AND kind = ?`
	args := []interface{}{libraryID, libraryType}
	if strings.HasPrefix(showKey, "tmdb-") {
		tmdbID, parseErr := strconv.Atoi(strings.TrimPrefix(showKey, "tmdb-"))
		if parseErr != nil {
			return 0, "", "", "", "", "", "", 0, nil, nil, nil
		}
		query += ` AND tmdb_id = ? LIMIT 1`
		args = append(args, tmdbID)
	} else {
		query += ` AND title_key = ? LIMIT 1`
		args = append(args, strings.TrimPrefix(showKey, "title-"))
	}
	err = db.QueryRow(query, args...).Scan(&showID, &title, &overview, &posterPath, &backdropPath, &airDate, &imdbID, &imdbRating)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, "", "", "", "", "", "", 0, nil, nil, nil
		}
		return 0, "", "", "", "", "", "", 0, nil, nil, err
	}
	genreRows, err := loadTitleGenres(db, "show", showID)
	if err != nil {
		return 0, "", "", "", "", "", "", 0, nil, nil, err
	}
	cast, err = loadTitleCast(db, "show", showID)
	if err != nil {
		return 0, "", "", "", "", "", "", 0, nil, nil, err
	}
	return showID, title, overview, posterPath, backdropPath, airDate, imdbID, imdbRating, titleGenreNames(genreRows), cast, nil
}

func GetLibraryMovieDetails(db *sql.DB, libraryID int, mediaID int) (*LibraryMovieDetails, error) {
	var details LibraryMovieDetails
	var refID int
	err := db.QueryRow(`SELECT m.id, m.library_id, g.ref_id, m.title, COALESCE(m.overview, ''), COALESCE(m.poster_path, ''), COALESCE(m.poster_url, ''), COALESCE(m.backdrop_path, ''), COALESCE(m.backdrop_url, ''), COALESCE(m.release_date, ''), COALESCE(m.imdb_id, ''), COALESCE(m.imdb_rating, 0), COALESCE(m.duration, 0)
FROM (
  SELECT mg.id, movies.id AS ref_id, movies.library_id, movies.title, movies.overview, movies.poster_path, '' AS poster_url, movies.backdrop_path, '' AS backdrop_url, movies.release_date, movies.imdb_id, movies.imdb_rating, movies.duration
  FROM media_global mg
  JOIN movies ON mg.kind = 'movie' AND mg.ref_id = movies.id
) m
JOIN media_global g ON g.id = m.id
WHERE m.library_id = ? AND m.id = ?`, libraryID, mediaID).
		Scan(&details.MediaID, &details.LibraryID, &refID, &details.Title, &details.Overview, &details.PosterPath, &details.PosterURL, &details.BackdropPath, &details.BackdropURL, &details.ReleaseDate, &details.IMDbID, &details.IMDbRating, &details.Runtime)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if details.Runtime > 0 {
		details.Runtime /= 60
	}
	genres, err := loadTitleGenres(db, "movie", refID)
	if err != nil {
		return nil, err
	}
	cast, err := loadTitleCast(db, "movie", refID)
	if err != nil {
		return nil, err
	}
	details.Genres = titleGenreNames(genres)
	details.Cast = cast
	if strings.TrimSpace(details.PosterPath) != "" {
		details.PosterURL = fmt.Sprintf("/api/media/%d/artwork/poster", details.MediaID)
	}
	if strings.TrimSpace(details.BackdropPath) != "" {
		details.BackdropURL = fmt.Sprintf("/api/media/%d/artwork/backdrop", details.MediaID)
	}
	return &details, nil
}

func GetLibraryShowDetails(db *sql.DB, libraryID int, showKey string) (*LibraryShowDetails, error) {
	var libraryType string
	if err := db.QueryRow(`SELECT type FROM libraries WHERE id = ?`, libraryID).Scan(&libraryType); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	items, err := queryMediaByLibraryID(db, libraryID, libraryType)
	if err != nil {
		return nil, err
	}
	filtered := make([]MediaItem, 0)
	for _, item := range items {
		if showKeyFromItem(item.TMDBID, item.Title) == showKey {
			filtered = append(filtered, item)
		}
	}
	if len(filtered) == 0 {
		return nil, nil
	}
	showID, title, overview, posterPath, backdropPath, airDate, imdbID, imdbRating, genres, cast, err := getShowCanonicalMetadata(db, libraryID, libraryType, showKey)
	if err != nil {
		return nil, err
	}
	first := filtered[0]
	details := &LibraryShowDetails{
		LibraryID:    libraryID,
		ShowKey:      showKey,
		Name:         first.Title,
		PosterPath:   first.ShowPosterPath,
		BackdropPath: first.BackdropPath,
		FirstAirDate: first.ReleaseDate,
		IMDbID:       first.IMDbID,
		IMDbRating:   first.IMDbRating,
		Runtime:      first.Duration / 60,
		Genres:       genres,
		Cast:         cast,
	}
	if title != "" {
		details.Name = title
	}
	if details.PosterPath == "" {
		details.PosterPath = first.PosterPath
	}
	if overview != "" {
		details.Overview = overview
	} else {
		details.Overview = first.Overview
	}
	if posterPath != "" {
		details.PosterPath = posterPath
	}
	if backdropPath != "" {
		details.BackdropPath = backdropPath
	}
	if airDate != "" {
		details.FirstAirDate = airDate
	}
	if imdbID != "" {
		details.IMDbID = imdbID
	}
	if imdbRating > 0 {
		details.IMDbRating = imdbRating
	}
	if strings.TrimSpace(details.PosterPath) != "" {
		details.PosterURL = showPosterDisplayURL(libraryID, showKey, details.PosterPath)
	}
	seasonSet := map[int]struct{}{}
	for _, item := range filtered {
		seasonSet[item.Season] = struct{}{}
		if details.Runtime == 0 && item.Duration > 0 {
			details.Runtime = item.Duration / 60
		}
	}
	details.NumberOfEpisodes = len(filtered)
	details.NumberOfSeasons = len(seasonSet)
	if showID > 0 && len(details.Genres) == 0 {
		_ = showID
	}
	return details, nil
}

func SearchLibraryMedia(db *sql.DB, query SearchQuery) (SearchResponse, error) {
	limit := query.Limit
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	trimmed := strings.TrimSpace(query.Query)
	response := SearchResponse{
		Query:   trimmed,
		Results: []SearchResult{},
		Facets:  SearchFacets{},
	}
	if len(trimmed) < 2 {
		return response, nil
	}

	docs, err := listSearchDocuments(db, query.UserID, query.LibraryID, query.Type, query.Genre)
	if err != nil {
		return response, err
	}
	docByKey := make(map[string]searchDocument, len(docs))
	for _, doc := range docs {
		docByKey[doc.DocKey] = doc
	}

	ftsQuery := buildFTSQuery(trimmed)
	candidates := make(map[string]SearchResult)
	if ftsQuery != "" {
		titleDocKeys, err := queryTitleMatchDocKeys(db, query, ftsQuery, limit*3)
		if err != nil {
			return response, err
		}
		for _, docKey := range titleDocKeys {
			doc, ok := docByKey[docKey]
			if !ok {
				continue
			}
			candidates[docKey] = toSearchResult(doc, "title", "")
		}

		actorMatches, err := queryActorMatches(db, query, ftsQuery, limit*4)
		if err != nil {
			return response, err
		}
		for _, match := range actorMatches {
			docKey := match.DocKey
			if _, ok := candidates[docKey]; ok {
				continue
			}
			doc, ok := docByKey[docKey]
			if !ok {
				continue
			}
			candidates[docKey] = toSearchResult(doc, "actor", match.PersonName)
		}
	}

	if len(candidates) < limit {
		fuzzy := fuzzySearchDocuments(docs, trimmed, limit)
		for _, result := range fuzzy {
			if hasSearchResultForHref(candidates, result.Href) {
				continue
			}
			candidates["fuzzy:"+result.Href] = result
		}
	}

	results := make([]SearchResult, 0, len(candidates))
	for _, result := range candidates {
		results = append(results, result)
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].MatchReason != results[j].MatchReason {
			return searchReasonRank(results[i].MatchReason) < searchReasonRank(results[j].MatchReason)
		}
		if results[i].Title != results[j].Title {
			return results[i].Title < results[j].Title
		}
		return results[i].Href < results[j].Href
	})
	if len(results) > limit {
		results = results[:limit]
	}
	response.Results = results
	response.Total = len(results)
	response.Facets = buildSearchFacets(results)
	return response, nil
}

func listSearchDocuments(db *sql.DB, userID int, libraryID int, kind string, genre string) ([]searchDocument, error) {
	base := `SELECT sd.doc_key, sd.kind, sd.library_id, sd.library_name, sd.library_type, sd.title, sd.normalized_title, sd.subtitle, sd.poster_path, sd.poster_url, sd.imdb_rating, sd.href
FROM search_documents sd
JOIN libraries l ON l.id = sd.library_id`
	args := make([]interface{}, 0, 3)
	clauses := make([]string, 0, 3)
	if userID > 0 {
		clauses = append(clauses, "l.user_id = ?")
		args = append(args, userID)
	}
	if libraryID > 0 {
		clauses = append(clauses, "sd.library_id = ?")
		args = append(args, libraryID)
	}
	if kind != "" {
		clauses = append(clauses, "sd.kind = ?")
		args = append(args, kind)
	}
	if len(clauses) > 0 {
		base += " WHERE " + strings.Join(clauses, " AND ")
	}
	rows, err := db.Query(base, args...)
	if err != nil {
		return nil, err
	}
	baseDocs := make([]searchDocument, 0)
	for rows.Next() {
		var doc searchDocument
		if err := rows.Scan(&doc.DocKey, &doc.Kind, &doc.LibraryID, &doc.LibraryName, &doc.LibraryType, &doc.Title, &doc.Normalized, &doc.Subtitle, &doc.PosterPath, &doc.PosterURL, &doc.IMDbRating, &doc.Href); err != nil {
			rows.Close()
			return nil, err
		}
		baseDocs = append(baseDocs, doc)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var out []searchDocument
	for _, doc := range baseDocs {
		genres, err := loadSearchDocumentGenres(db, doc.DocKey)
		if err != nil {
			return nil, err
		}
		if genre != "" && !containsGenre(genres, genre) {
			continue
		}
		cast, err := loadSearchDocumentCast(db, doc.DocKey)
		if err != nil {
			return nil, err
		}
		doc.Genres = genres
		doc.Cast = cast
		out = append(out, doc)
	}
	return out, nil
}

func loadSearchDocumentGenres(db *sql.DB, docKey string) ([]string, error) {
	rows, err := db.Query(`SELECT genre_name FROM search_document_genres WHERE doc_key = ? ORDER BY genre_name`, docKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

func loadSearchDocumentCast(db *sql.DB, docKey string) ([]TitleCastMember, error) {
	rows, err := db.Query(`SELECT person_name, COALESCE(character_name, ''), COALESCE(billing_order, 0) FROM search_document_cast WHERE doc_key = ? ORDER BY billing_order ASC, person_name ASC`, docKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TitleCastMember
	for rows.Next() {
		var castMember TitleCastMember
		if err := rows.Scan(&castMember.Name, &castMember.Character, &castMember.Order); err != nil {
			return nil, err
		}
		out = append(out, castMember)
	}
	return out, rows.Err()
}

func containsGenre(genres []string, expected string) bool {
	expected = genreSlug(expected)
	for _, genre := range genres {
		if genreSlug(genre) == expected {
			return true
		}
	}
	return false
}

func toSearchResult(doc searchDocument, matchReason string, matchedActor string) SearchResult {
	return SearchResult{
		Kind:         doc.Kind,
		LibraryID:    doc.LibraryID,
		LibraryName:  doc.LibraryName,
		LibraryType:  doc.LibraryType,
		Title:        doc.Title,
		Subtitle:     doc.Subtitle,
		PosterPath:   doc.PosterPath,
		PosterURL:    doc.PosterURL,
		IMDbRating:   doc.IMDbRating,
		MatchReason:  matchReason,
		MatchedActor: matchedActor,
		Href:         doc.Href,
		Genres:       append([]string(nil), doc.Genres...),
	}
}

func fuzzySearchDocuments(docs []searchDocument, query string, limit int) []SearchResult {
	normalized := normalizeSearchText(query)
	if normalized == "" {
		return nil
	}
	type scored struct {
		doc   searchDocument
		score float64
	}
	candidates := make([]scored, 0, len(docs))
	for _, doc := range docs {
		if doc.Normalized == "" {
			continue
		}
		distance := levenshteinDistance(normalized, doc.Normalized)
		maxLen := maxInt(len(normalized), len(doc.Normalized))
		if maxLen == 0 {
			continue
		}
		score := 1 - (float64(distance) / float64(maxLen))
		if score < 0.55 {
			continue
		}
		candidates = append(candidates, scored{doc: doc, score: score})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if math.Abs(candidates[i].score-candidates[j].score) > 0.0001 {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].doc.Title < candidates[j].doc.Title
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	out := make([]SearchResult, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, toSearchResult(candidate.doc, "title", ""))
	}
	return out
}

func buildSearchFacets(results []SearchResult) SearchFacets {
	libraryCounts := map[string]SearchFacetValue{}
	typeCounts := map[string]SearchFacetValue{}
	genreCounts := map[string]SearchFacetValue{}
	for _, result := range results {
		libraryKey := strconv.Itoa(result.LibraryID)
		libraryCounts[libraryKey] = SearchFacetValue{
			Value: libraryKey,
			Label: result.LibraryName,
			Count: libraryCounts[libraryKey].Count + 1,
		}
		typeCounts[result.Kind] = SearchFacetValue{
			Value: result.Kind,
			Label: strings.Title(result.Kind),
			Count: typeCounts[result.Kind].Count + 1,
		}
		for _, genre := range result.Genres {
			slug := genreSlug(genre)
			genreCounts[slug] = SearchFacetValue{
				Value: slug,
				Label: genre,
				Count: genreCounts[slug].Count + 1,
			}
		}
	}
	return SearchFacets{
		Libraries: sortFacetValues(libraryCounts),
		Types:     sortFacetValues(typeCounts),
		Genres:    sortFacetValues(genreCounts),
	}
}

func sortFacetValues(values map[string]SearchFacetValue) []SearchFacetValue {
	out := make([]SearchFacetValue, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Label < out[j].Label
	})
	return out
}

func levenshteinDistance(a string, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return len(b)
	}
	if b == "" {
		return len(a)
	}
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		current := make([]int, len(b)+1)
		current[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			current[j] = minInt(
				current[j-1]+1,
				prev[j]+1,
				prev[j-1]+cost,
			)
		}
		prev = current
	}
	return prev[len(b)]
}

func minInt(values ...int) int {
	best := values[0]
	for _, value := range values[1:] {
		if value < best {
			best = value
		}
	}
	return best
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func refIDForGlobalMediaID(db *sql.DB, mediaID int) (int, error) {
	var refID int
	err := db.QueryRow(`SELECT ref_id FROM media_global WHERE id = ?`, mediaID).Scan(&refID)
	if err != nil {
		return 0, err
	}
	return refID, nil
}

func hasSearchResultForHref(results map[string]SearchResult, href string) bool {
	for _, result := range results {
		if result.Href == href {
			return true
		}
	}
	return false
}

func searchReasonRank(reason string) int {
	switch reason {
	case "title":
		return 0
	case "actor":
		return 1
	default:
		return 2
	}
}
