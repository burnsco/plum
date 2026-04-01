package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMediaStackSettingsFromEnvSupportsInlineValues(t *testing.T) {
	t.Setenv(envRadarrBaseURL, "http://127.0.0.1:7878")
	t.Setenv(envRadarrAPIKey, "radarr-key")
	t.Setenv(envRadarrQualityProfileID, "4")
	t.Setenv(envRadarrRootFolderPath, "/storage/media/movies")
	t.Setenv(envSonarrTVBaseURL, "http://127.0.0.1:8989")
	t.Setenv(envSonarrTVAPIKey, "sonarr-key")
	t.Setenv(envSonarrTVQualityProfileID, "1")
	t.Setenv(envSonarrTVRootFolderPath, "/storage/media/tv")

	settings := LoadMediaStackSettingsFromEnv(getEnvFromOS)

	if settings.Radarr.BaseURL != "http://127.0.0.1:7878" || settings.Radarr.APIKey != "radarr-key" {
		t.Fatalf("radarr settings = %+v", settings.Radarr)
	}
	if settings.Radarr.QualityProfileID != 4 || settings.Radarr.RootFolderPath != "/storage/media/movies" {
		t.Fatalf("radarr settings = %+v", settings.Radarr)
	}
	if settings.SonarrTV.BaseURL != "http://127.0.0.1:8989" || settings.SonarrTV.APIKey != "sonarr-key" {
		t.Fatalf("sonarr settings = %+v", settings.SonarrTV)
	}
	if settings.SonarrTV.QualityProfileID != 1 || settings.SonarrTV.RootFolderPath != "/storage/media/tv" {
		t.Fatalf("sonarr settings = %+v", settings.SonarrTV)
	}
}

func TestLoadMediaStackSettingsFromEnvSupportsFileBackedAPIKeys(t *testing.T) {
	dir := t.TempDir()
	radarrKeyPath := filepath.Join(dir, "radarr.key")
	sonarrKeyPath := filepath.Join(dir, "sonarr.key")
	if err := osWriteFile(radarrKeyPath, []byte("radarr-file-key\n")); err != nil {
		t.Fatalf("write radarr key: %v", err)
	}
	if err := osWriteFile(sonarrKeyPath, []byte("sonarr-file-key\n")); err != nil {
		t.Fatalf("write sonarr key: %v", err)
	}
	t.Setenv(envRadarrAPIKeyFile, radarrKeyPath)
	t.Setenv(envSonarrTVAPIKeyFile, sonarrKeyPath)

	settings := LoadMediaStackSettingsFromEnv(getEnvFromOS)
	if settings.Radarr.APIKey != "radarr-file-key" {
		t.Fatalf("radarr api key = %q", settings.Radarr.APIKey)
	}
	if settings.SonarrTV.APIKey != "sonarr-file-key" {
		t.Fatalf("sonarr api key = %q", settings.SonarrTV.APIKey)
	}
}

func TestMergeMediaStackSettingsUsesEnvFallbackOnlyWhenDBIsEmpty(t *testing.T) {
	merged := MergeMediaStackSettings(
		DefaultMediaStackSettings(),
		MediaStackSettings{
			Radarr: MediaStackServiceSettings{
				BaseURL:          "http://127.0.0.1:7878",
				APIKey:           "radarr-key",
				QualityProfileID: 4,
				RootFolderPath:   "/storage/media/movies",
			},
		},
	)
	if merged.Radarr.BaseURL != "http://127.0.0.1:7878" || merged.Radarr.APIKey != "radarr-key" {
		t.Fatalf("merged settings = %+v", merged.Radarr)
	}

	merged = MergeMediaStackSettings(
		MediaStackSettings{
			Radarr: MediaStackServiceSettings{
				BaseURL:          "http://db-only:7878",
				APIKey:           "db-key",
				QualityProfileID: 8,
				RootFolderPath:   "/db/movies",
			},
		},
		MediaStackSettings{
			Radarr: MediaStackServiceSettings{
				BaseURL:          "http://env-only:7878",
				APIKey:           "env-key",
				QualityProfileID: 4,
				RootFolderPath:   "/env/movies",
			},
		},
	)
	if merged.Radarr.BaseURL != "http://db-only:7878" || merged.Radarr.APIKey != "db-key" {
		t.Fatalf("merged settings = %+v", merged.Radarr)
	}
}

func getEnvFromOS(key string) string {
	return os.Getenv(key)
}

func osWriteFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o600)
}
