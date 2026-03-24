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
	for _, name := range genres {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		slug := genreSlug(name)
		if slug == "" {
			continue
		}
		now := time.Now().UTC().Format(time.RFC3339)
		if _, err := tx.ExecContext(ctx, `INSERT INTO metadata_genres (name, slug, created_at, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(slug) DO UPDATE SET
	name = excluded.name,
	updated_at = excluded.updated_at`, name, slug, now, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO title_genres (title_kind, title_id, genre_slug) VALUES (?, ?, ?)`, titleKind, titleID, slug); err != nil {
			return err
		}
	}
	return nil
}

func replaceTitleCastTx(ctx context.Context, tx *sql.Tx, titleKind string, titleID int, cast []CastCredit) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM title_cast WHERE title_kind = ? AND title_id = ?`, titleKind, titleID); err != nil {
		return err
	}
	for _, member := range cast {
		member.Name = strings.TrimSpace(member.Name)
		if member.Name == "" {
			continue
		}
		now := time.Now().UTC().Format(time.RFC3339)
		nameKey := personNameKey(member.Name)
		if nameKey == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO metadata_people (
name, name_key, provider, provider_id, profile_path, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(name_key) DO UPDATE SET
	name = excluded.name,
	provider = CASE WHEN excluded.provider != '' THEN excluded.provider ELSE metadata_people.provider END,
	provider_id = CASE WHEN excluded.provider_id != '' THEN excluded.provider_id ELSE metadata_people.provider_id END,
	profile_path = CASE WHEN excluded.profile_path != '' THEN excluded.profile_path ELSE metadata_people.profile_path END,
	updated_at = excluded.updated_at`,
			member.Name,
			nameKey,
			member.Provider,
			member.ProviderID,
			member.ProfilePath,
			now,
			now,
		); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO title_cast (
title_kind, title_id, person_name_key, character_name, billing_order
) VALUES (?, ?, ?, ?, ?)`,
			titleKind,
			titleID,
			nameKey,
			nullStr(strings.TrimSpace(member.Character)),
			member.Order,
		); err != nil {
			return err
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
	var out []TitleCastMember
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
