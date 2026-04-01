package db

import (
	"database/sql"
	"os"
	"strconv"
	"strings"
)

const (
	envRadarrBaseURL          = "PLUM_RADARR_BASE_URL"
	envRadarrAPIKey           = "PLUM_RADARR_API_KEY"
	envRadarrAPIKeyFile       = "PLUM_RADARR_API_KEY_FILE"
	envRadarrQualityProfileID = "PLUM_RADARR_QUALITY_PROFILE_ID"
	envRadarrRootFolderPath   = "PLUM_RADARR_ROOT_FOLDER_PATH"
	envRadarrSearchOnAdd      = "PLUM_RADARR_SEARCH_ON_ADD"

	envSonarrTVBaseURL          = "PLUM_SONARR_TV_BASE_URL"
	envSonarrTVAPIKey           = "PLUM_SONARR_TV_API_KEY"
	envSonarrTVAPIKeyFile       = "PLUM_SONARR_TV_API_KEY_FILE"
	envSonarrTVQualityProfileID = "PLUM_SONARR_TV_QUALITY_PROFILE_ID"
	envSonarrTVRootFolderPath   = "PLUM_SONARR_TV_ROOT_FOLDER_PATH"
	envSonarrTVSearchOnAdd      = "PLUM_SONARR_TV_SEARCH_ON_ADD"
)

func LoadMediaStackSettingsFromEnv(getEnv func(string) string) MediaStackSettings {
	return NormalizeMediaStackSettings(MediaStackSettings{
		Radarr: loadMediaStackServiceSettingsFromEnv(
			getEnv,
			envRadarrBaseURL,
			envRadarrAPIKey,
			envRadarrAPIKeyFile,
			envRadarrQualityProfileID,
			envRadarrRootFolderPath,
			envRadarrSearchOnAdd,
		),
		SonarrTV: loadMediaStackServiceSettingsFromEnv(
			getEnv,
			envSonarrTVBaseURL,
			envSonarrTVAPIKey,
			envSonarrTVAPIKeyFile,
			envSonarrTVQualityProfileID,
			envSonarrTVRootFolderPath,
			envSonarrTVSearchOnAdd,
		),
	})
}

func GetEffectiveMediaStackSettings(dbConn *sql.DB) (MediaStackSettings, error) {
	settings, err := GetMediaStackSettings(dbConn)
	if err != nil {
		return MediaStackSettings{}, err
	}
	return MergeMediaStackSettings(settings, LoadMediaStackSettingsFromEnv(os.Getenv)), nil
}

func MergeMediaStackSettings(primary MediaStackSettings, fallback MediaStackSettings) MediaStackSettings {
	return NormalizeMediaStackSettings(MediaStackSettings{
		Radarr:   mergeMediaStackServiceSettings(primary.Radarr, fallback.Radarr),
		SonarrTV: mergeMediaStackServiceSettings(primary.SonarrTV, fallback.SonarrTV),
	})
}

func mergeMediaStackServiceSettings(
	primary MediaStackServiceSettings,
	fallback MediaStackServiceSettings,
) MediaStackServiceSettings {
	primary = normalizeMediaStackServiceSettings(primary)
	fallback = normalizeMediaStackServiceSettings(fallback)
	if mediaStackServiceHasConfiguredFields(primary) {
		return primary
	}
	return fallback
}

func mediaStackServiceHasConfiguredFields(settings MediaStackServiceSettings) bool {
	settings = normalizeMediaStackServiceSettings(settings)
	return settings.BaseURL != "" ||
		settings.APIKey != "" ||
		settings.RootFolderPath != "" ||
		settings.QualityProfileID > 0
}

func loadMediaStackServiceSettingsFromEnv(
	getEnv func(string) string,
	baseURLKey string,
	apiKeyKey string,
	apiKeyFileKey string,
	qualityProfileKey string,
	rootFolderKey string,
	searchOnAddKey string,
) MediaStackServiceSettings {
	settings := DefaultMediaStackServiceSettings()
	settings.BaseURL = strings.TrimSpace(getEnv(baseURLKey))
	settings.APIKey = readEnvValueOrFile(getEnv, apiKeyKey, apiKeyFileKey)
	settings.RootFolderPath = strings.TrimSpace(getEnv(rootFolderKey))
	settings.QualityProfileID = readEnvInt(getEnv, qualityProfileKey)
	if value, ok := readEnvBool(getEnv, searchOnAddKey); ok {
		settings.SearchOnAdd = value
	}
	return settings
}

func readEnvValueOrFile(getEnv func(string) string, valueKey string, fileKey string) string {
	if value := strings.TrimSpace(getEnv(valueKey)); value != "" {
		return value
	}
	filePath := strings.TrimSpace(getEnv(fileKey))
	if filePath == "" {
		return ""
	}
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func readEnvInt(getEnv func(string) string, key string) int {
	value := strings.TrimSpace(getEnv(key))
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}

func readEnvBool(getEnv func(string) string, key string) (bool, bool) {
	value := strings.TrimSpace(getEnv(key))
	if value == "" {
		return false, false
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, false
	}
	return parsed, true
}
