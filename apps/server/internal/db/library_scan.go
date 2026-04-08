package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"plum/internal/metadata"
)

// HandleScanLibrary walks the given filesystem path and inserts supported media files
// into the category table for this library type only (movies, tv_episodes, anime_episodes, or music_tracks).
// libraryID must be > 0. mediaType must be tv, movie, music, or anime.
// id may be nil; then no metadata lookup is performed.
func HandleScanLibrary(ctx context.Context, dbConn *sql.DB, root, mediaType string, libraryID int, id metadata.Identifier) (ScanResult, error) {
	var musicIdentifier metadata.MusicIdentifier
	if detected, ok := id.(metadata.MusicIdentifier); ok {
		musicIdentifier = detected
	}
	return HandleScanLibraryWithOptions(ctx, dbConn, root, mediaType, libraryID, ScanOptions{
		Identifier:             id,
		MusicIdentifier:        musicIdentifier,
		ProbeMedia:             true,
		ProbeEmbeddedSubtitles: true,
		ScanSidecarSubtitles:   true,
	})
}

type scanCandidate struct {
	Path    string
	RelPath string
	Name    string
	Size    int64
	ModTime string
}

func EstimateLibraryFiles(ctx context.Context, root, mediaType string) (int, error) {
	count := 0
	err := iterateLibraryFiles(ctx, root, mediaType, nil, nil, func(scanCandidate) error {
		count++
		return nil
	})
	return count, err
}

func iterateLibraryFiles(
	ctx context.Context,
	root, mediaType string,
	onDirectory func(string),
	onSkip func(),
	visit func(scanCandidate) error,
) error {
	if root == "" {
		return fmt.Errorf("path is required")
	}
	if mediaType == "" {
		mediaType = LibraryTypeMovie
	}

	info, err := os.Stat(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf(
				"path not found: %q — use a path visible to the backend process. In Docker that usually means the container mount path (for example /tv, /movies, /anime, /music); in local dev it may be the host path from PLUM_MEDIA_*_PATH in .env",
				root,
			)
		}
		return fmt.Errorf("stat path: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory")
	}

	exts := allowedExtensions(mediaType)
	return filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if onDirectory != nil && path != root {
				onDirectory(path)
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		ext := strings.ToLower(filepath.Ext(d.Name()))
		if _, ok := exts[ext]; !ok {
			return nil
		}
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if shouldSkipScanPath(mediaType, relPath, d.Name()) {
			if onSkip != nil {
				onSkip()
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return visit(scanCandidate{
			Path:    path,
			RelPath: relPath,
			Name:    d.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime().UTC().Format(time.RFC3339Nano),
		})
	})
}

const (
	scanInsertChunkSize   = 25
	EnrichmentWorkerCount = 2
	enrichmentWorkerCount = EnrichmentWorkerCount
)

type pendingDiscoveredInsert struct {
	Item MediaItem
}

func buildScannedMediaItem(root, kind string, candidate scanCandidate) (MediaItem, metadata.MediaInfo, metadata.MusicInfo, error) {
	title := strings.TrimSuffix(candidate.Name, filepath.Ext(candidate.Name))
	if title == "" {
		title = candidate.Name
	}

	item := MediaItem{
		Title:         title,
		Path:          candidate.Path,
		Type:          kind,
		MatchStatus:   MatchStatusLocal,
		FileSizeBytes: candidate.Size,
		FileModTime:   candidate.ModTime,
	}

	var fileInfo metadata.MediaInfo
	var musicInfo metadata.MusicInfo
	switch kind {
	case LibraryTypeMusic:
		pathInfo := metadata.ParsePathForMusic(candidate.RelPath, candidate.Name)
		merged := metadata.MergeMusicMetadata(pathInfo, metadata.MusicMetadata{}, title)
		item.Title = merged.Title
		item.Artist = merged.Artist
		item.Album = merged.Album
		item.AlbumArtist = merged.AlbumArtist
		item.DiscNumber = merged.DiscNumber
		item.TrackNumber = merged.TrackNumber
		item.ReleaseYear = merged.ReleaseYear
		musicInfo = metadata.MusicInfo{
			Title:       merged.Title,
			Artist:      merged.Artist,
			Album:       merged.Album,
			AlbumArtist: merged.AlbumArtist,
			DiscNumber:  merged.DiscNumber,
			TrackNumber: merged.TrackNumber,
			ReleaseYear: merged.ReleaseYear,
		}
	case LibraryTypeMovie:
		movieInfo := metadata.ParseMovie(candidate.RelPath, candidate.Name)
		item.Title = metadata.MovieDisplayTitle(movieInfo, title)
		fileInfo = metadata.MovieMediaInfo(movieInfo)
	case LibraryTypeTV, LibraryTypeAnime:
		fileInfo = metadata.ParseFilename(candidate.Name)
		pathInfo := metadata.ParsePathForTV(candidate.RelPath, candidate.Name)
		merged := metadata.MergePathInfo(pathInfo, fileInfo)
		showRoot := metadata.ShowRootPath(root, candidate.Path)
		metadata.ApplyShowNFO(&merged, showRoot)
		if kind == LibraryTypeAnime && merged.IsSpecial && merged.Episode > 0 {
			merged.Season = 0
		}
		item.Season = merged.Season
		item.Episode = merged.Episode
		item.Title = buildEpisodeDisplayTitle(pathInfo.ShowName, merged, title, fileInfo.Title)
		fileInfo = merged
	default:
		return MediaItem{}, metadata.MediaInfo{}, metadata.MusicInfo{}, fmt.Errorf("unsupported media type %q", kind)
	}

	return item, fileInfo, musicInfo, nil
}

