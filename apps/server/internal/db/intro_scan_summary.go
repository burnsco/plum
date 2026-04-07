package db

import (
	"database/sql"
)

// introDetectedCondition matches rows with a usable skip target: chapter intros set both
// start/end; silence fallback sets intro_end_sec only (start may be NULL).
const introDetectedCondition = `mf.intro_end_sec IS NOT NULL AND mf.intro_end_sec > 0`

// IntroScanLibrarySummary reports per-library intro detection coverage.
type IntroScanLibrarySummary struct {
	LibraryID     int    `json:"library_id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	IntroSkipMode string `json:"intro_skip_mode"`
	TotalEpisodes int    `json:"total_episodes"`
	WithIntro     int    `json:"with_intro"`
}

// ListIntroScanSummaries returns intro detection stats for every non-music library owned by userID.
func ListIntroScanSummaries(dbConn *sql.DB, userID int) ([]IntroScanLibrarySummary, error) {
	// One query per episode-table type, then movies. The three queries are unioned
	// so we hit the DB once.
	const q = `
SELECT l.id, l.name, l.type, COALESCE(l.intro_skip_mode, 'manual'),
       COUNT(DISTINCT e.id),
       COUNT(DISTINCT CASE WHEN ` + introDetectedCondition + ` THEN e.id END)
FROM libraries l
JOIN tv_episodes e ON e.library_id = l.id AND e.missing_since IS NULL
JOIN media_global mg ON mg.kind = 'tv' AND mg.ref_id = e.id
LEFT JOIN media_files mf ON mf.media_id = mg.id AND mf.is_primary = 1
WHERE l.user_id = ? AND l.type = 'tv'
GROUP BY l.id

UNION ALL

SELECT l.id, l.name, l.type, COALESCE(l.intro_skip_mode, 'manual'),
       COUNT(DISTINCT e.id),
       COUNT(DISTINCT CASE WHEN ` + introDetectedCondition + ` THEN e.id END)
FROM libraries l
JOIN anime_episodes e ON e.library_id = l.id AND e.missing_since IS NULL
JOIN media_global mg ON mg.kind = 'anime' AND mg.ref_id = e.id
LEFT JOIN media_files mf ON mf.media_id = mg.id AND mf.is_primary = 1
WHERE l.user_id = ? AND l.type = 'anime'
GROUP BY l.id

UNION ALL

SELECT l.id, l.name, l.type, COALESCE(l.intro_skip_mode, 'manual'),
       COUNT(DISTINCT m.id),
       COUNT(DISTINCT CASE WHEN ` + introDetectedCondition + ` THEN m.id END)
FROM libraries l
JOIN movies m ON m.library_id = l.id AND m.missing_since IS NULL
JOIN media_global mg ON mg.kind = 'movie' AND mg.ref_id = m.id
LEFT JOIN media_files mf ON mf.media_id = mg.id AND mf.is_primary = 1
WHERE l.user_id = ? AND l.type = 'movie'
GROUP BY l.id

ORDER BY 1
`
	rows, err := dbConn.Query(q, userID, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []IntroScanLibrarySummary
	for rows.Next() {
		var s IntroScanLibrarySummary
		if err := rows.Scan(&s.LibraryID, &s.Name, &s.Type, &s.IntroSkipMode, &s.TotalEpisodes, &s.WithIntro); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// IntroScanShowSummary reports per-show intro detection coverage within a library.
type IntroScanShowSummary struct {
	ShowKey       string `json:"show_key"`
	ShowTitle     string `json:"show_title"`
	TotalEpisodes int    `json:"total_episodes"`
	WithIntro     int    `json:"with_intro"`
}

// ListIntroScanShowSummaries returns intro stats grouped by show for the given library.
func ListIntroScanShowSummaries(dbConn *sql.DB, libraryID int) ([]IntroScanShowSummary, error) {
	// The shows table is shared between tv and anime. We determine the episode
	// table from the show's kind column and union both.
	const q = `
SELECT s.title_key, s.title,
       COUNT(DISTINCT e.id),
       COUNT(DISTINCT CASE WHEN ` + introDetectedCondition + ` THEN e.id END)
FROM shows s
JOIN tv_episodes e ON e.show_id = s.id AND e.missing_since IS NULL
JOIN media_global mg ON mg.kind = 'tv' AND mg.ref_id = e.id
LEFT JOIN media_files mf ON mf.media_id = mg.id AND mf.is_primary = 1
WHERE s.library_id = ? AND s.kind = 'tv'
GROUP BY s.id

UNION ALL

SELECT s.title_key, s.title,
       COUNT(DISTINCT e.id),
       COUNT(DISTINCT CASE WHEN ` + introDetectedCondition + ` THEN e.id END)
FROM shows s
JOIN anime_episodes e ON e.show_id = s.id AND e.missing_since IS NULL
JOIN media_global mg ON mg.kind = 'anime' AND mg.ref_id = e.id
LEFT JOIN media_files mf ON mf.media_id = mg.id AND mf.is_primary = 1
WHERE s.library_id = ? AND s.kind = 'anime'
GROUP BY s.id

ORDER BY 2
`
	rows, err := dbConn.Query(q, libraryID, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []IntroScanShowSummary
	for rows.Next() {
		var s IntroScanShowSummary
		if err := rows.Scan(&s.ShowKey, &s.ShowTitle, &s.TotalEpisodes, &s.WithIntro); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
