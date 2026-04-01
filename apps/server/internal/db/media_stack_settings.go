package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const appSettingsKeyMediaStack = "media_stack"

var (
	ErrMediaStackServiceIncomplete     = errors.New("media stack service requires both base url and api key")
	ErrMediaStackRootFolderRequired    = errors.New("media stack root folder is required when a service is configured")
	ErrMediaStackQualityProfileInvalid = errors.New("media stack quality profile is required when a service is configured")
)

type MediaStackServiceSettings struct {
	BaseURL          string `json:"baseUrl"`
	APIKey           string `json:"apiKey"`
	QualityProfileID int    `json:"qualityProfileId"`
	RootFolderPath   string `json:"rootFolderPath"`
	SearchOnAdd      bool   `json:"searchOnAdd"`
}

type MediaStackSettings struct {
	Radarr   MediaStackServiceSettings `json:"radarr"`
	SonarrTV MediaStackServiceSettings `json:"sonarrTv"`
}

func DefaultMediaStackServiceSettings() MediaStackServiceSettings {
	return MediaStackServiceSettings{
		SearchOnAdd: true,
	}
}

func DefaultMediaStackSettings() MediaStackSettings {
	return MediaStackSettings{
		Radarr:   DefaultMediaStackServiceSettings(),
		SonarrTV: DefaultMediaStackServiceSettings(),
	}
}

func normalizeMediaStackServiceSettings(settings MediaStackServiceSettings) MediaStackServiceSettings {
	settings.BaseURL = strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/")
	settings.APIKey = strings.TrimSpace(settings.APIKey)
	settings.RootFolderPath = strings.TrimSpace(settings.RootFolderPath)
	settings.SearchOnAdd = true
	return settings
}

func NormalizeMediaStackSettings(settings MediaStackSettings) MediaStackSettings {
	settings.Radarr = normalizeMediaStackServiceSettings(settings.Radarr)
	settings.SonarrTV = normalizeMediaStackServiceSettings(settings.SonarrTV)
	return settings
}

func validateMediaStackServiceSettings(settings MediaStackServiceSettings) error {
	configuredFields := 0
	if settings.BaseURL != "" {
		configuredFields++
	}
	if settings.APIKey != "" {
		configuredFields++
	}
	if settings.RootFolderPath != "" {
		configuredFields++
	}
	if settings.QualityProfileID > 0 {
		configuredFields++
	}
	if configuredFields == 0 {
		return nil
	}
	if settings.BaseURL == "" || settings.APIKey == "" {
		return ErrMediaStackServiceIncomplete
	}
	if settings.RootFolderPath == "" {
		return ErrMediaStackRootFolderRequired
	}
	if settings.QualityProfileID <= 0 {
		return ErrMediaStackQualityProfileInvalid
	}
	return nil
}

func ValidateMediaStackSettings(settings MediaStackSettings) error {
	settings = NormalizeMediaStackSettings(settings)
	if err := validateMediaStackServiceSettings(settings.Radarr); err != nil {
		return err
	}
	if err := validateMediaStackServiceSettings(settings.SonarrTV); err != nil {
		return err
	}
	return nil
}

func GetMediaStackSettings(dbConn *sql.DB) (MediaStackSettings, error) {
	var raw string
	err := dbConn.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, appSettingsKeyMediaStack).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return DefaultMediaStackSettings(), nil
	}
	if err != nil {
		return MediaStackSettings{}, err
	}

	settings := DefaultMediaStackSettings()
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return MediaStackSettings{}, err
	}
	return NormalizeMediaStackSettings(settings), nil
}

func SaveMediaStackSettings(dbConn *sql.DB, settings MediaStackSettings) (MediaStackSettings, error) {
	settings = NormalizeMediaStackSettings(settings)
	if err := ValidateMediaStackSettings(settings); err != nil {
		return MediaStackSettings{}, err
	}

	raw, err := json.Marshal(settings)
	if err != nil {
		return MediaStackSettings{}, err
	}

	now := time.Now().UTC()
	if _, err := dbConn.Exec(
		`INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		appSettingsKeyMediaStack,
		string(raw),
		now,
	); err != nil {
		return MediaStackSettings{}, err
	}

	return settings, nil
}
