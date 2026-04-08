package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"plum/internal/ffopts"
)

// attachSubtitlesBatch loads subtitle and embedded stream metadata for all items in batch queries.
func attachSubtitlesBatch(db *sql.DB, items []MediaItem) ([]MediaItem, error) {
	if len(items) == 0 {
		return items, nil
	}
	ids := make([]int, len(items))
	for i := range items {
		ids[i] = items[i].ID
	}
	subsByID, err := getSubtitlesByMediaIDs(db, ids)
	if err != nil {
		return nil, err
	}
	embByID, err := getEmbeddedSubtitlesByMediaIDs(db, ids)
	if err != nil {
		return nil, err
	}
	audioByID, err := getEmbeddedAudioTracksByMediaIDs(db, ids)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if subs := subsByID[items[i].ID]; subs != nil {
			items[i].Subtitles = subs
		} else {
			items[i].Subtitles = []Subtitle{}
		}
		if embedded := embByID[items[i].ID]; embedded != nil {
			items[i].EmbeddedSubtitles = embedded
		} else {
			items[i].EmbeddedSubtitles = []EmbeddedSubtitle{}
		}
		if audioTracks := audioByID[items[i].ID]; audioTracks != nil {
			items[i].EmbeddedAudioTracks = audioTracks
		} else {
			items[i].EmbeddedAudioTracks = []EmbeddedAudioTrack{}
		}
	}
	return items, nil
}

func EnsurePlaybackTrackMetadata(ctx context.Context, db *sql.DB, item *MediaItem) error {
	_, err := RefreshPlaybackTrackMetadata(ctx, db, item)
	return err
}

func RefreshPlaybackTrackMetadata(ctx context.Context, db *sql.DB, item *MediaItem) (PlaybackTrackMetadata, error) {
	metadata := PlaybackTrackMetadata{
		Subtitles:           []Subtitle{},
		EmbeddedSubtitles:   []EmbeddedSubtitle{},
		EmbeddedAudioTracks: []EmbeddedAudioTrack{},
	}
	if item == nil || item.ID <= 0 {
		return metadata, nil
	}

	sourcePath, err := ResolveMediaSourcePath(db, *item)
	if err != nil {
		return metadata, err
	}
	item.Path = sourcePath

	if item.Type == LibraryTypeMusic {
		return metadata, nil
	}

	if err := scanForSubtitles(ctx, db, item.ID, sourcePath); err != nil {
		slog.Warn("refresh playback sidecar subtitles", "media_id", item.ID, "path", sourcePath, "error", err)
	}

	subtitles, err := getSubtitlesForMedia(db, item.ID)
	if err != nil {
		return metadata, err
	}
	if subtitles != nil {
		metadata.Subtitles = subtitles
	}

	probed, err := readVideoMetadata(ctx, sourcePath)
	if err != nil {
		slog.Warn("refresh playback embedded tracks", "media_id", item.ID, "path", sourcePath, "error", err)
		embeddedSubtitles, embeddedAudioTracks, getErr := getPersistedPlaybackTracks(db, item.ID)
		if getErr != nil {
			return metadata, getErr
		}
		metadata.EmbeddedSubtitles = embeddedSubtitles
		metadata.EmbeddedAudioTracks = embeddedAudioTracks
	} else {
		persistEmbeddedStreams(ctx, db, item.ID, probed.EmbeddedSubtitles, probed.EmbeddedAudioTracks)
		metadata.EmbeddedSubtitles = append(metadata.EmbeddedSubtitles, probed.EmbeddedSubtitles...)
		metadata.EmbeddedAudioTracks = append(metadata.EmbeddedAudioTracks, probed.EmbeddedAudioTracks...)
		if err := UpdateMediaFileIntroFromProbe(ctx, db, item.ID, sourcePath, probed); err != nil {
			slog.Warn("persist intro chapters", "media_id", item.ID, "path", sourcePath, "error", err)
		}
		locked, lockErr := MediaFileIntroLocked(ctx, db, item.ID)
		if lockErr != nil {
			slog.Warn("intro lock read", "media_id", item.ID, "error", lockErr)
		}
		if locked {
			if err := ApplyPrimaryMediaIntroCreditsToItem(db, item); err != nil {
				slog.Warn("reload intro after locked probe", "media_id", item.ID, "error", err)
			}
		} else {
			if probed.IntroStartSeconds != nil {
				v := *probed.IntroStartSeconds
				item.IntroStartSeconds = &v
			} else {
				item.IntroStartSeconds = nil
			}
			if probed.IntroEndSeconds != nil {
				v := *probed.IntroEndSeconds
				item.IntroEndSeconds = &v
			} else {
				item.IntroEndSeconds = nil
			}
		}
	}

	item.Subtitles = append([]Subtitle{}, metadata.Subtitles...)
	item.EmbeddedSubtitles = append([]EmbeddedSubtitle{}, metadata.EmbeddedSubtitles...)
	item.EmbeddedAudioTracks = append([]EmbeddedAudioTrack{}, metadata.EmbeddedAudioTracks...)
	return metadata, nil
}

// RefreshPlaybackTrackMetadataForLibrary runs RefreshPlaybackTrackMetadata for every present
// (non-missing) item in the library. Music libraries are no-ops. Per-item errors are counted in
// failed and do not abort the rest.
func RefreshPlaybackTrackMetadataForLibrary(ctx context.Context, dbConn *sql.DB, libraryID int) (refreshed int, failed int, err error) {
	var typ string
	if err := dbConn.QueryRow(`SELECT type FROM libraries WHERE id = ?`, libraryID).Scan(&typ); err != nil {
		return 0, 0, err
	}
	if typ == LibraryTypeMusic {
		return 0, 0, nil
	}
	items, err := GetMediaByLibraryID(dbConn, libraryID)
	if err != nil {
		return 0, 0, err
	}
	for i := range items {
		it := items[i]
		_, rerr := RefreshPlaybackTrackMetadata(ctx, dbConn, &it)
		if rerr != nil {
			slog.Warn("refresh playback tracks", "library_id", libraryID, "media_id", it.ID, "error", rerr)
			failed++
		} else {
			refreshed++
		}
	}
	return refreshed, failed, nil
}