func flushPendingDiscoveredInserts(
	ctx context.Context,
	dbConn *sql.DB,
	table, kind string,
	libraryID int,
	pending []pendingDiscoveredInsert,
	seenAt string,
) ([]EnrichmentTask, error) {
	if len(pending) == 0 {
		return nil, nil
	}

	tx, err := dbConn.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	rollback := true
	defer func() {
		if rollback {
			_ = tx.Rollback()
		}
	}()

	insertSQL := `INSERT INTO ` + table + ` (library_id, title, path, duration, file_size_bytes, file_mod_time, file_hash, file_hash_kind, last_seen_at, missing_since, match_status, tmdb_id, tvdb_id, overview, poster_path, backdrop_path, release_date, vote_average, imdb_id, imdb_rating) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`
	switch table {
	case "music_tracks":
		insertSQL = `INSERT INTO music_tracks (library_id, title, path, duration, file_size_bytes, file_mod_time, file_hash, file_hash_kind, last_seen_at, missing_since, match_status, artist, album, album_artist, poster_path, musicbrainz_artist_id, musicbrainz_release_group_id, musicbrainz_release_id, musicbrainz_recording_id, disc_number, track_number, release_year) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`
	case "tv_episodes", "anime_episodes":
		insertSQL = `INSERT INTO ` + table + ` (library_id, title, path, duration, file_size_bytes, file_mod_time, file_hash, file_hash_kind, last_seen_at, missing_since, match_status, tmdb_id, tvdb_id, overview, poster_path, backdrop_path, release_date, vote_average, imdb_id, imdb_rating, season, episode, metadata_review_needed, metadata_confirmed) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`
	}
	insertStmt, err := tx.PrepareContext(ctx, insertSQL)
	if err != nil {
		return nil, err
	}
	defer insertStmt.Close()

	globalStmt, err := tx.PrepareContext(ctx, `INSERT INTO media_global (kind, ref_id) VALUES (?, ?) RETURNING id`)
	if err != nil {
		return nil, err
	}
	defer globalStmt.Close()

	mediaFileStmt, err := tx.PrepareContext(ctx, `INSERT INTO media_files (
media_id, path, file_size_bytes, file_mod_time, file_hash, file_hash_kind, duration, missing_since, last_seen_at, is_primary, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(path) DO UPDATE SET
media_id = excluded.media_id,
file_size_bytes = excluded.file_size_bytes,
file_mod_time = excluded.file_mod_time,
file_hash = excluded.file_hash,
file_hash_kind = excluded.file_hash_kind,
duration = excluded.duration,
missing_since = excluded.missing_since,
last_seen_at = excluded.last_seen_at,
is_primary = excluded.is_primary,
updated_at = excluded.updated_at`)
	if err != nil {
		return nil, err
	}
	defer mediaFileStmt.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	tasks := make([]EnrichmentTask, 0, len(pending))
	for _, pendingInsert := range pending {
		item := pendingInsert.Item
		var refID int
		switch table {
		case "music_tracks":
			err = insertStmt.QueryRowContext(ctx,
				libraryID, item.Title, item.Path, item.Duration, item.FileSizeBytes, nullStr(item.FileModTime), nullStr(item.FileHash), nullStr(item.FileHashKind), nullStr(seenAt), item.MatchStatus, nullStr(item.Artist), nullStr(item.Album), nullStr(item.AlbumArtist), nullStr(item.PosterPath), nullStr(item.MusicBrainzArtistID), nullStr(item.MusicBrainzReleaseGroupID), nullStr(item.MusicBrainzReleaseID), nullStr(item.MusicBrainzRecordingID), item.DiscNumber, item.TrackNumber, item.ReleaseYear,
			).Scan(&refID)
		case "tv_episodes", "anime_episodes":
			err = insertStmt.QueryRowContext(ctx,
				libraryID, item.Title, item.Path, item.Duration, item.FileSizeBytes, nullStr(item.FileModTime), nullStr(item.FileHash), nullStr(item.FileHashKind), nullStr(seenAt), item.MatchStatus, item.TMDBID, nullStr(item.TVDBID), nullStr(item.Overview), nullStr(item.PosterPath), nullStr(item.BackdropPath), nullStr(item.ReleaseDate), nullFloat64(item.VoteAverage), nullStr(item.IMDbID), nullFloat64(item.IMDbRating), item.Season, item.Episode, item.MetadataReviewNeeded, item.MetadataConfirmed,
			).Scan(&refID)
		default:
			err = insertStmt.QueryRowContext(ctx,
				libraryID, item.Title, item.Path, item.Duration, item.FileSizeBytes, nullStr(item.FileModTime), nullStr(item.FileHash), nullStr(item.FileHashKind), nullStr(seenAt), item.MatchStatus, item.TMDBID, nullStr(item.TVDBID), nullStr(item.Overview), nullStr(item.PosterPath), nullStr(item.BackdropPath), nullStr(item.ReleaseDate), nullFloat64(item.VoteAverage), nullStr(item.IMDbID), nullFloat64(item.IMDbRating),
			).Scan(&refID)
		}
		if err != nil {
			return nil, err
		}

		var globalID int
		if err := globalStmt.QueryRowContext(ctx, kind, refID).Scan(&globalID); err != nil {
			return nil, err
		}
		if _, err := mediaFileStmt.ExecContext(ctx,
			globalID,
			item.Path,
			item.FileSizeBytes,
			nullStr(item.FileModTime),
			nullStr(item.FileHash),
			nullStr(item.FileHashKind),
			item.Duration,
			nil,
			nullStr(seenAt),
			1,
			now,
			now,
		); err != nil {
			return nil, err
		}
		tasks = append(tasks, EnrichmentTask{
			LibraryID: libraryID,
			Kind:      kind,
			RefID:     refID,
			GlobalID:  globalID,
			Path:      item.Path,
		})
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	rollback = false
	return tasks, nil
}

// findRelocatedTVEpisodeRow returns an existing row for the same library, show folder, season, and
// episode whose file path is gone (rename/move) so discovery can update the path instead of inserting
// a duplicate. Rows for other shows under the same library are ignored so S01E01 in Show A cannot
// steal a relocation match from Show B.
func findRelocatedTVEpisodeRow(ctx context.Context, dbConn *sql.DB, table, kind string, libraryID, season, episode int, libraryRoot, newPath string) (existingMediaRow, bool, error) {
	var zero existingMediaRow
	if table != "tv_episodes" && table != "anime_episodes" {
		return zero, false, nil
	}
	if episode <= 0 {
		return zero, false, nil
	}
	newShowRoot := filepath.Clean(metadata.ShowRootPath(libraryRoot, newPath))
	if newShowRoot == "" || newShowRoot == "." {
		return zero, false, nil
	}
	query := `SELECT m.path, m.id, COALESCE(g.id, 0), COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.duration, 0), COALESCE(m.last_seen_at, ''), COALESCE(m.missing_since, ''), COALESCE(m.tmdb_id, 0), m.tvdb_id, m.imdb_id, COALESCE(m.match_status, 'local'), COALESCE(m.metadata_review_needed, 0), COALESCE(m.metadata_confirmed, 0),
CASE WHEN (mf.intro_probed_at IS NOT NULL AND TRIM(mf.intro_probed_at) != '') OR COALESCE(mf.intro_locked, 0) != 0 THEN 1 ELSE 0 END
FROM ` + table + ` m
LEFT JOIN media_global g ON g.kind = ? AND g.ref_id = m.id
LEFT JOIN media_files mf ON mf.media_id = g.id AND mf.is_primary = 1
WHERE m.library_id = ? AND COALESCE(m.season, 0) = ? AND COALESCE(m.episode, 0) = ?`
	rows, err := dbConn.QueryContext(ctx, query, kind, libraryID, season, episode)
	if err != nil {
		return zero, false, err
	}
	defer rows.Close()

	var absent []existingMediaRow
	for rows.Next() {
		var row existingMediaRow
		var tvdbID, imdbID sql.NullString
		var introProbeDone int
		if err := rows.Scan(&row.Path, &row.RefID, &row.GlobalID, &row.FileSizeBytes, &row.FileModTime, &row.FileHash, &row.FileHashKind, &row.Duration, &row.LastSeenAt, &row.MissingSince, &row.TMDBID, &tvdbID, &imdbID, &row.MatchStatus, &row.MetadataReviewNeeded, &row.MetadataConfirmed, &introProbeDone); err != nil {
			return zero, false, err
		}
		row.PrimaryIntroProbed = introProbeDone != 0
		if tvdbID.Valid {
			row.TVDBID = tvdbID.String
		}
		if imdbID.Valid {
			row.IMDbID = imdbID.String
		}
		rowShowRoot := filepath.Clean(metadata.ShowRootPath(libraryRoot, row.Path))
		if rowShowRoot != newShowRoot {
			continue
		}
		if row.Path == newPath {
			continue
		}
		if row.MissingSince != "" {
			absent = append(absent, row)
			continue
		}
		if _, statErr := os.Stat(row.Path); errors.Is(statErr, os.ErrNotExist) {
			absent = append(absent, row)
		}
	}
	if err := rows.Err(); err != nil {
		return zero, false, err
	}
	if len(absent) != 1 {
		return zero, false, nil
	}
	return absent[0], true, nil
}

func appendPlaceholders(dst []string, count int) []string {
	for i := 0; i < count; i++ {
		dst = append(dst, "?")
	}
	return dst
}

func batchUpdateMissingMedia(ctx context.Context, dbConn *sql.DB, table, kind string, staleIDs []int, missingSince string) error {
	if len(staleIDs) == 0 {
		return nil
	}
	for start := 0; start < len(staleIDs); start += scanInsertChunkSize {
		end := start + scanInsertChunkSize
		if end > len(staleIDs) {
			end = len(staleIDs)
		}
		chunk := staleIDs[start:end]
		placeholders := appendPlaceholders(nil, len(chunk))
		args := make([]any, 0, len(chunk)+1)
		args = append(args, missingSince)
		for _, id := range chunk {
			args = append(args, id)
		}
		if _, err := dbConn.ExecContext(ctx,
			`UPDATE `+table+` SET missing_since = ?, last_seen_at = COALESCE(last_seen_at, '') WHERE id IN (`+strings.Join(placeholders, ",")+`)`,
			args...,
		); err != nil {
			return err
		}

		now := time.Now().UTC().Format(time.RFC3339)
		mediaArgs := make([]any, 0, len(chunk)+2)
		mediaArgs = append(mediaArgs, missingSince, now, kind)
		for _, id := range chunk {
			mediaArgs = append(mediaArgs, id)
		}
		if _, err := dbConn.ExecContext(ctx,
			`UPDATE media_files
			    SET missing_since = ?, updated_at = ?
			  WHERE media_id IN (
				SELECT id FROM media_global WHERE kind = ? AND ref_id IN (`+strings.Join(placeholders, ",")+`)
			  )`,
			mediaArgs...,
		); err != nil && !strings.Contains(strings.ToLower(err.Error()), "no such table: media_files") {
			return err
		}
	}
	return nil
}

func ScanLibraryDiscovery(
	ctx context.Context,
	dbConn *sql.DB,
	root, mediaType string,
	libraryID int,
	options ScanOptions,
) (ScanDelta, error) {
	delta := ScanDelta{}
	if mediaType == "" {
		mediaType = LibraryTypeMovie
	}
	if libraryID <= 0 {
		return delta, fmt.Errorf("library id is required")
	}

	kind := mediaType
	table := mediaTableForKind(kind)
	scanSubpaths, err := NormalizeScanSubpaths(options.Subpaths)
	if err != nil {
		return delta, err
	}
	scanRoots, markRoots, err := resolveScanRoots(root, scanSubpaths)
	if err != nil {
		return delta, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	existingByPath, err := preloadExistingMediaByPath(dbConn, table, kind, libraryID)
	if err != nil {
		return delta, err
	}

	seenPaths := map[string]struct{}{}
	pending := make([]pendingDiscoveredInsert, 0, scanInsertChunkSize)
	emitProgress := func() {
		if options.Progress != nil {
			options.Progress(ScanProgress{
				Processed: delta.Result.Added + delta.Result.Updated + delta.Result.Skipped,
				Result:    delta.Result,
			})
		}
	}
	flushPending := func() error {
		if len(pending) == 0 {
			return nil
		}
		tasks, err := flushPendingDiscoveredInserts(ctx, dbConn, table, kind, libraryID, pending, now)
		if err != nil {
			return err
		}
		delta.TouchedFiles = append(delta.TouchedFiles, tasks...)
		for range pending {
			delta.Result.Added++
			emitProgress()
		}
		pending = pending[:0]
		return nil
	}

	for _, scanRoot := range scanRoots {
		err = iterateLibraryFiles(ctx, scanRoot, kind, func(path string) {
			if options.Activity != nil {
				options.Activity(ScanActivity{
					Phase:  "discovery",
					Target: "directory",
					Path:   path,
				})
			}
		}, func() {
			delta.Result.Skipped++
			emitProgress()
		}, func(candidate scanCandidate) error {
			if options.Activity != nil {
				options.Activity(ScanActivity{
					Phase:  "discovery",
					Target: "file",
					Path:   candidate.Path,
				})
			}
			if _, ok := seenPaths[candidate.Path]; ok {
				return nil
			}
			seenPaths[candidate.Path] = struct{}{}

			relPath, err := filepath.Rel(root, candidate.Path)
			if err != nil {
				return err
			}
			candidate.RelPath = relPath

			existing := existingByPath[candidate.Path]
			isNew := existing.RefID == 0

			item, _, _, err := buildScannedMediaItem(root, kind, candidate)
			if err != nil {
				return err
			}
			if isNew && (kind == LibraryTypeTV || kind == LibraryTypeAnime) {
				relocated, ok, err := findRelocatedTVEpisodeRow(ctx, dbConn, table, kind, libraryID, item.Season, item.Episode, root, candidate.Path)
				if err != nil {
					return err
				}
				if ok {
					delete(existingByPath, relocated.Path)
					existing = relocated
					isNew = false
				}
			}
			if !isNew {
				applyExistingMetadata(&item, existing, kind)
				if kind == LibraryTypeTV || kind == LibraryTypeAnime {
					item.MetadataReviewNeeded = existing.MetadataReviewNeeded
					item.MetadataConfirmed = existing.MetadataConfirmed
				}
			}

			hasStableFileState := !isNew &&
				existing.MissingSince == "" &&
				existing.FileSizeBytes == candidate.Size &&
				existing.FileModTime == candidate.ModTime
			isUnchanged := !isNew &&
				existing.Path == candidate.Path &&
				hasStableFileState &&
				existing.FileHash != "" &&
				existing.FileHashKind != ""

			if isUnchanged {
				if err := markMediaPresent(ctx, dbConn, table, existing.RefID, candidate.Size, candidate.ModTime, existing.FileHash, existing.FileHashKind, now); err != nil {
					return err
				}
				if existing.GlobalID > 0 {
					if err := upsertMediaFileForMediaID(ctx, dbConn, existing.GlobalID, MediaItem{
						Path:          candidate.Path,
						Duration:      existing.Duration,
						FileSizeBytes: candidate.Size,
						FileModTime:   candidate.ModTime,
						FileHash:      existing.FileHash,
						FileHashKind:  existing.FileHashKind,
					}, true); err != nil {
						return err
					}
					if kind != LibraryTypeMusic && !existing.PrimaryIntroProbed {
						delta.TouchedFiles = append(delta.TouchedFiles, EnrichmentTask{
							LibraryID: libraryID,
							Kind:      kind,
							RefID:     existing.RefID,
							GlobalID:  existing.GlobalID,
							Path:      candidate.Path,
						})
					}
				}
				delta.Result.Updated++
				emitProgress()
				return nil
			}

			item.FileHash = ""
			item.FileHashKind = ""
			if isNew {
				pending = append(pending, pendingDiscoveredInsert{Item: item})
				if len(pending) >= scanInsertChunkSize {
					return flushPending()
				}
				return nil
			}

			if err := updateScannedItem(ctx, dbConn, table, existing.RefID, item, now); err != nil {
				return err
			}
			if existing.GlobalID > 0 {
				if err := upsertMediaFileForMediaID(ctx, dbConn, existing.GlobalID, item, true); err != nil {
					return err
				}
			}
			delta.TouchedFiles = append(delta.TouchedFiles, EnrichmentTask{
				LibraryID: libraryID,
				Kind:      kind,
				RefID:     existing.RefID,
				GlobalID:  existing.GlobalID,
				Path:      item.Path,
			})
			delta.Result.Updated++
			emitProgress()
			return nil
		})
		if err != nil {
			return delta, err
		}
	}
	if err := flushPending(); err != nil {
		return delta, err
	}
	if err := markMissingMedia(ctx, dbConn, table, kind, libraryID, markRoots, seenPaths, now); err != nil {
		return delta, err
	}
	emitProgress()
	return delta, nil
}

func enrichTask(
	ctx context.Context,
	dbConn *sql.DB,
	root, mediaType string,
	libraryID int,
	task EnrichmentTask,
	options ScanOptions,
) error {
	table := mediaTableForKind(mediaType)
	existing, err := lookupExistingMedia(dbConn, table, mediaType, libraryID, task.Path)
	if err != nil {
		return err
	}
	if existing.RefID == 0 {
		return nil
	}
	info, err := os.Stat(task.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	candidate := scanCandidate{
		Path:    task.Path,
		RelPath: "",
		Name:    filepath.Base(task.Path),
		Size:    info.Size(),
		ModTime: info.ModTime().UTC().Format(time.RFC3339Nano),
	}
	if relPath, err := filepath.Rel(root, task.Path); err == nil {
		candidate.RelPath = relPath
	}

	needsRehash := existing.FileHash == "" ||
		existing.FileHashKind == "" ||
		existing.FileSizeBytes != info.Size() ||
		existing.FileModTime != candidate.ModTime
	// Skip redundant ffprobe/hash work when this task was already fully enriched (e.g. recovery
	// lists were overly broad before we filtered ListLibraryEnrichmentTasks).
	if mediaType == LibraryTypeMusic &&
		!needsRehash &&
		options.MusicIdentifier == nil &&
		existing.Duration > 0 {
		return nil
	}

	item, _, musicInfo, err := buildScannedMediaItem(root, mediaType, candidate)
	if err != nil {
		return err
	}
	applyExistingMetadata(&item, existing, mediaType)
	if mediaType == LibraryTypeTV || mediaType == LibraryTypeAnime {
		item.MetadataReviewNeeded = existing.MetadataReviewNeeded
		item.MetadataConfirmed = existing.MetadataConfirmed
	}
	if existing.Duration > 0 {
		item.Duration = existing.Duration
	}
	item.MatchStatus = existing.MatchStatus

	var (
		embeddedSubtitles []EmbeddedSubtitle
		embeddedAudio     []EmbeddedAudioTrack
		probedVideo       *VideoProbeResult
	)
	if mediaType == LibraryTypeMusic {
		if options.ProbeMedia && !SkipFFprobeInScan {
			if probed, duration, err := readAudioMetadata(ctx, task.Path); err == nil {
				merged := metadata.MergeMusicMetadata(metadata.ParsePathForMusic(candidate.RelPath, candidate.Name), probed, item.Title)
				item.Title = merged.Title
				item.Artist = merged.Artist
				item.Album = merged.Album
				item.AlbumArtist = merged.AlbumArtist
				item.DiscNumber = merged.DiscNumber
				item.TrackNumber = merged.TrackNumber
				item.ReleaseYear = merged.ReleaseYear
				item.Duration = duration
				musicInfo = metadata.MusicInfo{
					Title:       merged.Title,
					Artist:      merged.Artist,
					Album:       merged.Album,
					AlbumArtist: merged.AlbumArtist,
					DiscNumber:  merged.DiscNumber,
					TrackNumber: merged.TrackNumber,
					ReleaseYear: merged.ReleaseYear,
				}
			}
		}
		if options.MusicIdentifier != nil {
			if res := options.MusicIdentifier.IdentifyMusic(ctx, musicInfo); res != nil {
				applyMusicMatchResultToMediaItem(&item, res)
				item.MatchStatus = MatchStatusIdentified
			}
		}
	} else if options.ProbeMedia && !SkipFFprobeInScan {
		if probed, err := readVideoMetadata(ctx, task.Path); err == nil {
			probedVideo = &probed
			if probed.Duration > 0 {
				item.Duration = probed.Duration
			}
			if options.ProbeEmbeddedSubtitles {
				embeddedSubtitles = probed.EmbeddedSubtitles
			}
			embeddedAudio = probed.EmbeddedAudioTracks
		}
	}

	hash := existing.FileHash
	hashKind := existing.FileHashKind
	if needsRehash {
		var hashErr error
		hash, hashErr = computeMediaHash(ctx, task.Path)
		if hashErr != nil {
			return hashErr
		}
		hashKind = fileHashKindSampledSHA256
	} else if hashKind == "" && hash != "" {
		hashKind = fileHashKindSHA256
	}
	item.FileHash = hash
	item.FileHashKind = hashKind
	now := time.Now().UTC().Format(time.RFC3339)
	if err := updateScannedItem(ctx, dbConn, table, existing.RefID, item, now); err != nil {
		return err
	}
	globalID := task.GlobalID
	if globalID <= 0 {
		globalID = existing.GlobalID
	}
	if globalID > 0 {
		if err := upsertMediaFileForMediaID(ctx, dbConn, globalID, item, true); err != nil {
			return err
		}
		if probedVideo != nil {
			if err := UpdateMediaFileIntroFromProbe(ctx, dbConn, globalID, task.Path, *probedVideo); err != nil {
				slog.Warn("persist intro chapters", "media_id", globalID, "path", task.Path, "error", err)
			}
		}
	}
	if mediaType == LibraryTypeMusic {
		return nil
	}
	if options.ScanSidecarSubtitles && globalID > 0 {
		if err := scanForSubtitles(ctx, dbConn, globalID, task.Path); err != nil {
			slog.Warn("scan subtitles", "path", task.Path, "error", err)
		}
	}
	persistEmbeddedStreams(ctx, dbConn, globalID, embeddedSubtitles, embeddedAudio)
	return nil
}

func EnrichLibraryTasks(
	ctx context.Context,
	dbConn *sql.DB,
	root, mediaType string,
	libraryID int,
	tasks []EnrichmentTask,
	options ScanOptions,
) error {
	if len(tasks) == 0 {
		return nil
	}
	if mediaType == "" {
		mediaType = LibraryTypeMovie
	}

	// Keep enrichment narrow to the paths discovery actually touched.
	unique := make([]EnrichmentTask, 0, len(tasks))
	seen := make(map[string]struct{}, len(tasks))
	for _, task := range tasks {
		if task.Path == "" {
			continue
		}
		if _, ok := seen[task.Path]; ok {
			continue
		}
		seen[task.Path] = struct{}{}
		unique = append(unique, task)
	}
	if len(unique) == 0 {
		return nil
	}

	jobs := make(chan EnrichmentTask)
	errs := make(chan error, len(unique))
	workerCount := enrichmentWorkerCount
	if workerCount > len(unique) {
		workerCount = len(unique)
	}
	if workerCount < 1 {
		workerCount = 1
	}

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range jobs {
				if options.Activity != nil && task.Path != "" {
					options.Activity(ScanActivity{
						Phase:  "enrichment",
						Target: "file",
						Path:   task.Path,
					})
				}
				if err := enrichTask(ctx, dbConn, root, mediaType, libraryID, task, options); err != nil {
					errs <- err
				}
			}
		}()
	}
	for _, task := range unique {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return ctx.Err()
		case jobs <- task:
		}
	}
	close(jobs)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			return err
		}

	}
	return nil
}

