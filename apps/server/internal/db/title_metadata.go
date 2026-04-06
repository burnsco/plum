package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type CastCredit struct {
	Name        string
	Character   string
	Order       int
	ProfilePath string
	Provider    string
	ProviderID  string
}

type TitleGenre struct {
	Name string `json:"name"`
}

type TitleCastMember struct {
	Name        string `json:"name"`
	Character   string `json:"character,omitempty"`
	Order       int    `json:"order,omitempty"`
	ProfilePath string `json:"profile_path,omitempty"`
}

func genreSlug(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "&", " and ")
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, name)
	name = strings.Trim(name, "-")
	name = strings.ReplaceAll(name, "--", "-")
	return name
}

func personNameKey(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(name)), " "))
}

func syncTitleMetadataTx(ctx context.Context, tx *sql.Tx, titleKind string, titleID int, canonical CanonicalMetadata) error {
	if titleID <= 0 {
		return nil
	}
	if err := replaceTitleGenresTx(ctx, tx, titleKind, titleID, canonical.Genres); err != nil {
		return err
	}
	return replaceTitleCastTx(ctx, tx, titleKind, titleID, canonical.Cast)
}

func SyncMovieTitleMetadata(db *sql.DB, movieRefID int, canonical CanonicalMetadata) error {
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := syncTitleMetadataTx(ctx, tx, "movie", movieRefID, canonical); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func SyncShowTitleMetadata(db *sql.DB, showID int, canonical CanonicalMetadata) error {
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := syncTitleMetadataTx(ctx, tx, "show", showID, canonical); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func replaceTitleGenresTx(ctx context.Context, tx *sql.Tx, titleKind string, titleID int, genres []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM title_genres WHERE title_kind = ? AND title_id = ?`, titleKind, titleID); err != nil {
		return err
	}

	type genreRow struct{ name, slug string }
	seen := make(map[string]bool, len(genres))
	rows := make([]genreRow, 0, len(genres))
	for _, name := range genres {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		slug := genreSlug(name)
		if slug == "" || seen[slug] {
			continue
		}
		seen[slug] = true
		rows = append(rows, genreRow{name, slug})
	}
	if len(rows) == 0 {
		return nil
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// Single upsert for all genres.
	{
		ph := make([]string, len(rows))
		args := make([]any, 0, len(rows)*4)
		for i, r := range rows {
			ph[i] = "(?,?,?,?)"
			args = append(args, r.name, r.slug, now, now)
		}
		q := `INSERT INTO metadata_genres (name, slug, created_at, updated_at) VALUES ` +
			strings.Join(ph, ",") +
			` ON CONFLICT(slug) DO UPDATE SET name = excluded.name, updated_at = excluded.updated_at`
		if _, err := tx.ExecContext(ctx, q, args...); err != nil {
			return err
		}
	}

	// Single insert for all title_genre links.
	{
		ph := make([]string, len(rows))
		args := make([]any, 0, len(rows)*3)
		for i, r := range rows {
			ph[i] = "(?,?,?)"
			args = append(args, titleKind, titleID, r.slug)
		}
		q := `INSERT OR IGNORE INTO title_genres (title_kind, title_id, genre_slug) VALUES ` + strings.Join(ph, ",")
		if _, err := tx.ExecContext(ctx, q, args...); err != nil {
			return err
		}
	}

	return nil
}

func replaceTitleCastTx(ctx context.Context, tx *sql.Tx, titleKind string, titleID int, cast []CastCredit) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM title_cast WHERE title_kind = ? AND title_id = ?`, titleKind, titleID); err != nil {
		return err
	}

	type castRow struct {
		name, nameKey, provider, providerID, profilePath, character string
		order                                                        int
	}
	rows := make([]castRow, 0, len(cast))
	for _, m := range cast {
		m.Name = strings.TrimSpace(m.Name)
		if m.Name == "" {
			continue
		}
		nameKey := personNameKey(m.Name)
		if nameKey == "" {
			continue
		}
		rows = append(rows, castRow{
			name: m.Name, nameKey: nameKey,
			provider: m.Provider, providerID: m.ProviderID, profilePath: m.ProfilePath,
			character: strings.TrimSpace(m.Character), order: m.Order,
		})
	}
	if len(rows) == 0 {
		return nil
	}

	now := time.Now().UTC().Format(time.RFC3339)

	const chunkSize = 200
	for i := 0; i < len(rows); i += chunkSize {
		chunk := rows[i:min(i+chunkSize, len(rows))]

		// Batch upsert metadata_people.
		{
			ph := make([]string, len(chunk))
			args := make([]any, 0, len(chunk)*7)
			for j, r := range chunk {
				ph[j] = "(?,?,?,?,?,?,?)"
				args = append(args, r.name, r.nameKey, r.provider, r.providerID, r.profilePath, now, now)
			}
			q := `INSERT INTO metadata_people (name, name_key, provider, provider_id, profile_path, created_at, updated_at) VALUES ` +
				strings.Join(ph, ",") +
				` ON CONFLICT(name_key) DO UPDATE SET
	name = excluded.name,
	provider = CASE WHEN excluded.provider != '' THEN excluded.provider ELSE metadata_people.provider END,
	provider_id = CASE WHEN excluded.provider_id != '' THEN excluded.provider_id ELSE metadata_people.provider_id END,
	profile_path = CASE WHEN excluded.profile_path != '' THEN excluded.profile_path ELSE metadata_people.profile_path END,
	updated_at = excluded.updated_at`
			if _, err := tx.ExecContext(ctx, q, args...); err != nil {
				return err
			}
		}

		// Batch insert title_cast links.
		{
			ph := make([]string, len(chunk))
			args := make([]any, 0, len(chunk)*5)
			for j, r := range chunk {
				ph[j] = "(?,?,?,?,?)"
				args = append(args, titleKind, titleID, r.nameKey, nullStr(r.character), r.order)
			}
			q := `INSERT INTO title_cast (title_kind, title_id, person_name_key, character_name, billing_order) VALUES ` +
				strings.Join(ph, ",")
			if _, err := tx.ExecContext(ctx, q, args...); err != nil {
				return err
			}
		}
	}

	return nil
}

func loadTitleGenres(db *sql.DB, titleKind string, titleID int) ([]TitleGenre, error) {
	rows, err := db.Query(`SELECT g.name
FROM title_genres tg
JOIN metadata_genres g ON g.slug = tg.genre_slug
WHERE tg.title_kind = ? AND tg.title_id = ?
ORDER BY g.name`, titleKind, titleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TitleGenre
	for rows.Next() {
		var genre TitleGenre
		if err := rows.Scan(&genre.Name); err != nil {
			return nil, err
		}
		out = append(out, genre)
	}
	return out, rows.Err()
}

func loadTitleCast(db *sql.DB, titleKind string, titleID int) ([]TitleCastMember, error) {
	rows, err := db.Query(`SELECT p.name, COALESCE(tc.character_name, ''), COALESCE(tc.billing_order, 0), COALESCE(p.profile_path, '')
FROM title_cast tc
JOIN metadata_people p ON p.name_key = tc.person_name_key
WHERE tc.title_kind = ? AND tc.title_id = ?
ORDER BY tc.billing_order ASC, p.name ASC`, titleKind, titleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	// Non-nil so JSON encodes [] not null (clients expect an array).
	out := make([]TitleCastMember, 0)
	for rows.Next() {
		var castMember TitleCastMember
		if err := rows.Scan(&castMember.Name, &castMember.Character, &castMember.Order, &castMember.ProfilePath); err != nil {
			return nil, err
		}
		out = append(out, castMember)
	}
	return out, rows.Err()
}

func titleSubtitle(year int, extra string) string {
	parts := make([]string, 0, 2)
	if year > 0 {
		parts = append(parts, fmt.Sprintf("%d", year))
	}
	if strings.TrimSpace(extra) != "" {
		parts = append(parts, strings.TrimSpace(extra))
	}
	return strings.Join(parts, " • ")
}