// RefreshIntroProbeOnly runs ffprobe/chapter/silence intro detection and persists results (unless intro_locked).
// Does not refresh embedded subtitle/audio track rows or sidecar subtitle scan.
func RefreshIntroProbeOnly(ctx context.Context, dbConn *sql.DB, item *MediaItem) error {
	if item == nil || item.ID <= 0 || item.Type == LibraryTypeMusic {
		return nil
	}
	sourcePath, err := ResolveMediaSourcePath(dbConn, *item)
	if err != nil {
		return err
	}
	probed, err := readVideoMetadata(ctx, sourcePath)
	if err != nil {
		return err
	}
	if err := UpdateMediaFileIntroFromProbe(ctx, dbConn, item.ID, sourcePath, probed); err != nil {
		return err
	}
	locked, err := MediaFileIntroLocked(ctx, dbConn, item.ID)
	if err != nil {
		return err
	}
	if locked {
		return ApplyPrimaryMediaIntroCreditsToItem(dbConn, item)
	}
	if probed.IntroStartSeconds != nil {
		v := *probed.IntroStartSeconds
		item.IntroStartSeconds = &v
	} else {
		item.IntroStartSeconds = nil
	}
	if probed.IntroEndSeconds != nil {
		v := *probed.IntroEndSeconds
		item.IntroEndSeconds = &v
	} else {
		item.IntroEndSeconds = nil
	}
	return nil
}

// RefreshIntroProbeOnlyForLibrary runs RefreshIntroProbeOnly for every non-missing item in the library.
func RefreshIntroProbeOnlyForLibrary(ctx context.Context, dbConn *sql.DB, libraryID int) (refreshed int, failed int, err error) {
	var typ string
	if err := dbConn.QueryRow(`SELECT type FROM libraries WHERE id = ?`, libraryID).Scan(&typ); err != nil {
		return 0, 0, err
	}
	if typ == LibraryTypeMusic {
		return 0, 0, nil
	}
	items, err := GetMediaByLibraryID(dbConn, libraryID)
	if err != nil {
		return 0, 0, err
	}
	for i := range items {
		it := items[i]
		if it.Missing {
			continue
		}
		rerr := RefreshIntroProbeOnly(ctx, dbConn, &it)
		if rerr != nil {
			slog.Warn("intro probe refresh", "library_id", libraryID, "media_id", it.ID, "error", rerr)
			failed++
		} else {
			refreshed++
		}
	}
	return refreshed, failed, nil
}

func getPersistedPlaybackTracks(db *sql.DB, mediaID int) ([]EmbeddedSubtitle, []EmbeddedAudioTrack, error) {
	embeddedSubtitles, err := getEmbeddedSubtitlesForMedia(db, mediaID)
	if err != nil {
		return nil, nil, err
	}
	embeddedAudioTracks, err := getEmbeddedAudioTracksForMedia(db, mediaID)
	if err != nil {
		return nil, nil, err
	}
	if embeddedSubtitles == nil {
		embeddedSubtitles = []EmbeddedSubtitle{}
	}
	if embeddedAudioTracks == nil {
		embeddedAudioTracks = []EmbeddedAudioTrack{}
	}
	return embeddedSubtitles, embeddedAudioTracks, nil
}

