package httpapi

import (
	"context"
	"database/sql"
	"log"
	"sync"
	"time"

	"plum/internal/db"
	"plum/internal/metadata"
)

type SearchIndexManager struct {
	db     *sql.DB
	movies metadata.MovieDetailsProvider
	series metadata.SeriesDetailsProvider

	onQueue func(libraryID int, full bool)
	refresh func(libraryID int, full bool) error

	sem      chan struct{}
	mu       sync.Mutex
	queued   map[int]bool
	running  map[int]bool
	needFull map[int]bool
}

func NewSearchIndexManager(sqlDB *sql.DB, movies metadata.MovieDetailsProvider, series metadata.SeriesDetailsProvider) *SearchIndexManager {
	return &SearchIndexManager{
		db:       sqlDB,
		movies:   movies,
		series:   series,
		sem:      make(chan struct{}, 1),
		queued:   make(map[int]bool),
		running:  make(map[int]bool),
		needFull: make(map[int]bool),
	}
}

func (m *SearchIndexManager) Queue(libraryID int, full bool) {
	if m == nil || libraryID <= 0 {
		return
	}
	if m.onQueue != nil {
		m.onQueue(libraryID, full)
	}
	m.mu.Lock()
	if full {
		m.needFull[libraryID] = true
	}
	if m.running[libraryID] || m.queued[libraryID] {
		m.queued[libraryID] = true
		m.mu.Unlock()
		return
	}
	m.queued[libraryID] = true
	m.mu.Unlock()
	go m.runLibrary(libraryID)
}

func (m *SearchIndexManager) QueueAllLibraries(full bool) {
	if m == nil {
		return
	}
	rows, err := m.db.Query(`SELECT id FROM libraries WHERE type IN (?, ?, ?)`, db.LibraryTypeMovie, db.LibraryTypeTV, db.LibraryTypeAnime)
	if err != nil {
		log.Printf("search index queue all: %v", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var libraryID int
		if err := rows.Scan(&libraryID); err != nil {
			log.Printf("search index queue all scan: %v", err)
			return
		}
		m.Queue(libraryID, full)
	}
}

func (m *SearchIndexManager) runLibrary(libraryID int) {
	for {
		m.mu.Lock()
		m.queued[libraryID] = false
		m.running[libraryID] = true
		full := m.needFull[libraryID]
		delete(m.needFull, libraryID)
		m.mu.Unlock()

		refresh := m.refresh
		if refresh == nil {
			refresh = m.refreshLibrary
		}
		m.sem <- struct{}{}
		err := refresh(libraryID, full)
		<-m.sem
		if err != nil {
			log.Printf("search index refresh library %d: %v", libraryID, err)
			time.Sleep(5 * time.Second)
		}

		m.mu.Lock()
		m.running[libraryID] = false
		if !m.queued[libraryID] && !m.needFull[libraryID] {
			delete(m.queued, libraryID)
			delete(m.running, libraryID)
			m.mu.Unlock()
			return
		}
		m.queued[libraryID] = false
		m.mu.Unlock()
	}
}

func (m *SearchIndexManager) refreshLibrary(libraryID int, full bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if full {
		if err := m.backfillMovieMetadata(ctx, libraryID); err != nil {
			return err
		}
		if err := m.backfillShowMetadata(ctx, libraryID); err != nil {
			return err
		}
	}
	return db.RefreshLibrarySearchIndex(ctx, m.db, libraryID)
}

func (m *SearchIndexManager) backfillMovieMetadata(ctx context.Context, libraryID int) error {
	if m.movies == nil {
		return nil
	}
	rows, err := m.db.QueryContext(ctx, `SELECT id, COALESCE(tmdb_id, 0) FROM movies WHERE library_id = ? AND COALESCE(tmdb_id, 0) > 0`, libraryID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var refID int
		var tmdbID int
		if err := rows.Scan(&refID, &tmdbID); err != nil {
			return err
		}
		details, err := m.movies.GetMovieDetails(ctx, tmdbID)
		if err != nil || details == nil {
			continue
		}
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
		if err := db.SyncMovieTitleMetadata(m.db, refID, db.CanonicalMetadata{
			Title:        details.Title,
			Overview:     details.Overview,
			PosterPath:   details.PosterPath,
			BackdropPath: details.BackdropPath,
			ReleaseDate:  details.ReleaseDate,
			IMDbID:       details.IMDbID,
			IMDbRating:   details.IMDbRating,
			Genres:       details.Genres,
			Cast:         cast,
			Runtime:      details.Runtime,
		}); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (m *SearchIndexManager) backfillShowMetadata(ctx context.Context, libraryID int) error {
	if m.series == nil {
		return nil
	}
	rows, err := m.db.QueryContext(ctx, `SELECT id, COALESCE(tmdb_id, 0) FROM shows WHERE library_id = ? AND COALESCE(tmdb_id, 0) > 0`, libraryID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var showID int
		var tmdbID int
		if err := rows.Scan(&showID, &tmdbID); err != nil {
			return err
		}
		details, err := m.series.GetSeriesDetails(ctx, tmdbID)
		if err != nil || details == nil {
			continue
		}
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
		if err := db.SyncShowTitleMetadata(m.db, showID, db.CanonicalMetadata{
			Title:        details.Name,
			Overview:     details.Overview,
			PosterPath:   details.PosterPath,
			BackdropPath: details.BackdropPath,
			ReleaseDate:  details.FirstAirDate,
			IMDbID:       details.IMDbID,
			IMDbRating:   details.IMDbRating,
			Genres:       details.Genres,
			Cast:         cast,
			Runtime:      details.Runtime,
		}); err != nil {
			return err
		}
	}
	return rows.Err()
}
