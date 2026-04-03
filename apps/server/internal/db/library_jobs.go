package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type LibraryJobStatus struct {
	LibraryID         int
	Path              string
	Type              string
	Phase             string
	EnrichmentPhase   string
	Enriching         bool
	IdentifyPhase     string
	Identified        int
	IdentifyFailed    int
	Processed         int
	Added             int
	Updated           int
	Removed           int
	Unmatched         int
	Skipped           int
	IdentifyRequested bool
	QueuedAt          string
	EstimatedItems    int
	Error             string
	RetryCount        int
	MaxRetries        int
	NextRetryAt       string
	LastError         string
	NextScheduledAt   string
	StartedAt         string
	FinishedAt        string
}

func UpsertLibraryJobStatus(dbConn *sql.DB, status LibraryJobStatus) error {
	now := time.Now().UTC().Format(time.RFC3339)
	enrichmentPhase := status.EnrichmentPhase
	if enrichmentPhase == "" {
		if status.Enriching {
			enrichmentPhase = "running"
		} else {
			enrichmentPhase = "idle"
		}
	}
	_, err := dbConn.Exec(
		`INSERT INTO library_job_status (
			library_id, phase, enrichment_phase, enriching, identify_phase, identified, identify_failed,
			processed, added, updated, removed, unmatched, skipped,
			identify_requested, queued_at, estimated_items, error, retry_count, max_retries,
			next_retry_at, last_error, next_scheduled_at, started_at, finished_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(library_id) DO UPDATE SET
			phase = excluded.phase,
			enrichment_phase = excluded.enrichment_phase,
			enriching = excluded.enriching,
			identify_phase = excluded.identify_phase,
			identified = excluded.identified,
			identify_failed = excluded.identify_failed,
			processed = excluded.processed,
			added = excluded.added,
			updated = excluded.updated,
			removed = excluded.removed,
			unmatched = excluded.unmatched,
			skipped = excluded.skipped,
			identify_requested = excluded.identify_requested,
			queued_at = excluded.queued_at,
			estimated_items = excluded.estimated_items,
			error = excluded.error,
			retry_count = excluded.retry_count,
			max_retries = excluded.max_retries,
			next_retry_at = excluded.next_retry_at,
			last_error = excluded.last_error,
			next_scheduled_at = excluded.next_scheduled_at,
			started_at = excluded.started_at,
			finished_at = excluded.finished_at,
			updated_at = excluded.updated_at`,
		status.LibraryID,
		status.Phase,
		enrichmentPhase,
		boolToInt(status.Enriching),
		status.IdentifyPhase,
		status.Identified,
		status.IdentifyFailed,
		status.Processed,
		status.Added,
		status.Updated,
		status.Removed,
		status.Unmatched,
		status.Skipped,
		boolToInt(status.IdentifyRequested),
		nullStr(status.QueuedAt),
		status.EstimatedItems,
		nullStr(status.Error),
		status.RetryCount,
		status.MaxRetries,
		nullStr(status.NextRetryAt),
		nullStr(status.LastError),
		nullStr(status.NextScheduledAt),
		nullStr(status.StartedAt),
		nullStr(status.FinishedAt),
		now,
	)
	return err
}

func ListLibraryJobStatuses(dbConn *sql.DB) ([]LibraryJobStatus, error) {
	rows, err := dbConn.Query(
		`SELECT
			s.library_id,
			l.path,
			l.type,
			s.phase,
			COALESCE(s.enrichment_phase, CASE WHEN COALESCE(s.enriching, 0) != 0 THEN 'running' ELSE 'idle' END),
			COALESCE(s.enriching, 0),
			COALESCE(s.identify_phase, 'idle'),
			COALESCE(s.identified, 0),
			COALESCE(s.identify_failed, 0),
			COALESCE(s.processed, 0),
			COALESCE(s.added, 0),
			COALESCE(s.updated, 0),
			COALESCE(s.removed, 0),
			COALESCE(s.unmatched, 0),
			COALESCE(s.skipped, 0),
			COALESCE(s.identify_requested, 0),
			COALESCE(s.queued_at, ''),
			COALESCE(s.estimated_items, 0),
			COALESCE(s.error, ''),
			COALESCE(s.retry_count, 0),
			COALESCE(s.max_retries, 3),
			COALESCE(s.next_retry_at, ''),
			COALESCE(s.last_error, ''),
			COALESCE(s.next_scheduled_at, ''),
			COALESCE(s.started_at, ''),
			COALESCE(s.finished_at, '')
		FROM library_job_status s
		JOIN libraries l ON l.id = s.library_id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []LibraryJobStatus
	for rows.Next() {
		var status LibraryJobStatus
		var enrichmentPhase string
		var enriching int
		var identifyRequested int
		if err := rows.Scan(
			&status.LibraryID,
			&status.Path,
			&status.Type,
			&status.Phase,
			&enrichmentPhase,
			&enriching,
			&status.IdentifyPhase,
			&status.Identified,
			&status.IdentifyFailed,
			&status.Processed,
			&status.Added,
			&status.Updated,
			&status.Removed,
			&status.Unmatched,
			&status.Skipped,
			&identifyRequested,
			&status.QueuedAt,
			&status.EstimatedItems,
			&status.Error,
			&status.RetryCount,
			&status.MaxRetries,
			&status.NextRetryAt,
			&status.LastError,
			&status.NextScheduledAt,
			&status.StartedAt,
			&status.FinishedAt,
		); err != nil {
			return nil, err
		}
		status.EnrichmentPhase = enrichmentPhase
		status.Enriching = enriching != 0
		status.IdentifyRequested = identifyRequested != 0
		out = append(out, status)
	}
	return out, rows.Err()
}

func ListLibraryEnrichmentTasks(
	ctx context.Context,
	dbConn *sql.DB,
	libraryID int,
	libraryType string,
	identifyRequested bool,
) ([]EnrichmentTask, error) {
	var table string
	switch libraryType {
	case LibraryTypeMovie:
		table = "movies"
	case LibraryTypeTV:
		table = "tv_episodes"
	case LibraryTypeAnime:
		table = "anime_episodes"
	case LibraryTypeMusic:
		table = "music_tracks"
	default:
		return nil, fmt.Errorf("unsupported library type %q", libraryType)
	}

	// Recovery must not replay enrichment for every row in the library. Only rows that still
	// need analyzer work (deferred hash, changed file metadata) or, for music with identify,
	// tracks not yet matched to a provider.
	needsWork := `(COALESCE(t.file_hash, '') = '' OR COALESCE(t.file_hash_kind, '') = '')`
	if libraryType == LibraryTypeMusic && identifyRequested {
		needsWork = `(` + needsWork + ` OR COALESCE(t.match_status, 'local') != 'identified')`
	}

	rows, err := dbConn.QueryContext(
		ctx,
		fmt.Sprintf(
			`SELECT t.id, COALESCE(g.id, 0), t.path
			   FROM %s t
			   LEFT JOIN media_global g ON g.kind = ? AND g.ref_id = t.id
			  WHERE t.library_id = ?
			    AND COALESCE(t.missing_since, '') = ''
			    AND %s
			  ORDER BY t.id`,
			table,
			needsWork,
		),
		libraryType,
		libraryID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]EnrichmentTask, 0)
	for rows.Next() {
		var task EnrichmentTask
		task.LibraryID = libraryID
		task.Kind = libraryType
		if err := rows.Scan(&task.RefID, &task.GlobalID, &task.Path); err != nil {
			return nil, err
		}
		if task.Path == "" {
			continue
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tasks, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