func getSubtitlesByMediaIDs(db *sql.DB, mediaIDs []int) (map[int][]Subtitle, error) {
	if len(mediaIDs) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(mediaIDs))
	args := make([]interface{}, len(mediaIDs))
	for i := range mediaIDs {
		placeholders[i] = "?"
		args[i] = mediaIDs[i]
	}
	query := `SELECT id, media_id, title, language, format, path FROM subtitles WHERE media_id IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int][]Subtitle)
	for rows.Next() {
		var s Subtitle
		if err := rows.Scan(&s.ID, &s.MediaID, &s.Title, &s.Language, &s.Format, &s.Path); err != nil {
			return nil, err
		}
		out[s.MediaID] = append(out[s.MediaID], s)
	}
	return out, rows.Err()
}

func getEmbeddedSubtitlesByMediaIDs(db *sql.DB, mediaIDs []int) (map[int][]EmbeddedSubtitle, error) {
	if len(mediaIDs) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(mediaIDs))
	args := make([]interface{}, len(mediaIDs))
	for i := range mediaIDs {
		placeholders[i] = "?"
		args[i] = mediaIDs[i]
	}
	query := `SELECT media_id, stream_index, language, title, COALESCE(codec, ''), supported FROM embedded_subtitles WHERE media_id IN (` + strings.Join(placeholders, ",") + `) ORDER BY media_id, stream_index`
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int][]EmbeddedSubtitle)
	for rows.Next() {
		var s EmbeddedSubtitle
		var supportedInt sql.NullInt64
		if err := rows.Scan(&s.MediaID, &s.StreamIndex, &s.Language, &s.Title, &s.Codec, &supportedInt); err != nil {
			return nil, err
		}
		if supportedInt.Valid {
			v := supportedInt.Int64 != 0
			s.Supported = &v
		}
		out[s.MediaID] = append(out[s.MediaID], s)
	}
	return out, rows.Err()
}

func getEmbeddedAudioTracksByMediaIDs(db *sql.DB, mediaIDs []int) (map[int][]EmbeddedAudioTrack, error) {
	if len(mediaIDs) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(mediaIDs))
	args := make([]interface{}, len(mediaIDs))
	for i := range mediaIDs {
		placeholders[i] = "?"
		args[i] = mediaIDs[i]
	}
	query := `SELECT media_id, stream_index, language, title FROM embedded_audio_tracks WHERE media_id IN (` + strings.Join(placeholders, ",") + `) ORDER BY media_id, stream_index`
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int][]EmbeddedAudioTrack)
	for rows.Next() {
		var track EmbeddedAudioTrack
		if err := rows.Scan(&track.MediaID, &track.StreamIndex, &track.Language, &track.Title); err != nil {
			return nil, err
		}
		out[track.MediaID] = append(out[track.MediaID], track)
	}
	return out, rows.Err()
}

func GetMediaByID(db *sql.DB, id int) (*MediaItem, error) {
	var kind string
	var refID int
	err := db.QueryRow(`SELECT kind, ref_id FROM media_global WHERE id = ?`, id).Scan(&kind, &refID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	table := mediaTableForKind(kind)
	var libID int
	var title, path string
	var duration int
	var season, episode int
	var metadataReviewNeeded sql.NullBool
	var metadataConfirmed sql.NullBool
	var overview, posterPath, backdropPath, releaseDate, thumbnailPath, matchStatus, imdbID sql.NullString
	var voteAvg, imdbRating sql.NullFloat64
	var tmdbID sql.NullInt64
	var tvdbID sql.NullString
	var artist, album, albumArtist sql.NullString
	var musicPosterPath sql.NullString
	var discNumber, trackNumber, releaseYear int
	var fileSizeBytes int64
	var fileModTime, fileHash, fileHashKind, missingSince string
	switch table {
	case "music_tracks":
		err = db.QueryRow(`SELECT m.id, m.library_id, m.title, m.path, m.duration, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.missing_since, ''), m.match_status, m.artist, m.album, m.album_artist, m.poster_path, COALESCE(m.disc_number, 0), COALESCE(m.track_number, 0), COALESCE(m.release_year, 0) FROM music_tracks m WHERE m.id = ?`, refID).
			Scan(&refID, &libID, &title, &path, &duration, &fileSizeBytes, &fileModTime, &fileHash, &fileHashKind, &missingSince, &matchStatus, &artist, &album, &albumArtist, &musicPosterPath, &discNumber, &trackNumber, &releaseYear)
	case "tv_episodes", "anime_episodes":
		err = db.QueryRow(`SELECT m.id, m.library_id, m.title, m.path, m.duration, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.missing_since, ''), m.match_status, m.tmdb_id, m.tvdb_id, m.overview, m.poster_path, m.backdrop_path, m.release_date, m.vote_average, m.imdb_id, m.imdb_rating, COALESCE(m.season, 0), COALESCE(m.episode, 0), COALESCE(m.metadata_review_needed, 0), COALESCE(m.metadata_confirmed, 0), m.thumbnail_path FROM `+table+` m WHERE m.id = ?`, refID).
			Scan(&refID, &libID, &title, &path, &duration, &fileSizeBytes, &fileModTime, &fileHash, &fileHashKind, &missingSince, &matchStatus, &tmdbID, &tvdbID, &overview, &posterPath, &backdropPath, &releaseDate, &voteAvg, &imdbID, &imdbRating, &season, &episode, &metadataReviewNeeded, &metadataConfirmed, &thumbnailPath)
	default:
		err = db.QueryRow(`SELECT m.id, m.library_id, m.title, m.path, m.duration, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.missing_since, ''), m.match_status, m.tmdb_id, m.tvdb_id, m.overview, m.poster_path, m.backdrop_path, m.release_date, m.vote_average, m.imdb_id, m.imdb_rating FROM `+table+` m WHERE m.id = ?`, refID).
			Scan(&refID, &libID, &title, &path, &duration, &fileSizeBytes, &fileModTime, &fileHash, &fileHashKind, &missingSince, &matchStatus, &tmdbID, &tvdbID, &overview, &posterPath, &backdropPath, &releaseDate, &voteAvg, &imdbID, &imdbRating)
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	m := MediaItem{
		ID:            id,
		LibraryID:     libID,
		Title:         title,
		Path:          path,
		Duration:      duration,
		Type:          kind,
		FileSizeBytes: fileSizeBytes,
		FileModTime:   fileModTime,
		FileHash:      fileHash,
		FileHashKind:  fileHashKind,
		MissingSince:  missingSince,
	}
	if matchStatus.Valid {
		m.MatchStatus = matchStatus.String
	}
	m.Missing = m.MissingSince != ""
	switch table {
	case "tv_episodes", "anime_episodes":
		m.Season = season
		m.Episode = episode
		if metadataReviewNeeded.Valid {
			m.MetadataReviewNeeded = metadataReviewNeeded.Bool
		}
		if metadataConfirmed.Valid {
			m.MetadataConfirmed = metadataConfirmed.Bool
		}
		if thumbnailPath.Valid {
			m.ThumbnailPath = thumbnailPath.String
		}
	case "music_tracks":
		if artist.Valid {
			m.Artist = artist.String
		}
		if album.Valid {
			m.Album = album.String
		}
		if albumArtist.Valid {
			m.AlbumArtist = albumArtist.String
		}
		if musicPosterPath.Valid {
			m.PosterPath = musicPosterPath.String
		}
		m.DiscNumber = discNumber
		m.TrackNumber = trackNumber
		m.ReleaseYear = releaseYear
	}
	if overview.Valid {
		m.Overview = overview.String
	}
	if posterPath.Valid {
		m.PosterPath = posterPath.String
	}
	if backdropPath.Valid {
		m.BackdropPath = backdropPath.String
	}
	if releaseDate.Valid {
		m.ReleaseDate = releaseDate.String
	}
	if voteAvg.Valid {
		m.VoteAverage = voteAvg.Float64
	}
	if imdbID.Valid {
		m.IMDbID = imdbID.String
	}
	if imdbRating.Valid {
		m.IMDbRating = imdbRating.Float64
	}
	if tmdbID.Valid {
		m.TMDBID = int(tmdbID.Int64)
	}
	if tvdbID.Valid {
		m.TVDBID = tvdbID.String
	}
	if file, err := lookupPrimaryMediaFile(db, id); err == nil {
		m.Path = file.Path
		if file.Duration > 0 {
			m.Duration = file.Duration
		}
		m.FileSizeBytes = file.FileSizeBytes
		m.FileModTime = file.FileModTime
		m.FileHash = file.FileHash
		m.FileHashKind = file.FileHashKind
		m.MissingSince = file.MissingSince
		m.Missing = file.MissingSince != ""
		if file.IntroStartSec.Valid {
			v := file.IntroStartSec.Float64
			m.IntroStartSeconds = &v
		}
		if file.IntroEndSec.Valid {
			v := file.IntroEndSec.Float64
			m.IntroEndSeconds = &v
		}
		m.IntroLocked = file.IntroLocked != 0
		if file.CreditsStartSec.Valid {
			v := file.CreditsStartSec.Float64
			m.CreditsStartSeconds = &v
		}
		if file.CreditsEndSec.Valid {
			v := file.CreditsEndSec.Float64
			m.CreditsEndSeconds = &v
		}
	}
	decorateMediaItemURLs(&m)
	subs, err := getSubtitlesForMedia(db, id)
	if err != nil {
		return nil, err
	}
	if subs != nil {
		m.Subtitles = subs
	} else {
		m.Subtitles = []Subtitle{}
	}
	emb, err := getEmbeddedSubtitlesForMedia(db, id)
	if err != nil {
		return nil, err
	}
	if emb != nil {
		m.EmbeddedSubtitles = emb
	} else {
		m.EmbeddedSubtitles = []EmbeddedSubtitle{}
	}
	audioTracks, err := getEmbeddedAudioTracksForMedia(db, id)
	if err != nil {
		return nil, err
	}
	if audioTracks != nil {
		m.EmbeddedAudioTracks = audioTracks
	} else {
		m.EmbeddedAudioTracks = []EmbeddedAudioTrack{}
	}
	dupSlice := []MediaItem{m}
	dupSlice, err = attachDuplicateState(db, dupSlice)
	if err != nil {
		return nil, err
	}
	m = dupSlice[0]
	return &m, nil
}

// GetMediaByLibraryID returns all media for a library (one category table only), no N+1.
func GetMediaByLibraryID(db *sql.DB, libraryID int) ([]MediaItem, error) {
	var typ string
	err := db.QueryRow(`SELECT type FROM libraries WHERE id = ?`, libraryID).Scan(&typ)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []MediaItem{}, nil
		}
		return nil, err
	}
	if typ == LibraryTypeTV || typ == LibraryTypeAnime {
		if err := ensureLibraryShowsAndSeasons(db, libraryID, typ); err != nil {
			return nil, err
		}
	}
	items, _, err := queryMediaByLibraryID(db, libraryID, typ, 0, 0)
	if err != nil {
		return nil, err
	}
	items, err = attachMediaFilesBatch(db, items)
	if err != nil {
		return nil, err
	}
	items, err = attachSubtitlesBatch(db, items)
	if err != nil {
		return nil, err
	}
	return attachDuplicateState(db, items)
}

func GetMediaPageByLibraryID(db *sql.DB, libraryID int, offset int, limit int) (LibraryMediaPage, error) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 60
	}
	var typ string
	err := db.QueryRow(`SELECT type FROM libraries WHERE id = ?`, libraryID).Scan(&typ)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LibraryMediaPage{Items: []MediaItem{}, HasMore: false, Total: 0}, nil
		}
		return LibraryMediaPage{}, err
	}
	if typ == LibraryTypeTV || typ == LibraryTypeAnime {
		if err := ensureLibraryShowsAndSeasons(db, libraryID, typ); err != nil {
			return LibraryMediaPage{}, err
		}
	}
	items, total, err := queryMediaByLibraryID(db, libraryID, typ, offset, limit)
	if err != nil {
		return LibraryMediaPage{}, err
	}
	items, err = attachMediaFilesBatch(db, items)
	if err != nil {
		return LibraryMediaPage{}, err
	}
	hasMore := offset+len(items) < total
	var nextOffset *int
	if hasMore {
		value := offset + len(items)
		nextOffset = &value
	}
	return LibraryMediaPage{
		Items:      items,
		NextOffset: nextOffset,
		HasMore:    hasMore,
		Total:      total,
	}, nil
}

// queryMediaByLibraryID queries the single category table for this library.
func queryMediaByLibraryID(db *sql.DB, libraryID int, kind string, offset int, limit int) ([]MediaItem, int, error) {
	table := mediaTableForKind(kind)
	countQuery := `SELECT COUNT(1) FROM ` + table + ` WHERE library_id = ? AND COALESCE(missing_since, '') = ''`
	var total int
	if err := db.QueryRow(countQuery, libraryID).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := `SELECT g.id, m.library_id, m.title, m.path, m.duration, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.missing_since, ''), m.match_status, m.tmdb_id, m.tvdb_id, m.overview, m.poster_path, m.backdrop_path, m.release_date, m.vote_average, m.imdb_id, m.imdb_rating
