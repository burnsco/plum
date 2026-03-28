package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

const metadataArtworkSettingsKey = "metadata_artwork"

type ShowMetadataArtworkFetchers struct {
	Fanart bool `json:"fanart"`
	TMDB   bool `json:"tmdb"`
	TVDB   bool `json:"tvdb"`
}

type EpisodeMetadataArtworkFetchers struct {
	TMDB bool `json:"tmdb"`
	TVDB bool `json:"tvdb"`
	OMDB bool `json:"omdb"`
}

type MetadataArtworkSettings struct {
	Movies   ShowMetadataArtworkFetchers    `json:"movies"`
	Shows    ShowMetadataArtworkFetchers    `json:"shows"`
	Seasons  ShowMetadataArtworkFetchers    `json:"seasons"`
	Episodes EpisodeMetadataArtworkFetchers `json:"episodes"`
}

func DefaultMetadataArtworkSettings() MetadataArtworkSettings {
	return MetadataArtworkSettings{
		Movies: ShowMetadataArtworkFetchers{
			Fanart: true,
			TMDB:   true,
			TVDB:   true,
		},
		Shows: ShowMetadataArtworkFetchers{
			Fanart: true,
			TMDB:   true,
			TVDB:   true,
		},
		Seasons: ShowMetadataArtworkFetchers{
			Fanart: true,
			TMDB:   true,
			TVDB:   true,
		},
		Episodes: EpisodeMetadataArtworkFetchers{
			TMDB: true,
			TVDB: true,
			OMDB: true,
		},
	}
}

func NormalizeMetadataArtworkSettings(settings MetadataArtworkSettings) MetadataArtworkSettings {
	return settings
}

func ensureMetadataArtworkSettingsDefaults(db *sql.DB) error {
	settings := DefaultMetadataArtworkSettings()
	raw, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO app_settings (key, value, updated_at)
VALUES (?, ?, ?)
ON CONFLICT(key) DO NOTHING`,
		metadataArtworkSettingsKey,
		string(raw),
		time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

func GetMetadataArtworkSettings(dbConn *sql.DB) (MetadataArtworkSettings, error) {
	var raw string
	err := dbConn.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, metadataArtworkSettingsKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return DefaultMetadataArtworkSettings(), nil
	}
	if err != nil {
		return MetadataArtworkSettings{}, err
	}
	settings := DefaultMetadataArtworkSettings()
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return MetadataArtworkSettings{}, err
	}
	return NormalizeMetadataArtworkSettings(settings), nil
}

func SaveMetadataArtworkSettings(dbConn *sql.DB, settings MetadataArtworkSettings) (MetadataArtworkSettings, error) {
	settings = NormalizeMetadataArtworkSettings(settings)
	raw, err := json.Marshal(settings)
	if err != nil {
		return MetadataArtworkSettings{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := dbConn.Exec(
		`INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		metadataArtworkSettingsKey,
		string(raw),
		now,
	); err != nil {
		return MetadataArtworkSettings{}, err
	}
	return settings, nil
}