func HandleScanLibraryWithOptions(
	ctx context.Context,
	dbConn *sql.DB,
	root, mediaType string,
	libraryID int,
	options ScanOptions,
) (ScanResult, error) {
	result := ScanResult{}
	if mediaType == "" {
		mediaType = LibraryTypeMovie
	}
	if libraryID <= 0 {
		return result, fmt.Errorf("library id is required")
	}

	kind := mediaType
	table := mediaTableForKind(kind)
	identifier := options.Identifier
	musicIdentifier := options.MusicIdentifier
	probeMedia := options.ProbeMedia
	probeEmbeddedSubtitleStreams := options.ProbeEmbeddedSubtitles && probeMedia
	scanSidecarSubtitles := options.ScanSidecarSubtitles
	hashMode := options.HashMode
	if hashMode == "" {
		hashMode = ScanHashModeInline
	}
	scanSubpaths, err := NormalizeScanSubpaths(options.Subpaths)
	if err != nil {
		return result, err
	}
	scanRoots, markRoots, err := resolveScanRoots(root, scanSubpaths)
	if err != nil {
		return result, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	existingByPath, err := preloadExistingMediaByPath(dbConn, table, kind, libraryID)
	if err != nil {
		return result, err
	}
	seenPaths := map[string]struct{}{}
	emitProgress := func() {
		if options.Progress != nil {
			options.Progress(ScanProgress{
				Processed: result.Added + result.Updated + result.Skipped,
				Result:    result,
			})
		}
	}
	for _, scanRoot := range scanRoots {
		err = iterateLibraryFiles(ctx, scanRoot, kind, func(path string) {
			if options.Activity != nil {
				options.Activity(ScanActivity{
					Phase:  "discovery",
					Target: "directory",
					Path:   path,
				})
			}
		}, func() {
			result.Skipped++
			emitProgress()
		}, func(candidate scanCandidate) error {
			if options.Activity != nil {
				options.Activity(ScanActivity{
					Phase:  "discovery",
					Target: "file",
					Path:   candidate.Path,
				})
			}
			path := candidate.Path
			if _, ok := seenPaths[path]; ok {
				return nil
			}
			seenPaths[path] = struct{}{}

			relPath, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			candidate.RelPath = relPath

			existing := existingByPath[path]
			isNew := existing.RefID == 0

			title := strings.TrimSuffix(candidate.Name, filepath.Ext(candidate.Name))
			if title == "" {
				title = candidate.Name
			}

			mItem := MediaItem{
				Title:         title,
				Path:          path,
				Type:          kind,
				MatchStatus:   MatchStatusLocal,
				FileSizeBytes: candidate.Size,
				FileModTime:   candidate.ModTime,
				FileHash:      existing.FileHash,
				FileHashKind:  existing.FileHashKind,
			}
			var fileInfo metadata.MediaInfo
			var musicInfo metadata.MusicInfo
			switch kind {
			case LibraryTypeMusic:
				pathInfo := metadata.ParsePathForMusic(candidate.RelPath, candidate.Name)
				audioMeta := metadata.MusicMetadata{}
				if probeMedia && !SkipFFprobeInScan {
					if probed, duration, err := readAudioMetadata(ctx, path); err == nil {
						audioMeta = probed
						mItem.Duration = duration
					}
				}
				merged := metadata.MergeMusicMetadata(pathInfo, audioMeta, title)
				mItem.Title = merged.Title
				mItem.Artist = merged.Artist
				mItem.Album = merged.Album
				mItem.AlbumArtist = merged.AlbumArtist
				mItem.DiscNumber = merged.DiscNumber
				mItem.TrackNumber = merged.TrackNumber
				mItem.ReleaseYear = merged.ReleaseYear
				musicInfo = metadata.MusicInfo{
					Title:       merged.Title,
					Artist:      merged.Artist,
					Album:       merged.Album,
					AlbumArtist: merged.AlbumArtist,
					DiscNumber:  merged.DiscNumber,
					TrackNumber: merged.TrackNumber,
					ReleaseYear: merged.ReleaseYear,
				}
			case LibraryTypeMovie:
				movieInfo := metadata.ParseMovie(candidate.RelPath, candidate.Name)
				mItem.Title = metadata.MovieDisplayTitle(movieInfo, title)
				fileInfo = metadata.MovieMediaInfo(movieInfo)
			case LibraryTypeTV, LibraryTypeAnime:
				fileInfo = metadata.ParseFilename(candidate.Name)
				pathInfo := metadata.ParsePathForTV(candidate.RelPath, candidate.Name)
				merged := metadata.MergePathInfo(pathInfo, fileInfo)
				showRoot := metadata.ShowRootPath(root, path)
				metadata.ApplyShowNFO(&merged, showRoot)
				if kind == LibraryTypeAnime && merged.IsSpecial && merged.Episode > 0 {
					merged.Season = 0
				}
				mItem.Season = merged.Season
				mItem.Episode = merged.Episode
				mItem.Title = buildEpisodeDisplayTitle(pathInfo.ShowName, merged, title, fileInfo.Title)
				fileInfo = merged
			}

			if isNew && (kind == LibraryTypeTV || kind == LibraryTypeAnime) {
				relocated, ok, err := findRelocatedTVEpisodeRow(ctx, dbConn, table, kind, libraryID, mItem.Season, mItem.Episode, root, path)
				if err != nil {
					return err
				}
				if ok {
					delete(existingByPath, relocated.Path)
					existing = relocated
					isNew = false
					mItem.FileHash = existing.FileHash
					mItem.FileHashKind = existing.FileHashKind
				}
			}

			hasStableFileState := !isNew &&
				existing.MissingSince == "" &&
				existing.FileSizeBytes == candidate.Size &&
				existing.FileModTime == candidate.ModTime
			isUnchanged := !isNew &&
				existing.Path == path &&
				hasStableFileState &&
				existing.FileHash != "" &&
				existing.FileHashKind != ""

			identifyInfo := fileInfo
			hasMetadata := existingHasMetadata(kind, existing)
			forceRefresh := kind != LibraryTypeMusic && hasExplicitProviderID(identifyInfo) && !existing.MetadataConfirmed
			shouldIdentify := identifier != nil &&
				(kind == LibraryTypeTV || kind == LibraryTypeAnime || kind == LibraryTypeMovie) &&
				(!hasMetadata || forceRefresh)
			shouldIdentifyMusic := kind == LibraryTypeMusic && musicIdentifier != nil
			if isUnchanged && !shouldIdentify && !shouldIdentifyMusic {
				if err := markMediaPresent(ctx, dbConn, table, existing.RefID, candidate.Size, candidate.ModTime, existing.FileHash, existing.FileHashKind, now); err != nil {
					return err
				}
				if existing.GlobalID > 0 {
					if err := upsertMediaFileForMediaID(ctx, dbConn, existing.GlobalID, MediaItem{
						Path:          path,
						Duration:      existing.Duration,
						FileSizeBytes: candidate.Size,
						FileModTime:   candidate.ModTime,
						FileHash:      existing.FileHash,
						FileHashKind:  existing.FileHashKind,
					}, true); err != nil {
						return err
					}
					if probeMedia && !SkipFFprobeInScan && kind != LibraryTypeMusic && !existing.PrimaryIntroProbed {
						var embeddedSubs []EmbeddedSubtitle
						var embeddedAudioTracks []EmbeddedAudioTrack
						if probed, err := readVideoMetadata(ctx, path); err == nil {
							if probeEmbeddedSubtitleStreams {
								embeddedSubs = probed.EmbeddedSubtitles
							}
							embeddedAudioTracks = probed.EmbeddedAudioTracks
							if err := UpdateMediaFileIntroFromProbe(ctx, dbConn, existing.GlobalID, path, probed); err != nil {
								slog.Warn("persist intro chapters", "media_id", existing.GlobalID, "path", path, "error", err)
							}
							persistEmbeddedStreams(ctx, dbConn, existing.GlobalID, embeddedSubs, embeddedAudioTracks)
						}
					}
				}
				result.Updated++
				emitProgress()
				return nil
			}
			if shouldIdentify {
				mItem.MetadataReviewNeeded = false
				mItem.MetadataConfirmed = false
				switch kind {
				case LibraryTypeTV:
					if res := identifier.IdentifyTV(ctx, identifyInfo); res != nil {
						applyMatchResultToMediaItem(&mItem, res)
						mItem.MatchStatus = MatchStatusIdentified
					} else {
						mItem.MatchStatus = MatchStatusUnmatched
					}
				case LibraryTypeAnime:
					if res := identifier.IdentifyAnime(ctx, identifyInfo); res != nil {
						applyMatchResultToMediaItem(&mItem, res)
						mItem.MatchStatus = MatchStatusIdentified
					} else {
						mItem.MatchStatus = MatchStatusUnmatched
					}
				case LibraryTypeMovie:
					if res := identifier.IdentifyMovie(ctx, identifyInfo); res != nil {
						applyMatchResultToMediaItem(&mItem, res)
						mItem.MatchStatus = MatchStatusIdentified
					} else {
						mItem.MatchStatus = MatchStatusUnmatched
					}
				}
			} else if shouldIdentifyMusic {
				if res := musicIdentifier.IdentifyMusic(ctx, musicInfo); res != nil {
					applyMusicMatchResultToMediaItem(&mItem, res)
					mItem.MatchStatus = MatchStatusIdentified
				} else {
					mItem.MatchStatus = MatchStatusUnmatched
				}
			} else if !isNew {
				applyExistingMetadata(&mItem, existing, kind)
			}
			if (kind == LibraryTypeTV || kind == LibraryTypeAnime) && !shouldIdentify {
				mItem.MetadataReviewNeeded = existing.MetadataReviewNeeded
				mItem.MetadataConfirmed = existing.MetadataConfirmed
			}
			if (shouldIdentify || shouldIdentifyMusic) && mItem.MatchStatus == MatchStatusUnmatched {
				result.Unmatched++
			}

			itemHashMode := hashMode
			needsHashBackfill := hashMode == ScanHashModeDefer &&
				hasStableFileState &&
				!shouldIdentify &&
				!shouldIdentifyMusic &&
				(existing.FileHash == "" || existing.FileHashKind == "")
			if needsHashBackfill {
				// A deferred discovery pass may have been interrupted before enrichment ran.
				itemHashMode = ScanHashModeInline
			}
			if itemHashMode == ScanHashModeDefer {
				// Discovery scans can defer hashing so rows become visible quickly.
				mItem.FileHash = ""
				mItem.FileHashKind = ""
			} else if hasStableFileState && mItem.FileHash != "" && mItem.FileHashKind != "" {
				// Hash already valid for this size/mtime; keep existing values.
			} else if hasStableFileState && mItem.FileHash != "" && mItem.FileHashKind == "" {
				// Legacy row: hash computed before file_hash_kind existed.
				mItem.FileHashKind = fileHashKindSHA256
			} else {
				if hash, err := computeMediaHash(ctx, path); err == nil {
					mItem.FileHash = hash
					mItem.FileHashKind = fileHashKindSampledSHA256
				} else {
					return err
				}
			}

			refID := existing.RefID
			globalID := existing.GlobalID
			if isNew {
				refID, globalID, err = insertScannedItem(ctx, dbConn, table, kind, libraryID, mItem, now)
				if err != nil {
					return err
				}
				result.Added++
			} else {
				if err := updateScannedItem(ctx, dbConn, table, refID, mItem, now); err != nil {
					return err
				}
				result.Updated++
			}
			if globalID > 0 {
				if err := upsertMediaFileForMediaID(ctx, dbConn, globalID, mItem, true); err != nil {
					return err
				}
			}
			emitProgress()

			if kind == LibraryTypeMusic {
				return nil
			}
			if scanSidecarSubtitles {
				if err := scanForSubtitles(ctx, dbConn, globalID, path); err != nil {
					slog.Warn("scan subtitles", "path", path, "error", err)
				}
			}

			var (
				embeddedSubs        []EmbeddedSubtitle
				embeddedAudioTracks []EmbeddedAudioTrack
			)
			if probeMedia && !SkipFFprobeInScan {
				if probed, err := readVideoMetadata(ctx, path); err == nil {
					if mItem.Duration == 0 && probed.Duration > 0 {
						mItem.Duration = probed.Duration
						if err := updateMediaDuration(ctx, dbConn, table, refID, mItem.Duration); err != nil {
							return err
						}
						if globalID > 0 {
							if err := upsertMediaFileForMediaID(ctx, dbConn, globalID, mItem, true); err != nil {
								return err
							}
						}
					}
					if probeEmbeddedSubtitleStreams {
						embeddedSubs = probed.EmbeddedSubtitles
					}
					embeddedAudioTracks = probed.EmbeddedAudioTracks
					if globalID > 0 {
						if err := UpdateMediaFileIntroFromProbe(ctx, dbConn, globalID, path, probed); err != nil {
							slog.Warn("persist intro chapters", "media_id", globalID, "path", path, "error", err)
						}
					}
				}
			}
			persistEmbeddedStreams(ctx, dbConn, globalID, embeddedSubs, embeddedAudioTracks)
			return nil
		})
		if err != nil {
			return result, err
		}
	}
	if err := markMissingMedia(ctx, dbConn, table, kind, libraryID, markRoots, seenPaths, now); err != nil {
		return result, err
	}
	emitProgress()
	return result, nil
}

const (
	// fileHashKindSHA256 marks a legacy full-file SHA-256 (entire byte stream).
	fileHashKindSHA256 = "sha256"
	// fileHashKindSampledSHA256 is SHA-256 over file size (8-byte BE) plus three 1 MiB samples:
	// start, middle, end. For files ≤ 3 MiB the whole file is hashed (same as full-file for tiny media).
	fileHashKindSampledSHA256 = "sampled-sha256-v1"
)

// hashSampleBlock is the size of each sampled region for sampled-sha256-v1.
const hashSampleBlock = 1 << 20

// hashReadThrottleBytesPerSec limits sequential read throughput during hashing so library scans
// do not saturate the disk (0 disables throttling).
const hashReadThrottleBytesPerSec = 32 * 1024 * 1024

func NormalizeScanSubpaths(subpaths []string) ([]string, error) {
	if len(subpaths) == 0 {
		return nil, nil
	}
	normalized := make([]string, 0, len(subpaths))
	for _, subpath := range subpaths {
		subpath = strings.TrimSpace(subpath)
		if subpath == "" || subpath == "." {
			return nil, nil
		}
		clean := filepath.Clean(subpath)
		if clean == "." {
			return nil, nil
		}
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
			return nil, fmt.Errorf("invalid scan subpath %q", subpath)
		}
		normalized = append(normalized, clean)
	}
	sort.Strings(normalized)
	out := make([]string, 0, len(normalized))
	for _, subpath := range normalized {
		if len(out) > 0 && isSubpath(out[len(out)-1], subpath) {
			continue
		}
		out = append(out, subpath)
	}
	return out, nil
}