FROM ` + table + ` m
JOIN media_global g ON g.kind = ? AND g.ref_id = m.id
WHERE m.library_id = ? AND COALESCE(m.missing_since, '') = ''
ORDER BY g.id`
	switch table {
	case "music_tracks":
		q = `SELECT g.id, m.library_id, m.title, m.path, m.duration, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.missing_since, ''), m.match_status, m.artist, m.album, m.album_artist, m.poster_path, COALESCE(m.disc_number, 0), COALESCE(m.track_number, 0), COALESCE(m.release_year, 0)
FROM music_tracks m
JOIN media_global g ON g.kind = 'music' AND g.ref_id = m.id
WHERE m.library_id = ? AND COALESCE(m.missing_since, '') = ''
ORDER BY g.id`
	case "tv_episodes", "anime_episodes":
		q = `SELECT g.id, m.library_id, m.title, m.path, m.duration, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.missing_since, ''), m.match_status, m.tmdb_id, m.tvdb_id, m.overview, m.poster_path, m.backdrop_path, m.release_date, m.vote_average, m.imdb_id, m.imdb_rating, COALESCE(m.season, 0), COALESCE(m.episode, 0), COALESCE(m.metadata_review_needed, 0), COALESCE(m.metadata_confirmed, 0), m.thumbnail_path, COALESCE(s.poster_path, ''), COALESCE(s.vote_average, 0), COALESCE(s.imdb_rating, 0)
