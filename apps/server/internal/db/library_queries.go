package db

import (
	"database/sql"
	"sort"
	"strings"
)

func GetAllMediaForUser(db *sql.DB, userID int) ([]MediaItem, error) {
	items, err := queryAllMediaByKind(db, userID, "")
	if err != nil {
		return nil, err
	}
	items, err = attachMediaFilesBatch(db, items)
	if err != nil {
		return nil, err
	}
	return attachSubtitlesBatch(db, items)
}

func UserHasLibraryAccess(db *sql.DB, userID, libraryID int) (bool, error) {
	var ownerID int
	err := db.QueryRow(`SELECT user_id FROM libraries WHERE id = ?`, libraryID).Scan(&ownerID)
	if err != nil {
		return false, err
	}
	return ownerID == userID, nil
}

// queryAllMediaByKind returns media from category tables joined with media_global.
// If kind is "", queries all four categories and merges; otherwise only that kind.
// If userID > 0, filters media to only those in libraries owned by that user.
func queryAllMediaByKind(db *sql.DB, userID int, kind string) ([]MediaItem, error) {
	kinds := []string{"movie", "tv", "anime", "music"}
	if kind != "" {
		kinds = []string{kind}
	}
	var items []MediaItem
	for _, k := range kinds {
		table := mediaTableForKind(k)
		var q string
		var args []interface{}
		args = append(args, k)

		switch table {
		case "music_tracks":
			q = `SELECT g.id, m.library_id, m.title, m.path, m.duration, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.missing_since, ''), m.match_status, m.artist, m.album, m.album_artist, m.poster_path, COALESCE(m.disc_number, 0), COALESCE(m.track_number, 0), COALESCE(m.release_year, 0) FROM music_tracks m JOIN media_global g ON g.kind = ? AND g.ref_id = m.id `
		case "tv_episodes", "anime_episodes":
			q = `SELECT g.id, m.library_id, m.title, m.path, m.duration, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.missing_since, ''), m.match_status, m.tmdb_id, m.tvdb_id, m.overview, m.poster_path, m.backdrop_path, m.release_date, m.vote_average, m.imdb_id, m.imdb_rating, COALESCE(m.season, 0), COALESCE(m.episode, 0), COALESCE(m.metadata_review_needed, 0), COALESCE(m.metadata_confirmed, 0), m.thumbnail_path FROM ` + table + ` m JOIN media_global g ON g.kind = ? AND g.ref_id = m.id `
		default:
			q = `SELECT g.id, m.library_id, m.title, m.path, m.duration, COALESCE(m.file_size_bytes, 0), COALESCE(m.file_mod_time, ''), COALESCE(m.file_hash, ''), COALESCE(m.file_hash_kind, ''), COALESCE(m.missing_since, ''), m.match_status, m.tmdb_id, m.tvdb_id, m.overview, m.poster_path, m.backdrop_path, m.release_date, m.vote_average, m.imdb_id, m.imdb_rating FROM ` + table + ` m JOIN media_global g ON g.kind = ? AND g.ref_id = m.id `
		}

		if userID > 0 {
			q += ` JOIN libraries l ON l.id = m.library_id AND l.user_id = ? `
			args = append(args, userID)
		}

		q += ` ORDER BY g.id`

		rows, err := db.Query(q, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var m MediaItem
			m.Type = k
			var overview, posterPath, backdropPath, releaseDate, thumbnailPath sql.NullString
			var matchStatus, imdbID sql.NullString
			var voteAvg, imdbRating sql.NullFloat64
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
				err = rows.Scan(&m.ID, &m.LibraryID, &m.Title, &m.Path, &m.Duration, &m.FileSizeBytes, &m.FileModTime, &m.FileHash, &m.FileHashKind, &m.MissingSince, &matchStatus, &tmdbID, &tvdbID, &overview, &posterPath, &backdropPath, &releaseDate, &voteAvg, &imdbID, &imdbRating, &m.Season, &m.Episode, &metadataReviewNeeded, &metadataConfirmed, &thumbnailPath)
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
				rows.Close()
				return nil, err
			}
			items = append(items, m)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return attachDuplicateState(db, items)
}

func mediaTableForKind(kind string) string {
	return MediaTableForKind(kind)
}

// MediaTableForKind returns the category table name for a library kind (exported for use by http handlers).
func MediaTableForKind(kind string) string {
	switch kind {
	case "movie":
		return "movies"
	case "tv":
		return "tv_episodes"
	case "anime":
		return "anime_episodes"
	case "music":
		return "music_tracks"
	default:
		return "movies"
	}
}

// duplicateHashQueryChunk limits bound variables per query (below SQLite's default SQLITE_MAX_VARIABLE_NUMBER).
const duplicateHashQueryChunk = 400

type duplicateStateGroupKey struct {
	libraryID int
	kind      string
}

func mediaFilesTableExists(db *sql.DB) (bool, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(1) FROM sqlite_master WHERE type = 'table' AND name = 'media_files'`).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func queryDuplicateCountsMediaFiles(db *sql.DB, kind string, libraryID int, hashes []string) (map[string]int, error) {
	if len(hashes) == 0 {
		return map[string]int{}, nil
	}
	table := mediaTableForKind(kind)
	ph := make([]string, len(hashes))
	args := make([]any, 0, 2+len(hashes))
	args = append(args, kind, libraryID)
	for i, h := range hashes {
		ph[i] = "?"
		args = append(args, h)
	}
	q := `SELECT COALESCE(mf.file_hash, ''), COUNT(1)
FROM media_files mf
JOIN media_global g ON g.id = mf.media_id
JOIN ` + table + ` m ON m.id = g.ref_id
WHERE g.kind = ?
AND m.library_id = ?
AND COALESCE(mf.file_hash, '') IN (` + strings.Join(ph, ",") + `)
AND COALESCE(mf.missing_since, '') = ''
GROUP BY COALESCE(mf.file_hash, '')`
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int, len(hashes))
	for rows.Next() {
		var hash string
		var cnt int
		if err := rows.Scan(&hash, &cnt); err != nil {
			return nil, err
		}
		out[hash] = cnt
	}
	return out, rows.Err()
}

func queryDuplicateCountsLegacy(db *sql.DB, table string, libraryID int, hashes []string) (map[string]int, error) {
	if len(hashes) == 0 {
		return map[string]int{}, nil
	}
	ph := make([]string, len(hashes))
	args := make([]any, 0, 1+len(hashes))
	args = append(args, libraryID)
	for i, h := range hashes {
		ph[i] = "?"
		args = append(args, h)
	}
	q := `SELECT COALESCE(file_hash, ''), COUNT(1) FROM ` + table + `
WHERE library_id = ?
AND COALESCE(file_hash, '') IN (` + strings.Join(ph, ",") + `)
AND COALESCE(missing_since, '') = ''
GROUP BY COALESCE(file_hash, '')`
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int, len(hashes))
	for rows.Next() {
		var hash string
		var cnt int
		if err := rows.Scan(&hash, &cnt); err != nil {
			return nil, err
		}
		out[hash] = cnt
	}
	return out, rows.Err()
}

func duplicateCountsForHashes(db *sql.DB, useMediaFiles bool, kind string, libraryID int, hashes []string) (map[string]int, error) {
	out := make(map[string]int, len(hashes))
	table := mediaTableForKind(kind)
	for start := 0; start < len(hashes); start += duplicateHashQueryChunk {
		end := start + duplicateHashQueryChunk
		if end > len(hashes) {
			end = len(hashes)
		}
		chunk := hashes[start:end]
		var part map[string]int
		var err error
		if useMediaFiles {
			part, err = queryDuplicateCountsMediaFiles(db, kind, libraryID, chunk)
		} else {
			part, err = queryDuplicateCountsLegacy(db, table, libraryID, chunk)
		}
		if err != nil {
			return nil, err
		}
		for h, c := range part {
			out[h] = c
		}
	}
	return out, nil
}

func attachDuplicateState(db *sql.DB, items []MediaItem) ([]MediaItem, error) {
	if len(items) == 0 {
		return items, nil
	}
	useMediaFiles, err := mediaFilesTableExists(db)
	if err != nil {
		return nil, err
	}
	groups := make(map[duplicateStateGroupKey]map[string]struct{})
	for i := range items {
		it := &items[i]
		if it.LibraryID <= 0 || it.Missing || it.FileHash == "" {
			it.Duplicate = false
			it.DuplicateCount = 0
			continue
		}
		gk := duplicateStateGroupKey{libraryID: it.LibraryID, kind: it.Type}
		if groups[gk] == nil {
			groups[gk] = make(map[string]struct{})
		}
		groups[gk][it.FileHash] = struct{}{}
	}
	countsByGroup := make(map[duplicateStateGroupKey]map[string]int, len(groups))
	for gk, hashSet := range groups {
		hashes := make([]string, 0, len(hashSet))
		for h := range hashSet {
			hashes = append(hashes, h)
		}
		sort.Strings(hashes)
		m, err := duplicateCountsForHashes(db, useMediaFiles, gk.kind, gk.libraryID, hashes)
		if err != nil {
			return nil, err
		}
		countsByGroup[gk] = m
	}
	for i := range items {
		it := &items[i]
		if it.LibraryID <= 0 || it.Missing || it.FileHash == "" {
			continue
		}
		gk := duplicateStateGroupKey{libraryID: it.LibraryID, kind: it.Type}
		c := countsByGroup[gk][it.FileHash]
		if c > 1 {
			it.Duplicate = true
			it.DuplicateCount = c
		} else {
			it.Duplicate = false
			it.DuplicateCount = 0
		}
	}
	return items, nil
}