func isSubpath(parent, child string) bool {
	if parent == child {
		return true
	}
	return strings.HasPrefix(child, parent+string(os.PathSeparator))
}

func resolveScanRoots(root string, subpaths []string) ([]string, []string, error) {
	if len(subpaths) == 0 {
		return []string{root}, []string{root}, nil
	}
	roots := make([]string, 0, len(subpaths))
	markRoots := make([]string, 0, len(subpaths))
	for _, subpath := range subpaths {
		scanRoot := filepath.Join(root, subpath)
		markRoots = append(markRoots, scanRoot)
		info, err := os.Stat(scanRoot)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, nil, err
		}
		if !info.IsDir() {
			return nil, nil, fmt.Errorf("scan subpath is not a directory: %s", subpath)
		}
		roots = append(roots, scanRoot)
	}
	return roots, markRoots, nil
}

func applyMatchResultToMediaItem(item *MediaItem, res *metadata.MatchResult) {
	if item == nil || res == nil {
		return
	}
	item.Title = res.Title
	item.Overview = res.Overview
	item.PosterPath = res.PosterURL
	item.BackdropPath = res.BackdropURL
	item.ReleaseDate = res.ReleaseDate
	item.VoteAverage = res.VoteAverage
	item.IMDbID = res.IMDbID
	item.IMDbRating = res.IMDbRating
	switch res.Provider {
	case "tmdb":
		if id, err := parseInt(res.ExternalID); err == nil {
			item.TMDBID = id
			item.TVDBID = ""
		}
	case "tvdb":
		item.TVDBID = res.ExternalID
	}
}