FROM ` + table + ` m
JOIN media_global g ON g.kind = ? AND g.ref_id = m.id
LEFT JOIN shows s ON s.id = m.show_id
WHERE m.library_id = ? AND COALESCE(m.missing_since, '') = ''
ORDER BY g.id`
	default:
		q = `SELECT g.id, m.library_id, m.title, m.path, m.duration, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.missing_since, ''), m.match_status, m.tmdb_id, m.tvdb_id, m.overview, m.poster_path, m.backdrop_path, m.release_date, m.vote_average, m.imdb_id, m.imdb_rating
FROM ` + table + ` m
JOIN media_global g ON g.kind = ? AND g.ref_id = m.id
WHERE m.library_id = ? AND COALESCE(m.missing_since, '') = ''
ORDER BY g.id`
	}
	if limit > 0 {
		q += ` LIMIT ? OFFSET ?`
	}
	var rows *sql.Rows
	var err error
	if table == "music_tracks" {
		if limit > 0 {
			rows, err = db.Query(q, libraryID, limit, offset)
		} else {
			rows, err = db.Query(q, libraryID)
		}
	} else {
		if limit > 0 {
			rows, err = db.Query(q, kind, libraryID, limit, offset)
		} else {
			rows, err = db.Query(q, kind, libraryID)
		}
	}
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]MediaItem, 0)
	for rows.Next() {
		var m MediaItem
		m.Type = kind
		m.LibraryID = libraryID
		var overview, posterPath, backdropPath, releaseDate, thumbnailPath, matchStatus, imdbID sql.NullString
		var showPosterPath sql.NullString
		var voteAvg, showVoteAvg, showImdbAvg, imdbRating sql.NullFloat64
		var tmdbID sql.NullInt64
		var tvdbID sql.NullString
		var metadataReviewNeeded sql.NullBool
		var metadataConfirmed sql.NullBool
		var artist, album, albumArtist sql.NullString
		var musicPosterPath sql.NullString
		switch table {
		case "music_tracks":
			err = rows.Scan(&m.ID, &m.LibraryID, &m.Title, &m.Path, &m.Duration, &m.FileSizeBytes, &m.FileModTime, &m.FileHash, &m.FileHashKind, &m.MissingSince, &matchStatus, &artist, &album, &albumArtist, &musicPosterPath, &m.DiscNumber, &m.TrackNumber, &m.ReleaseYear)
			if artist.Valid {
				m.Artist = artist.String
			}
			if album.Valid {
				m.Album = album.String
			}
			if albumArtist.Valid {
				m.AlbumArtist = albumArtist.String
			}
			if musicPosterPath.Valid {
				m.PosterPath = musicPosterPath.String
			}
		case "tv_episodes", "anime_episodes":
			err = rows.Scan(&m.ID, &m.LibraryID, &m.Title, &m.Path, &m.Duration, &m.FileSizeBytes, &m.FileModTime, &m.FileHash, &m.FileHashKind, &m.MissingSince, &matchStatus, &tmdbID, &tvdbID, &overview, &posterPath, &backdropPath, &releaseDate, &voteAvg, &imdbID, &imdbRating, &m.Season, &m.Episode, &metadataReviewNeeded, &metadataConfirmed, &thumbnailPath, &showPosterPath, &showVoteAvg, &showImdbAvg)
			m.TMDBID = int(tmdbID.Int64)
			if tvdbID.Valid {
				m.TVDBID = tvdbID.String
			}
			if overview.Valid {
				m.Overview = overview.String
			}
			if posterPath.Valid {
				m.PosterPath = posterPath.String
			}
			if backdropPath.Valid {
				m.BackdropPath = backdropPath.String
			}
			if releaseDate.Valid {
				m.ReleaseDate = releaseDate.String
			}
			if voteAvg.Valid {
				m.VoteAverage = voteAvg.Float64
			}
			if imdbID.Valid {
				m.IMDbID = imdbID.String
			}
			if imdbRating.Valid {
				m.IMDbRating = imdbRating.Float64
			}
			if metadataReviewNeeded.Valid {
				m.MetadataReviewNeeded = metadataReviewNeeded.Bool
			}
			if metadataConfirmed.Valid {
				m.MetadataConfirmed = metadataConfirmed.Bool
			}
			if thumbnailPath.Valid {
				m.ThumbnailPath = thumbnailPath.String
			}
			if showPosterPath.Valid {
				m.ShowPosterPath = showPosterPath.String
			}
			if showVoteAvg.Valid {
				m.ShowVoteAverage = showVoteAvg.Float64
			}
			if showImdbAvg.Valid {
				m.ShowIMDbRating = showImdbAvg.Float64
			}
		default:
			err = rows.Scan(&m.ID, &m.LibraryID, &m.Title, &m.Path, &m.Duration, &m.FileSizeBytes, &m.FileModTime, &m.FileHash, &m.FileHashKind, &m.MissingSince, &matchStatus, &tmdbID, &tvdbID, &overview, &posterPath, &backdropPath, &releaseDate, &voteAvg, &imdbID, &imdbRating)
			m.TMDBID = int(tmdbID.Int64)
			if tvdbID.Valid {
				m.TVDBID = tvdbID.String
			}
			if overview.Valid {
				m.Overview = overview.String
			}
			if posterPath.Valid {
				m.PosterPath = posterPath.String
			}
			if backdropPath.Valid {
				m.BackdropPath = backdropPath.String
			}
			if releaseDate.Valid {
				m.ReleaseDate = releaseDate.String
			}
			if voteAvg.Valid {
				m.VoteAverage = voteAvg.Float64
			}
			if imdbID.Valid {
				m.IMDbID = imdbID.String
			}
			if imdbRating.Valid {
				m.IMDbRating = imdbRating.Float64
			}
		}
		if matchStatus.Valid {
			m.MatchStatus = matchStatus.String
		}
		m.Missing = m.MissingSince != ""
		if err != nil {
			return nil, 0, err
		}
		items = append(items, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if table == "tv_episodes" || table == "anime_episodes" {
		if err := hydrateEpisodeShowPosters(db, libraryID, kind, items); err != nil {
			return nil, 0, err
		}
	}
	return items, total, nil
}

// queryMediaByShowID returns episode rows for a single show (indexed by shows.id), ordered by season/episode.
func queryMediaByShowID(db *sql.DB, libraryID int, kind string, showID int) ([]MediaItem, error) {
	if showID <= 0 {
		return nil, nil
	}
	return queryMediaByShowIDs(db, libraryID, kind, []int{showID})
}

// queryMediaByShowIDs returns episode rows for multiple shows in one query (same columns/order semantics as single-show).
func queryMediaByShowIDs(db *sql.DB, libraryID int, kind string, showIDs []int) ([]MediaItem, error) {
	uniq := make([]int, 0, len(showIDs))
	seenID := make(map[int]struct{}, len(showIDs))
	for _, id := range showIDs {
		if id <= 0 {
			continue
		}
		if _, ok := seenID[id]; ok {
			continue
		}
		seenID[id] = struct{}{}
		uniq = append(uniq, id)
	}
	if len(uniq) == 0 {
		return nil, nil
	}
	showIDs = uniq
	table := mediaTableForKind(kind)
	if table != "tv_episodes" && table != "anime_episodes" {
		return nil, nil
	}
	placeholders := make([]string, len(showIDs))
	args := make([]interface{}, 0, 2+len(showIDs))
	args = append(args, kind, libraryID)
	for i, id := range showIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	q := `SELECT g.id, m.library_id, m.title, m.path, m.duration, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.missing_since, ''), m.match_status, m.tmdb_id, m.tvdb_id, m.overview, m.poster_path, m.backdrop_path, m.release_date, m.vote_average, m.imdb_id, m.imdb_rating, COALESCE(m.season, 0), COALESCE(m.episode, 0), COALESCE(m.metadata_review_needed, 0), COALESCE(m.metadata_confirmed, 0), m.thumbnail_path, COALESCE(m.show_id, 0), COALESCE(s.poster_path, ''), COALESCE(s.vote_average, 0), COALESCE(s.imdb_rating, 0)
FROM ` + table + ` m
JOIN media_global g ON g.kind = ? AND g.ref_id = m.id
LEFT JOIN shows s ON s.id = m.show_id
WHERE m.library_id = ? AND m.show_id IN (` + strings.Join(placeholders, ",") + `) AND COALESCE(m.missing_since, '') = ''
ORDER BY m.show_id, COALESCE(m.season, 0), COALESCE(m.episode, 0), COALESCE(m.title, ''), g.id`
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]MediaItem, 0)
	for rows.Next() {
		var m MediaItem
		m.Type = kind
		m.LibraryID = libraryID
		var overview, posterPath, backdropPath, releaseDate, thumbnailPath, matchStatus, imdbID sql.NullString
		var showPosterPath sql.NullString
		var voteAvg, showVoteAvg, showImdbAvg, imdbRating sql.NullFloat64
		var tmdbID sql.NullInt64
		var tvdbID sql.NullString
		var metadataReviewNeeded sql.NullBool
		var metadataConfirmed sql.NullBool
		err = rows.Scan(&m.ID, &m.LibraryID, &m.Title, &m.Path, &m.Duration, &m.FileSizeBytes, &m.FileModTime, &m.FileHash, &m.FileHashKind, &m.MissingSince, &matchStatus, &tmdbID, &tvdbID, &overview, &posterPath, &backdropPath, &releaseDate, &voteAvg, &imdbID, &imdbRating, &m.Season, &m.Episode, &metadataReviewNeeded, &metadataConfirmed, &thumbnailPath, &m.ShowID, &showPosterPath, &showVoteAvg, &showImdbAvg)
		if err != nil {
			return nil, err
		}
		m.TMDBID = int(tmdbID.Int64)
		if tvdbID.Valid {
			m.TVDBID = tvdbID.String
		}
		if overview.Valid {
			m.Overview = overview.String
		}
		if posterPath.Valid {
			m.PosterPath = posterPath.String
		}
		if backdropPath.Valid {
			m.BackdropPath = backdropPath.String
		}
		if releaseDate.Valid {
			m.ReleaseDate = releaseDate.String
		}
		if voteAvg.Valid {
			m.VoteAverage = voteAvg.Float64
		}
		if imdbID.Valid {
			m.IMDbID = imdbID.String
		}
		if imdbRating.Valid {
			m.IMDbRating = imdbRating.Float64
		}
		if metadataReviewNeeded.Valid {
			m.MetadataReviewNeeded = metadataReviewNeeded.Bool
		}
		if metadataConfirmed.Valid {
			m.MetadataConfirmed = metadataConfirmed.Bool
		}
		if thumbnailPath.Valid {
			m.ThumbnailPath = thumbnailPath.String
		}
		if showPosterPath.Valid {
			m.ShowPosterPath = showPosterPath.String
		}
		if showVoteAvg.Valid {
			m.ShowVoteAverage = showVoteAvg.Float64
		}
		if showImdbAvg.Valid {
			m.ShowIMDbRating = showImdbAvg.Float64
		}
		if matchStatus.Valid {
			m.MatchStatus = matchStatus.String
		}
		m.Missing = m.MissingSince != ""
		items = append(items, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := hydrateEpisodeShowPosters(db, libraryID, kind, items); err != nil {
		return nil, err
	}
	return items, nil
}

func hydrateEpisodeShowPosters(db *sql.DB, libraryID int, kind string, items []MediaItem) error {
	if len(items) == 0 || (kind != LibraryTypeTV && kind != LibraryTypeAnime) {
		return nil
	}
	rows, err := db.Query(`SELECT COALESCE(tmdb_id, 0), COALESCE(title_key, ''), COALESCE(poster_path, '')