func applyMusicMatchResultToMediaItem(item *MediaItem, res *metadata.MusicMatchResult) {
	if item == nil || res == nil {
		return
	}
	if res.Title != "" {
		item.Title = res.Title
	}
	if res.Artist != "" {
		item.Artist = res.Artist
	}
	if res.Album != "" {
		item.Album = res.Album
	}
	if res.AlbumArtist != "" {
		item.AlbumArtist = res.AlbumArtist
	}
	if res.PosterURL != "" {
		item.PosterPath = res.PosterURL
	}
	if res.ReleaseYear > 0 {
		item.ReleaseYear = res.ReleaseYear
	}
	if res.DiscNumber > 0 {
		item.DiscNumber = res.DiscNumber
	}
	if res.TrackNumber > 0 {
		item.TrackNumber = res.TrackNumber
	}
	item.MusicBrainzArtistID = res.ArtistID
	item.MusicBrainzReleaseGroupID = res.ReleaseGroupID
	item.MusicBrainzReleaseID = res.ReleaseID
	item.MusicBrainzRecordingID = res.RecordingID
}

func applyExistingMetadata(item *MediaItem, existing existingMediaRow, kind string) {
	if item == nil {
		return
	}
	item.MatchStatus = existing.MatchStatus
	item.PosterPath = existing.PosterPath
	item.BackdropPath = existing.BackdropPath
	item.Overview = existing.Overview
	item.ReleaseDate = existing.ReleaseDate
	item.VoteAverage = existing.VoteAverage
	item.TMDBID = existing.TMDBID
	item.TVDBID = existing.TVDBID
	item.IMDbID = existing.IMDbID
	item.IMDbRating = existing.IMDbRating
	item.MusicBrainzArtistID = existing.MusicBrainzArtistID
	item.MusicBrainzReleaseGroupID = existing.MusicBrainzReleaseGroupID
	item.MusicBrainzReleaseID = existing.MusicBrainzReleaseID
	item.MusicBrainzRecordingID = existing.MusicBrainzRecordingID
	if kind == LibraryTypeTV || kind == LibraryTypeAnime {
		item.MetadataReviewNeeded = existing.MetadataReviewNeeded
		item.MetadataConfirmed = existing.MetadataConfirmed
	}
}