FROM shows
WHERE library_id = ? AND kind = ?`, libraryID, kind)
	if err != nil {
		return err
	}
	defer rows.Close()

	postersByTMDBID := make(map[int]string)
	postersByTitleKey := make(map[string]string)
	for rows.Next() {
		var tmdbID int
		var titleKey string
		var posterPath string
		if err := rows.Scan(&tmdbID, &titleKey, &posterPath); err != nil {
			return err
		}
		if posterPath == "" {
			continue
		}
		if tmdbID > 0 {
			postersByTMDBID[tmdbID] = posterPath
		}
		if titleKey != "" {
			postersByTitleKey[titleKey] = posterPath
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range items {
		if items[i].ShowPosterPath != "" {
			continue
		}
		if items[i].TMDBID > 0 {
			if posterPath := postersByTMDBID[items[i].TMDBID]; posterPath != "" {
				items[i].ShowPosterPath = posterPath
				continue
			}
		}
		titleKey := normalizeShowKeyTitle(items[i].Title)
		if titleKey == "" {
			continue
		}
		if posterPath := postersByTitleKey[titleKey]; posterPath != "" {
			items[i].ShowPosterPath = posterPath
		}
	}
	return nil
}

// duplicateHashQueryChunk limits bound variables per query (below SQLite's default SQLITE_MAX_VARIABLE_NUMBER).

func getSubtitlesForMedia(db *sql.DB, mediaID int) ([]Subtitle, error) {
	rows, err := db.Query(`SELECT id, media_id, title, language, format, path FROM subtitles WHERE media_id = ?`, mediaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []Subtitle
	for rows.Next() {
		var s Subtitle
		if err := rows.Scan(&s.ID, &s.MediaID, &s.Title, &s.Language, &s.Format, &s.Path); err != nil {
			return nil, err
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}

func getEmbeddedSubtitlesForMedia(db *sql.DB, mediaID int) ([]EmbeddedSubtitle, error) {
	rows, err := db.Query(`SELECT media_id, stream_index, language, title, COALESCE(codec, ''), supported FROM embedded_subtitles WHERE media_id = ? ORDER BY stream_index`, mediaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []EmbeddedSubtitle
	for rows.Next() {
		var s EmbeddedSubtitle
		var supportedInt sql.NullInt64
		if err := rows.Scan(&s.MediaID, &s.StreamIndex, &s.Language, &s.Title, &s.Codec, &supportedInt); err != nil {
			return nil, err
		}
		if supportedInt.Valid {
			v := supportedInt.Int64 != 0
			s.Supported = &v
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}

func getEmbeddedAudioTracksForMedia(db *sql.DB, mediaID int) ([]EmbeddedAudioTrack, error) {
	rows, err := db.Query(`SELECT media_id, stream_index, language, title FROM embedded_audio_tracks WHERE media_id = ? ORDER BY stream_index`, mediaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []EmbeddedAudioTrack
	for rows.Next() {
		var track EmbeddedAudioTrack
		if err := rows.Scan(&track.MediaID, &track.StreamIndex, &track.Language, &track.Title); err != nil {
			return nil, err
		}
		tracks = append(tracks, track)
	}
	return tracks, rows.Err()
}

func GetSubtitleByID(db *sql.DB, id int) (*Subtitle, error) {
	var s Subtitle
	err := db.QueryRow(`SELECT id, media_id, title, language, format, path FROM subtitles WHERE id = ?`, id).
		Scan(&s.ID, &s.MediaID, &s.Title, &s.Language, &s.Format, &s.Path)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

func getMediaDuration(ctx context.Context, path string) (int, error) {
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, "ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	var d float64
	if _, err := fmt.Sscanf(string(out), "%f", &d); err != nil {
		return 0, err
	}
	return int(d), nil
}

type VideoProbeResult struct {
	Duration            int
	EmbeddedSubtitles   []EmbeddedSubtitle
	EmbeddedAudioTracks []EmbeddedAudioTrack
	IntroStartSeconds   *float64
	IntroEndSeconds     *float64
}

func probeVideoMetadata(ctx context.Context, path string) (VideoProbeResult, error) {
	// Large UHD remuxes: ffprobe may need to read InputProbeBeforeI (128 MiB) from slow disks/NAS;
	// a short timeout yields empty embedded subtitle metadata so clients only see in-band CEA-608.
	probeCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	args := []string{"-v", "error"}
	args = append(args, ffopts.InputProbeBeforeI...)
	args = append(args,
		"-show_entries", "format=duration:stream=index,codec_type,codec_name:stream_tags=language,title",
		"-show_entries", "chapter=start_time,end_time:chapter_tags=title",
		"-of", "json",
		path,
	)
	cmd := exec.CommandContext(probeCtx, "ffprobe", args...)
	out, err := cmd.Output()
	if err != nil {
		return VideoProbeResult{}, err
	}

	var parsed struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			Index     int    `json:"index"`
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
			Tags      struct {
				Language string `json:"language"`
				Title    string `json:"title"`
			} `json:"tags"`
		} `json:"streams"`
		Chapters []struct {
			StartTime string `json:"start_time"`
			EndTime   string `json:"end_time"`
			Tags      struct {
				Title string `json:"title"`
			} `json:"tags"`
		} `json:"chapters"`
	}

	if err := json.Unmarshal(out, &parsed); err != nil {
		return VideoProbeResult{}, err
	}

	result := VideoProbeResult{}
	if parsed.Format.Duration != "" {
		if f, err := strconv.ParseFloat(parsed.Format.Duration, 64); err == nil {
			result.Duration = int(f)
		}
	}
	for _, stream := range parsed.Streams {
		lang := stream.Tags.Language
		if lang == "" {
			lang = "und"
		}
		title := stream.Tags.Title
		if title == "" {
			title = lang
		}
		switch stream.CodecType {
		case "subtitle":
			codec := strings.TrimSpace(stream.CodecName)
			var supportedPtr *bool
			if codec != "" {
				supported := isSupportedEmbeddedSubtitleCodec(codec)
				supportedPtr = &supported
			}
			result.EmbeddedSubtitles = append(result.EmbeddedSubtitles, EmbeddedSubtitle{
				StreamIndex: stream.Index,
				Language:    lang,
				Title:       title,
				Codec:       codec,
				Supported:   supportedPtr,
			})
		case "audio":
			result.EmbeddedAudioTracks = append(result.EmbeddedAudioTracks, EmbeddedAudioTrack{
				StreamIndex: stream.Index,
				Language:    lang,
				Title:       title,
			})
		}
	}
	chProbes := make([]chapterProbe, 0, len(parsed.Chapters))
	for _, ch := range parsed.Chapters {
		st, errSt := strconv.ParseFloat(ch.StartTime, 64)
		et, errEt := strconv.ParseFloat(ch.EndTime, 64)
		if errSt != nil || errEt != nil {
			continue
		}
		chProbes = append(chProbes, chapterProbe{
			startSec: st,
			endSec:   et,
			title:    ch.Tags.Title,
		})
	}
	if start, end, ok := IntroChapterRangeFromProbes(chProbes); ok {
		s, e := start, end
		// Discard intro window that extends beyond the probed duration.
		if result.Duration > 0 && e > float64(result.Duration) {
			e = float64(result.Duration)
		}
		if s >= 0 && e > s {
			result.IntroStartSeconds = &s
			result.IntroEndSeconds = &e
		}
	}
	if result.IntroEndSeconds == nil {
		if end, ok := probeIntroEndViaSilenceDetect(ctx, path, result.Duration); ok {
			e := end
			z := 0.0
			result.IntroStartSeconds = &z
			result.IntroEndSeconds = &e
		}
	}
	return result, nil
}

func isSupportedEmbeddedSubtitleCodec(codec string) bool {
	switch strings.ToLower(strings.TrimSpace(codec)) {
	case "ass", "ssa", "subrip", "srt", "webvtt", "text", "mov_text", "ttml", "tx3g",
		"hdmv_text_subtitle", // Blu-ray TextST; ffmpeg can mux to WebVTT
		"eia_608", "eia_708", // ATSC closed captions
		"hdmv_pgs_subtitle", "pgssub", "pgs": // bitmap; WebVTT ineligible but Exo/Media3 can render raw PGS
		return true
	default:
		return false
	}
}

func probeEmbeddedSubtitles(ctx context.Context, path string) ([]EmbeddedSubtitle, error) {
	result, err := probeVideoMetadata(ctx, path)
	if err != nil {
		return nil, err
	}
	return result.EmbeddedSubtitles, nil
}

func probeEmbeddedAudioTracks(ctx context.Context, path string) ([]EmbeddedAudioTrack, error) {
	result, err := probeVideoMetadata(ctx, path)
	if err != nil {
		return nil, err
	}
	return result.EmbeddedAudioTracks, nil
}

func scanForSubtitles(ctx context.Context, dbConn *sql.DB, mediaID int, videoPath string) error {
	dir := filepath.Dir(videoPath)
	base := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, base) {
			ext := strings.ToLower(filepath.Ext(name))
			if ext == ".srt" || ext == ".vtt" || ext == ".ass" || ext == ".ssa" {
				path := filepath.Join(dir, name)
				lang := "und"
				parts := strings.Split(strings.TrimSuffix(name, ext), ".")
				if len(parts) > 1 {
					lastPart := parts[len(parts)-1]
					if len(lastPart) == 2 || len(lastPart) == 3 {
						lang = lastPart
					}
				}

				_, err := dbConn.ExecContext(ctx,
					`INSERT OR IGNORE INTO subtitles (media_id, title, language, format, path) VALUES (?, ?, ?, ?, ?)`,
					mediaID, name, lang, ext[1:], path,
				)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func persistEmbeddedStreams(ctx context.Context, dbConn *sql.DB, mediaID int, subtitles []EmbeddedSubtitle, audioTracks []EmbeddedAudioTrack) {
	if mediaID <= 0 {
		return
	}
	if _, err := dbConn.ExecContext(ctx, `DELETE FROM embedded_subtitles WHERE media_id = ?`, mediaID); err != nil {
		slog.Warn("clear embedded_subtitles", "media_id", mediaID, "error", err)
	} else {
		for _, s := range subtitles {
			var supportedVal interface{}
			if s.Supported != nil {
				if *s.Supported {
					supportedVal = 1
				} else {
					supportedVal = 0
				}
			}
			if _, err := dbConn.ExecContext(ctx,
				`INSERT INTO embedded_subtitles (media_id, stream_index, language, title, codec, supported) VALUES (?, ?, ?, ?, ?, ?)`,
				mediaID, s.StreamIndex, s.Language, s.Title, s.Codec, supportedVal,
			); err != nil {
				slog.Warn("insert embedded_subtitles", "media_id", mediaID, "error", err)
			}
		}
	}

	if _, err := dbConn.ExecContext(ctx, `DELETE FROM embedded_audio_tracks WHERE media_id = ?`, mediaID); err != nil {
		slog.Warn("clear embedded_audio_tracks", "media_id", mediaID, "error", err)
	} else {
		for _, track := range audioTracks {
			if _, err := dbConn.ExecContext(ctx, `INSERT INTO embedded_audio_tracks (media_id, stream_index, language, title) VALUES (?, ?, ?, ?)`, mediaID, track.StreamIndex, track.Language, track.Title); err != nil {
				slog.Warn("insert embedded_audio_tracks", "media_id", mediaID, "error", err)
			}
		}
	}
}