func hashThrottleAfterRead(ctx context.Context, n int) error {
	if n <= 0 || hashReadThrottleBytesPerSec <= 0 {
		return nil
	}
	d := time.Duration(n) * time.Second / time.Duration(hashReadThrottleBytesPerSec)
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func writeFileSizeToHash(h hash.Hash, size int64) {
	_ = binary.Write(h, binary.BigEndian, uint64(size))
}

func hashFullFileThrottled(ctx context.Context, f *os.File, h hash.Hash, size int64) error {
	buf := make([]byte, hashSampleBlock)
	var read int64
	for read < size {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		n, err := f.Read(buf)
		if n > 0 {
			if _, werr := h.Write(buf[:n]); werr != nil {
				return werr
			}
			read += int64(n)
			if err := hashThrottleAfterRead(ctx, n); err != nil {
				return err
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func hashReadAtThrottled(ctx context.Context, f *os.File, h hash.Hash, off int64, size int64) error {
	if off < 0 || off >= size {
		return nil
	}
	n := int64(hashSampleBlock)
	if off+n > size {
		n = size - off
	}
	if n <= 0 {
		return nil
	}
	buf := make([]byte, n)
	got, err := f.ReadAt(buf, off)
	if got > 0 {
		if _, werr := h.Write(buf[:got]); werr != nil {
			return werr
		}
		if err := hashThrottleAfterRead(ctx, got); err != nil {
			return err
		}
	}
	if err != nil && err != io.EOF {
		return err
	}
	return nil
}

func computeFileHash(ctx context.Context, path string) (string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	size := fi.Size()

	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	writeFileSizeToHash(h, size)

	if size <= 3*hashSampleBlock {
		if err := hashFullFileThrottled(ctx, f, h, size); err != nil {
			return "", err
		}
	} else {
		mid := (size - hashSampleBlock) / 2
		for _, off := range []int64{0, mid, size - hashSampleBlock} {
			if err := hashReadAtThrottled(ctx, f, h, off, size); err != nil {
				return "", err
			}
		}
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func markMediaPresent(ctx context.Context, dbConn *sql.DB, table string, refID int, fileSizeBytes int64, fileModTime, fileHash, fileHashKind, seenAt string) error {
	_, err := dbConn.ExecContext(
		ctx,
		`UPDATE `+table+` SET file_size_bytes = ?, file_mod_time = ?, file_hash = ?, file_hash_kind = ?, last_seen_at = ?, missing_since = NULL WHERE id = ?`,
		fileSizeBytes,
		nullStr(fileModTime),
		nullStr(fileHash),
		nullStr(fileHashKind),
		nullStr(seenAt),
		refID,
	)
	return err
}

func markMissingMedia(ctx context.Context, dbConn *sql.DB, table, kind string, libraryID int, scanRoots []string, seenPaths map[string]struct{}, missingSince string) error {
	rows, err := dbConn.Query(`SELECT id, path FROM `+table+` WHERE library_id = ?`, libraryID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var staleIDs []int
	for rows.Next() {
		var refID int
		var path string
		if err := rows.Scan(&refID, &path); err != nil {
			return err
		}
		if _, ok := seenPaths[path]; ok {
			continue
		}
		if !pathWithinAnyRoot(path, scanRoots) {
			continue
		}
		staleIDs = append(staleIDs, refID)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return batchUpdateMissingMedia(ctx, dbConn, table, kind, staleIDs, missingSince)
}

func pathWithinAnyRoot(path string, roots []string) bool {
	for _, root := range roots {
		if path == root || strings.HasPrefix(path, root+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

type existingMediaRow struct {
	RefID                     int
	GlobalID                  int
	Path                      string
	FileSizeBytes             int64
	FileModTime               string
	FileHash                  string
	FileHashKind              string
	Duration                  int
	LastSeenAt                string
	MissingSince              string
	TMDBID                    int
	TVDBID                    string
	IMDbID                    string
	IMDbRating                float64
	PosterPath                string
	BackdropPath              string
	Overview                  string
	ReleaseDate               string
	VoteAverage               float64
	MusicBrainzArtistID       string
	MusicBrainzReleaseGroupID string
	MusicBrainzReleaseID      string
	MusicBrainzRecordingID    string
	MatchStatus               string
	MetadataReviewNeeded      bool
	MetadataConfirmed         bool
	// PrimaryIntroProbed is true when the primary media_files row has intro_probed_at set (intro detection pass completed).
	// Always true for music libraries. Loaded by preloadExistingMediaByPath for video rows.
	PrimaryIntroProbed bool
}

func allowedExtensions(kind string) map[string]struct{} {
	if kind == LibraryTypeMusic {
		return audioExtensions
	}
	return videoExtensions
}

func shouldSkipScanPath(kind, relPath, filename string) bool {
	if kind != LibraryTypeMovie {
		return false
	}
	return metadata.ParseMovie(relPath, filename).IsExtra
}

func buildEpisodeDisplayTitle(showName string, info metadata.MediaInfo, fallbackTitle, fileTitle string) string {
	displayShow := strings.TrimSpace(showName)
	if normalized := strings.ToLower(displayShow); strings.HasPrefix(normalized, "season ") || strings.HasPrefix(normalized, "s0") {
		displayShow = ""
	}
	if candidate := prettifyDisplayTitle(info.Title); candidate != "" && (displayShow == "" || len(displayShow) <= 2) {
		displayShow = candidate
	}
	if displayShow == "" && info.Title != "" {
		displayShow = prettifyDisplayTitle(info.Title)
	}
	if displayShow == "" {
		displayShow = fallbackTitle
	}
	if info.Episode > 0 {
		title := fmt.Sprintf("%s - S%02dE%02d", displayShow, info.Season, info.Episode)
		extraTitle := prettifyTitle(fileTitle)
		if extraTitle != "" &&
			!metadata.IsGenericEpisodeTitle(fileTitle, info.Season, info.Episode) &&
			!strings.EqualFold(metadata.NormalizeSeriesTitle(extraTitle), metadata.NormalizeSeriesTitle(displayShow)) {
			title += " - " + extraTitle
		}
		return title
	}
	return displayShow
}

func prettifyTitle(s string) string {
	s = strings.TrimSpace(strings.TrimSuffix(s, filepath.Ext(s)))
	s = strings.ReplaceAll(s, ".", " ")
	s = strings.ReplaceAll(s, "_", " ")
	return strings.TrimSpace(s)
}

func prettifyDisplayTitle(s string) string {
	s = prettifyTitle(s)
	if s == strings.ToLower(s) {
		words := strings.Fields(s)
		for i, word := range words {
			if word == "" {
				continue
			}
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
		return strings.Join(words, " ")
	}
	return s
}

func preloadExistingMediaByPath(dbConn *sql.DB, table, kind string, libraryID int) (map[string]existingMediaRow, error) {
	query := `SELECT m.path, m.id, COALESCE(g.id, 0), COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.last_seen_at, ''), COALESCE(m.missing_since, ''), COALESCE(m.match_status, 'local') FROM ` + table + ` m
LEFT JOIN media_global g ON g.kind = ? AND g.ref_id = m.id
WHERE m.library_id = ?`
	if table == "music_tracks" {
		query = `SELECT m.path, m.id, COALESCE(g.id, 0), COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.duration, 0), COALESCE(m.last_seen_at, ''), COALESCE(m.missing_since, ''), COALESCE(m.match_status, 'local'), COALESCE(m.poster_path, ''), COALESCE(m.musicbrainz_artist_id, ''), COALESCE(m.musicbrainz_release_group_id, ''), COALESCE(m.musicbrainz_release_id, ''), COALESCE(m.musicbrainz_recording_id, ''), 1 FROM music_tracks m
LEFT JOIN media_global g ON g.kind = 'music' AND g.ref_id = m.id
WHERE m.library_id = ?`
	}
	if table == "tv_episodes" || table == "anime_episodes" {
		query = `SELECT m.path, m.id, COALESCE(g.id, 0), COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.duration, 0), COALESCE(m.last_seen_at, ''), COALESCE(m.missing_since, ''), COALESCE(m.tmdb_id, 0), COALESCE(m.tvdb_id, ''), COALESCE(m.imdb_id, ''), COALESCE(m.imdb_rating, 0), COALESCE(m.match_status, 'local'), COALESCE(m.poster_path, ''), COALESCE(m.backdrop_path, ''), COALESCE(m.overview, ''), COALESCE(m.release_date, ''), COALESCE(m.vote_average, 0), COALESCE(m.metadata_review_needed, 0), COALESCE(m.metadata_confirmed, 0),
CASE WHEN (mf.intro_probed_at IS NOT NULL AND TRIM(mf.intro_probed_at) != '') OR COALESCE(mf.intro_locked, 0) != 0 THEN 1 ELSE 0 END
FROM ` + table + ` m
LEFT JOIN media_global g ON g.kind = ? AND g.ref_id = m.id
LEFT JOIN media_files mf ON mf.media_id = g.id AND mf.is_primary = 1
WHERE m.library_id = ?`
	} else if table != "music_tracks" {
		query = `SELECT m.path, m.id, COALESCE(g.id, 0), COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.duration, 0), COALESCE(m.last_seen_at, ''), COALESCE(m.missing_since, ''), COALESCE(m.tmdb_id, 0), COALESCE(m.tvdb_id, ''), COALESCE(m.imdb_id, ''), COALESCE(m.imdb_rating, 0), COALESCE(m.match_status, 'local'), COALESCE(m.poster_path, ''), COALESCE(m.backdrop_path, ''), COALESCE(m.overview, ''), COALESCE(m.release_date, ''), COALESCE(m.vote_average, 0),
CASE WHEN (mf.intro_probed_at IS NOT NULL AND TRIM(mf.intro_probed_at) != '') OR COALESCE(mf.intro_locked, 0) != 0 THEN 1 ELSE 0 END
FROM ` + table + ` m
LEFT JOIN media_global g ON g.kind = ? AND g.ref_id = m.id
LEFT JOIN media_files mf ON mf.media_id = g.id AND mf.is_primary = 1
WHERE m.library_id = ?`
	}

	var (
		rows *sql.Rows
		err  error
	)
	if table == "music_tracks" {
		rows, err = dbConn.Query(query, libraryID)
	} else {
		rows, err = dbConn.Query(query, kind, libraryID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]existingMediaRow)
	for rows.Next() {
		var row existingMediaRow
		switch table {
		case "music_tracks":
			var musicIntroFlag int
			if err := rows.Scan(&row.Path, &row.RefID, &row.GlobalID, &row.FileSizeBytes, &row.FileModTime, &row.FileHash, &row.FileHashKind, &row.Duration, &row.LastSeenAt, &row.MissingSince, &row.MatchStatus, &row.PosterPath, &row.MusicBrainzArtistID, &row.MusicBrainzReleaseGroupID, &row.MusicBrainzReleaseID, &row.MusicBrainzRecordingID, &musicIntroFlag); err != nil {
				return nil, err
			}
			row.PrimaryIntroProbed = musicIntroFlag != 0
		case "tv_episodes", "anime_episodes":
			var introProbeDone int
			if err := rows.Scan(&row.Path, &row.RefID, &row.GlobalID, &row.FileSizeBytes, &row.FileModTime, &row.FileHash, &row.FileHashKind, &row.Duration, &row.LastSeenAt, &row.MissingSince, &row.TMDBID, &row.TVDBID, &row.IMDbID, &row.IMDbRating, &row.MatchStatus, &row.PosterPath, &row.BackdropPath, &row.Overview, &row.ReleaseDate, &row.VoteAverage, &row.MetadataReviewNeeded, &row.MetadataConfirmed, &introProbeDone); err != nil {
				return nil, err
			}
			row.PrimaryIntroProbed = introProbeDone != 0
		default:
			var introProbeDone int
			if err := rows.Scan(&row.Path, &row.RefID, &row.GlobalID, &row.FileSizeBytes, &row.FileModTime, &row.FileHash, &row.FileHashKind, &row.Duration, &row.LastSeenAt, &row.MissingSince, &row.TMDBID, &row.TVDBID, &row.IMDbID, &row.IMDbRating, &row.MatchStatus, &row.PosterPath, &row.BackdropPath, &row.Overview, &row.ReleaseDate, &row.VoteAverage, &introProbeDone); err != nil {
				return nil, err
			}
			row.PrimaryIntroProbed = introProbeDone != 0
		}
		out[row.Path] = row
	}
	return out, rows.Err()
}

func lookupExistingMedia(dbConn *sql.DB, table, kind string, libraryID int, path string) (existingMediaRow, error) {
	var row existingMediaRow
	if table == "music_tracks" {
		err := dbConn.QueryRow(`SELECT m.id, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.duration, 0), COALESCE(m.last_seen_at, ''), COALESCE(m.missing_since, ''), COALESCE(m.match_status, 'local'), COALESCE(m.poster_path, ''), COALESCE(m.musicbrainz_artist_id, ''), COALESCE(m.musicbrainz_release_group_id, ''), COALESCE(m.musicbrainz_release_id, ''), COALESCE(m.musicbrainz_recording_id, '') FROM music_tracks m WHERE m.library_id = ? AND m.path = ?`, libraryID, path).
			Scan(&row.RefID, &row.FileSizeBytes, &row.FileModTime, &row.FileHash, &row.FileHashKind, &row.Duration, &row.LastSeenAt, &row.MissingSince, &row.MatchStatus, &row.PosterPath, &row.MusicBrainzArtistID, &row.MusicBrainzReleaseGroupID, &row.MusicBrainzReleaseID, &row.MusicBrainzRecordingID)
		if errors.Is(err, sql.ErrNoRows) {
			return row, nil
		}
		if err != nil {
			return row, err
		}
	} else {
		var tvdbID, imdbID sql.NullString
		var posterPath, backdropPath, overview, releaseDate sql.NullString
		var voteAverage, imdbRating sql.NullFloat64
		var err error
		if table == "tv_episodes" || table == "anime_episodes" {
			var metadataReviewNeeded sql.NullBool
			var metadataConfirmed sql.NullBool
			err = dbConn.QueryRow(`SELECT m.id, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.duration, 0), COALESCE(m.last_seen_at, ''), COALESCE(m.missing_since, ''), COALESCE(m.tmdb_id, 0), m.tvdb_id, m.imdb_id, COALESCE(m.imdb_rating, 0), COALESCE(m.match_status, 'local'), m.poster_path, m.backdrop_path, m.overview, m.release_date, COALESCE(m.vote_average, 0), COALESCE(m.metadata_review_needed, 0), COALESCE(m.metadata_confirmed, 0) FROM `+table+` m WHERE m.library_id = ? AND m.path = ?`, libraryID, path).
				Scan(&row.RefID, &row.FileSizeBytes, &row.FileModTime, &row.FileHash, &row.FileHashKind, &row.Duration, &row.LastSeenAt, &row.MissingSince, &row.TMDBID, &tvdbID, &imdbID, &imdbRating, &row.MatchStatus, &posterPath, &backdropPath, &overview, &releaseDate, &voteAverage, &metadataReviewNeeded, &metadataConfirmed)
			if metadataReviewNeeded.Valid {
				row.MetadataReviewNeeded = metadataReviewNeeded.Bool
			}
			if metadataConfirmed.Valid {
				row.MetadataConfirmed = metadataConfirmed.Bool
			}
		} else {
			err = dbConn.QueryRow(`SELECT m.id, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.duration, 0), COALESCE(m.last_seen_at, ''), COALESCE(m.missing_since, ''), COALESCE(m.tmdb_id, 0), m.tvdb_id, m.imdb_id, COALESCE(m.imdb_rating, 0), COALESCE(m.match_status, 'local'), m.poster_path, m.backdrop_path, m.overview, m.release_date, COALESCE(m.vote_average, 0) FROM `+table+` m WHERE m.library_id = ? AND m.path = ?`, libraryID, path).
				Scan(&row.RefID, &row.FileSizeBytes, &row.FileModTime, &row.FileHash, &row.FileHashKind, &row.Duration, &row.LastSeenAt, &row.MissingSince, &row.TMDBID, &tvdbID, &imdbID, &imdbRating, &row.MatchStatus, &posterPath, &backdropPath, &overview, &releaseDate, &voteAverage)
		}
		if errors.Is(err, sql.ErrNoRows) {
			return row, nil
		}
		if err != nil {
			return row, err
		}
		if tvdbID.Valid {
			row.TVDBID = tvdbID.String
		}
		if imdbID.Valid {
			row.IMDbID = imdbID.String
		}
		if imdbRating.Valid {
			row.IMDbRating = imdbRating.Float64
		}
		if posterPath.Valid {
			row.PosterPath = posterPath.String
		}
		if backdropPath.Valid {
			row.BackdropPath = backdropPath.String
		}
		if overview.Valid {
			row.Overview = overview.String
		}
		if releaseDate.Valid {
			row.ReleaseDate = releaseDate.String
		}
		if voteAverage.Valid {
			row.VoteAverage = voteAverage.Float64
		}
	}
	row.Path = path
	_ = dbConn.QueryRow(`SELECT id FROM media_global WHERE kind = ? AND ref_id = ?`, kind, row.RefID).Scan(&row.GlobalID)
	return row, nil
}

func insertScannedItem(ctx context.Context, dbConn *sql.DB, table, kind string, libraryID int, mItem MediaItem, seenAt string) (int, int, error) {
	tx, err := dbConn.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}

	var refID int
	switch table {
	case "music_tracks":
		err = tx.QueryRowContext(ctx, `INSERT INTO music_tracks (library_id, title, path, duration, file_size_bytes, file_mod_time, file_hash, file_hash_kind, last_seen_at, missing_since, match_status, artist, album, album_artist, poster_path, musicbrainz_artist_id, musicbrainz_release_group_id, musicbrainz_release_id, musicbrainz_recording_id, disc_number, track_number, release_year) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			libraryID, mItem.Title, mItem.Path, mItem.Duration, mItem.FileSizeBytes, nullStr(mItem.FileModTime), nullStr(mItem.FileHash), nullStr(mItem.FileHashKind), nullStr(seenAt), mItem.MatchStatus, nullStr(mItem.Artist), nullStr(mItem.Album), nullStr(mItem.AlbumArtist), nullStr(mItem.PosterPath), nullStr(mItem.MusicBrainzArtistID), nullStr(mItem.MusicBrainzReleaseGroupID), nullStr(mItem.MusicBrainzReleaseID), nullStr(mItem.MusicBrainzRecordingID), mItem.DiscNumber, mItem.TrackNumber, mItem.ReleaseYear).Scan(&refID)
		if err != nil {
			_ = tx.Rollback()
			return 0, 0, err
		}
	case "tv_episodes", "anime_episodes":
		err = tx.QueryRowContext(ctx, `INSERT INTO `+table+` (library_id, title, path, duration, file_size_bytes, file_mod_time, file_hash, file_hash_kind, last_seen_at, missing_since, match_status, tmdb_id, tvdb_id, overview, poster_path, backdrop_path, release_date, vote_average, imdb_id, imdb_rating, season, episode, metadata_review_needed, metadata_confirmed) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			libraryID, mItem.Title, mItem.Path, mItem.Duration, mItem.FileSizeBytes, nullStr(mItem.FileModTime), nullStr(mItem.FileHash), nullStr(mItem.FileHashKind), nullStr(seenAt), mItem.MatchStatus, mItem.TMDBID, nullStr(mItem.TVDBID), nullStr(mItem.Overview), nullStr(mItem.PosterPath), nullStr(mItem.BackdropPath), nullStr(mItem.ReleaseDate), nullFloat64(mItem.VoteAverage), nullStr(mItem.IMDbID), nullFloat64(mItem.IMDbRating), mItem.Season, mItem.Episode, mItem.MetadataReviewNeeded, mItem.MetadataConfirmed).Scan(&refID)
		if err != nil {
			_ = tx.Rollback()
			return 0, 0, err
		}
	default:
		err = tx.QueryRowContext(ctx, `INSERT INTO `+table+` (library_id, title, path, duration, file_size_bytes, file_mod_time, file_hash, file_hash_kind, last_seen_at, missing_since, match_status, tmdb_id, tvdb_id, overview, poster_path, backdrop_path, release_date, vote_average, imdb_id, imdb_rating) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			libraryID, mItem.Title, mItem.Path, mItem.Duration, mItem.FileSizeBytes, nullStr(mItem.FileModTime), nullStr(mItem.FileHash), nullStr(mItem.FileHashKind), nullStr(seenAt), mItem.MatchStatus, mItem.TMDBID, nullStr(mItem.TVDBID), nullStr(mItem.Overview), nullStr(mItem.PosterPath), nullStr(mItem.BackdropPath), nullStr(mItem.ReleaseDate), nullFloat64(mItem.VoteAverage), nullStr(mItem.IMDbID), nullFloat64(mItem.IMDbRating)).Scan(&refID)
		if err != nil {
			_ = tx.Rollback()
			return 0, 0, err
		}
	}
	var globalID int
	err = tx.QueryRowContext(ctx, `INSERT INTO media_global (kind, ref_id) VALUES (?, ?) RETURNING id`, kind, refID).Scan(&globalID)
	if err != nil {
		_ = tx.Rollback()
		return 0, 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return refID, globalID, nil
}

func updateScannedItem(ctx context.Context, dbConn *sql.DB, table string, refID int, mItem MediaItem, seenAt string) error {
	if table == "music_tracks" {
		_, err := dbConn.ExecContext(ctx, `UPDATE music_tracks SET title = ?, path = ?, duration = ?, file_size_bytes = ?, file_mod_time = ?, file_hash = ?, file_hash_kind = ?, last_seen_at = ?, missing_since = NULL, match_status = ?, artist = ?, album = ?, album_artist = ?, poster_path = ?, musicbrainz_artist_id = ?, musicbrainz_release_group_id = ?, musicbrainz_release_id = ?, musicbrainz_recording_id = ?, disc_number = ?, track_number = ?, release_year = ? WHERE id = ?`,
			mItem.Title, mItem.Path, mItem.Duration, mItem.FileSizeBytes, nullStr(mItem.FileModTime), nullStr(mItem.FileHash), nullStr(mItem.FileHashKind), nullStr(seenAt), mItem.MatchStatus, nullStr(mItem.Artist), nullStr(mItem.Album), nullStr(mItem.AlbumArtist), nullStr(mItem.PosterPath), nullStr(mItem.MusicBrainzArtistID), nullStr(mItem.MusicBrainzReleaseGroupID), nullStr(mItem.MusicBrainzReleaseID), nullStr(mItem.MusicBrainzRecordingID), mItem.DiscNumber, mItem.TrackNumber, mItem.ReleaseYear, refID)
		return err
	}
	if table == "tv_episodes" || table == "anime_episodes" {
		_, err := dbConn.ExecContext(ctx, `UPDATE `+table+` SET title = ?, path = ?, duration = ?, file_size_bytes = ?, file_mod_time = ?, file_hash = ?, file_hash_kind = ?, last_seen_at = ?, missing_since = NULL, match_status = ?, tmdb_id = ?, tvdb_id = ?, overview = ?, poster_path = ?, backdrop_path = ?, release_date = ?, vote_average = ?, imdb_id = ?, imdb_rating = ?, season = ?, episode = ?, metadata_review_needed = ?, metadata_confirmed = ? WHERE id = ?`,
			mItem.Title, mItem.Path, mItem.Duration, mItem.FileSizeBytes, nullStr(mItem.FileModTime), nullStr(mItem.FileHash), nullStr(mItem.FileHashKind), nullStr(seenAt), mItem.MatchStatus, mItem.TMDBID, nullStr(mItem.TVDBID), nullStr(mItem.Overview), nullStr(mItem.PosterPath), nullStr(mItem.BackdropPath), nullStr(mItem.ReleaseDate), nullFloat64(mItem.VoteAverage), nullStr(mItem.IMDbID), nullFloat64(mItem.IMDbRating), mItem.Season, mItem.Episode, mItem.MetadataReviewNeeded, mItem.MetadataConfirmed, refID)
		return err
	}
	_, err := dbConn.ExecContext(ctx, `UPDATE `+table+` SET title = ?, path = ?, duration = ?, file_size_bytes = ?, file_mod_time = ?, file_hash = ?, file_hash_kind = ?, last_seen_at = ?, missing_since = NULL, match_status = ?, tmdb_id = ?, tvdb_id = ?, overview = ?, poster_path = ?, backdrop_path = ?, release_date = ?, vote_average = ?, imdb_id = ?, imdb_rating = ? WHERE id = ?`,
		mItem.Title, mItem.Path, mItem.Duration, mItem.FileSizeBytes, nullStr(mItem.FileModTime), nullStr(mItem.FileHash), nullStr(mItem.FileHashKind), nullStr(seenAt), mItem.MatchStatus, mItem.TMDBID, nullStr(mItem.TVDBID), nullStr(mItem.Overview), nullStr(mItem.PosterPath), nullStr(mItem.BackdropPath), nullStr(mItem.ReleaseDate), nullFloat64(mItem.VoteAverage), nullStr(mItem.IMDbID), nullFloat64(mItem.IMDbRating), refID)
	return err
}

func updateMediaDuration(ctx context.Context, dbConn *sql.DB, table string, refID int, duration int) error {
	_, err := dbConn.ExecContext(ctx, `UPDATE `+table+` SET duration = ? WHERE id = ?`, duration, refID)
	return err
}

func pruneMissingMedia(ctx context.Context, dbConn *sql.DB, table, kind string, libraryID int, seenPaths map[string]struct{}) (int, error) {
	rows, err := dbConn.Query(`SELECT m.id, m.path, COALESCE(g.id, 0) FROM `+table+` m LEFT JOIN media_global g ON g.kind = ? AND g.ref_id = m.id WHERE m.library_id = ?`, kind, libraryID)
	if err != nil {
		return 0, err
	}
	type staleRow struct {
		refID    int
		globalID int
		path     string
	}
	var stale []staleRow
	for rows.Next() {
		var refID, globalID int
		var path string
		if err := rows.Scan(&refID, &path, &globalID); err != nil {
			rows.Close()
			return 0, err
		}
		if _, ok := seenPaths[path]; ok {
			continue
		}
		stale = append(stale, staleRow{refID: refID, globalID: globalID, path: path})
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	removed := 0
	for _, row := range stale {
		if row.globalID > 0 {
			if _, err := dbConn.ExecContext(ctx, `DELETE FROM subtitles WHERE media_id = ?`, row.globalID); err != nil {
				return removed, err
			}
			if _, err := dbConn.ExecContext(ctx, `DELETE FROM embedded_subtitles WHERE media_id = ?`, row.globalID); err != nil {
				return removed, err
			}
			if _, err := dbConn.ExecContext(ctx, `DELETE FROM embedded_audio_tracks WHERE media_id = ?`, row.globalID); err != nil {
				return removed, err
			}
			if _, err := dbConn.ExecContext(ctx, `DELETE FROM media_global WHERE id = ?`, row.globalID); err != nil {
				return removed, err
			}
		}
		if _, err := dbConn.ExecContext(ctx, `DELETE FROM `+table+` WHERE id = ?`, row.refID); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

func parseInt(s string) (int, error) {
	return strconv.Atoi(s)
}

func hasExplicitProviderID(info metadata.MediaInfo) bool {
	return info.TMDBID > 0 || info.TVDBID != ""
}

func existingHasMetadata(kind string, row existingMediaRow) bool {
	if (kind == LibraryTypeTV || kind == LibraryTypeAnime) && row.MetadataConfirmed {
		return true
	}
	hasProviderID := row.TMDBID != 0
	if kind != LibraryTypeAnime {
		hasProviderID = hasProviderID || row.TVDBID != ""
	}
	return hasProviderID && row.IMDbID != ""
}
