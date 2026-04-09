package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"plum/internal/db"
	"plum/internal/dotenv"
	"plum/internal/metadata"
)

const (
	envTMDBAPIKey             = "TMDB_API_KEY"
	envTVDBAPIKey             = "TVDB_API_KEY"
	envOMDBAPIKey             = "OMDB_API_KEY"
	envFanartAPIKey           = "FANART_API_KEY"
	envMusicBrainzContactURL  = "MUSICBRAINZ_CONTACT_URL"
	envPLUMAddr               = "PLUM_ADDR"
	envPLUMDatabaseURL        = "PLUM_DATABASE_URL"
	envPLUMDatabaseLegacyPath = "PLUM_DB_PATH"
)

var managedEnvKeys = map[string]struct{}{
	envTMDBAPIKey:             {},
	envTVDBAPIKey:             {},
	envOMDBAPIKey:             {},
	envFanartAPIKey:           {},
	envMusicBrainzContactURL:  {},
	envPLUMAddr:               {},
	envPLUMDatabaseURL:        {},
	envPLUMDatabaseLegacyPath: {},
}

// ServerEnvSettingsHandler reads and updates the on-disk .env file (same discovery rules as startup)
// and applies metadata API key changes to the running process without restart.
type ServerEnvSettingsHandler struct {
	DB       *sql.DB
	Pipeline *metadata.Pipeline
}

type serverEnvSecretsPresent struct {
	TMDBAPIKey   bool `json:"tmdb_api_key"`
	TVDBAPIKey   bool `json:"tvdb_api_key"`
	OMDBAPIKey   bool `json:"omdb_api_key"`
	FanartAPIKey bool `json:"fanart_api_key"`
}

type serverEnvSettingsResponse struct {
	EnvFilePath           string                  `json:"env_file_path"`
	EnvFileExisted        bool                    `json:"env_file_existed"`
	EnvFileWritable       bool                    `json:"env_file_writable"`
	PLUMAddr              string                  `json:"plum_addr"`
	PLUMDatabaseURL       string                  `json:"plum_database_url"`
	MusicBrainzContactURL string                  `json:"musicbrainz_contact_url"`
	SecretsPresent        serverEnvSecretsPresent `json:"secrets_present"`
	RestartRecommended    bool                    `json:"restart_recommended"`
	Help                  string                  `json:"help"`
}

type serverEnvUpdateRequest struct {
	PLUMAddr              *string `json:"plum_addr"`
	PLUMDatabaseURL       *string `json:"plum_database_url"`
	MusicBrainzContactURL *string `json:"musicbrainz_contact_url"`

	TMDBAPIKey   *string `json:"tmdb_api_key"`
	TVDBAPIKey   *string `json:"tvdb_api_key"`
	OMDBAPIKey   *string `json:"omdb_api_key"`
	FanartAPIKey *string `json:"fanart_api_key"`

	ClearTMDBAPIKey   *bool `json:"tmdb_api_key_clear"`
	ClearTVDBAPIKey   *bool `json:"tvdb_api_key_clear"`
	ClearOMDBAPIKey   *bool `json:"omdb_api_key_clear"`
	ClearFanartAPIKey *bool `json:"fanart_api_key_clear"`
}

func effectiveDatabaseURLWithOverrides(overrides map[string]string) string {
	if v := effectiveEnvString(overrides, envPLUMDatabaseURL); v != "" {
		return v
	}
	return effectiveEnvString(overrides, envPLUMDatabaseLegacyPath)
}

func effectiveEnvString(overrides map[string]string, envName string) string {
	if overrides != nil {
		if v, ok := overrides[envName]; ok {
			return strings.TrimSpace(v)
		}
	}
	return strings.TrimSpace(os.Getenv(envName))
}

func effectiveSecretPresent(overrides map[string]string, envName string) bool {
	return effectiveEnvString(overrides, envName) != ""
}

func (h *ServerEnvSettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	overrides, err := h.loadServerEnvOverrides()
	if err != nil {
		http.Error(w, "load server env overrides: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writePath, err := dotenv.ResolveWritePath()
	if err != nil {
		http.Error(w, "resolve .env path: "+err.Error(), http.StatusInternalServerError)
		return
	}
	existing, hadFile := dotenv.ResolveExistingPath()
	envPath := writePath
	if hadFile {
		envPath = existing
	}

	resp := serverEnvSettingsResponse{
		EnvFilePath:           writePath,
		EnvFileExisted:        hadFile,
		EnvFileWritable:       dotenv.IsWritablePath(writePath),
		PLUMAddr:              effectiveEnvString(overrides, envPLUMAddr),
		PLUMDatabaseURL:       effectiveDatabaseURLWithOverrides(overrides),
		MusicBrainzContactURL: effectiveEnvString(overrides, envMusicBrainzContactURL),
		SecretsPresent: serverEnvSecretsPresent{
			TMDBAPIKey:   effectiveSecretPresent(overrides, envTMDBAPIKey),
			TVDBAPIKey:   effectiveSecretPresent(overrides, envTVDBAPIKey),
			OMDBAPIKey:   effectiveSecretPresent(overrides, envOMDBAPIKey),
			FanartAPIKey: effectiveSecretPresent(overrides, envFanartAPIKey),
		},
		RestartRecommended: false,
		Help: "Values match what the server process loaded from environment and .env. " +
			"API keys are never returned in full; leave a field empty when saving to keep the current key. " +
			"Changing listen address or database path updates the file but requires a server restart.",
	}

	// Enrich non-secret display from the file when the process env is empty (e.g. keys only in .env before export).
	fileVals := dotenv.ReadKeyValues(envPath, managedEnvKeys)
	if resp.PLUMAddr == "" {
		resp.PLUMAddr = strings.TrimSpace(fileVals[envPLUMAddr])
	}
	if resp.PLUMDatabaseURL == "" {
		if v := strings.TrimSpace(fileVals[envPLUMDatabaseURL]); v != "" {
			resp.PLUMDatabaseURL = v
		} else {
			resp.PLUMDatabaseURL = strings.TrimSpace(fileVals[envPLUMDatabaseLegacyPath])
		}
	}
	if resp.MusicBrainzContactURL == "" {
		resp.MusicBrainzContactURL = strings.TrimSpace(fileVals[envMusicBrainzContactURL])
	}
	if !resp.SecretsPresent.TMDBAPIKey {
		resp.SecretsPresent.TMDBAPIKey = strings.TrimSpace(fileVals[envTMDBAPIKey]) != ""
	}
	if !resp.SecretsPresent.TVDBAPIKey {
		resp.SecretsPresent.TVDBAPIKey = strings.TrimSpace(fileVals[envTVDBAPIKey]) != ""
	}
	if !resp.SecretsPresent.OMDBAPIKey {
		resp.SecretsPresent.OMDBAPIKey = strings.TrimSpace(fileVals[envOMDBAPIKey]) != ""
	}
	if !resp.SecretsPresent.FanartAPIKey {
		resp.SecretsPresent.FanartAPIKey = strings.TrimSpace(fileVals[envFanartAPIKey]) != ""
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *ServerEnvSettingsHandler) loadServerEnvOverrides() (map[string]string, error) {
	if h.DB == nil {
		return nil, nil
	}
	return db.GetServerEnvOverrides(h.DB)
}

func (h *ServerEnvSettingsHandler) Put(w http.ResponseWriter, r *http.Request) {
	var req serverEnvUpdateRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}

	writePath, err := dotenv.ResolveWritePath()
	if err != nil {
		http.Error(w, "resolve .env path: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !dotenv.IsWritablePath(writePath) {
		http.Error(w, ".env file is not writable at "+writePath, http.StatusForbidden)
		return
	}

	updates := map[string]string{}
	restart := false

	if req.PLUMAddr != nil {
		updates[envPLUMAddr] = strings.TrimSpace(*req.PLUMAddr)
		restart = true
	}
	if req.PLUMDatabaseURL != nil {
		v := strings.TrimSpace(*req.PLUMDatabaseURL)
		updates[envPLUMDatabaseURL] = v
		updates[envPLUMDatabaseLegacyPath] = ""
		restart = true
	}
	if req.MusicBrainzContactURL != nil {
		updates[envMusicBrainzContactURL] = strings.TrimSpace(*req.MusicBrainzContactURL)
	}

	applySecret := func(clear *bool, set *string, key string) {
		if clear != nil && *clear {
			updates[key] = ""
			return
		}
		if set == nil {
			return
		}
		v := strings.TrimSpace(*set)
		if v == "" {
			return
		}
		updates[key] = v
	}
	applySecret(req.ClearTMDBAPIKey, req.TMDBAPIKey, envTMDBAPIKey)
	applySecret(req.ClearTVDBAPIKey, req.TVDBAPIKey, envTVDBAPIKey)
	applySecret(req.ClearOMDBAPIKey, req.OMDBAPIKey, envOMDBAPIKey)
	applySecret(req.ClearFanartAPIKey, req.FanartAPIKey, envFanartAPIKey)

	if len(updates) == 0 {
		http.Error(w, "no fields to update", http.StatusBadRequest)
		return
	}

	if h.DB == nil {
		http.Error(w, "database not configured for server env settings", http.StatusInternalServerError)
		return
	}
	// Persist app_settings first so a failed write does not leave .env ahead of the DB (restart
	// would otherwise diverge from runtime overrides and pipeline reconfigure).
	if err := db.UpsertServerEnvOverrides(r.Context(), h.DB, updates); err != nil {
		http.Error(w, "persist server env overrides: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := dotenv.Upsert(writePath, updates); err != nil {
		http.Error(w, "write .env: "+err.Error(), http.StatusInternalServerError)
		return
	}

	overrides, err := db.GetServerEnvOverrides(h.DB)
	if err != nil {
		http.Error(w, "reload server env overrides: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if h.Pipeline != nil {
		h.Pipeline.ReconfigureKeys(
			effectiveEnvString(overrides, envTMDBAPIKey),
			effectiveEnvString(overrides, envTVDBAPIKey),
			effectiveEnvString(overrides, envOMDBAPIKey),
			effectiveEnvString(overrides, envFanartAPIKey),
			effectiveEnvString(overrides, envMusicBrainzContactURL),
		)
	}

	resp := serverEnvSettingsResponse{
		EnvFilePath:           writePath,
		EnvFileExisted:        true,
		EnvFileWritable:       true,
		PLUMAddr:              effectiveEnvString(overrides, envPLUMAddr),
		PLUMDatabaseURL:       effectiveDatabaseURLWithOverrides(overrides),
		MusicBrainzContactURL: effectiveEnvString(overrides, envMusicBrainzContactURL),
		SecretsPresent: serverEnvSecretsPresent{
			TMDBAPIKey:   effectiveSecretPresent(overrides, envTMDBAPIKey),
			TVDBAPIKey:   effectiveSecretPresent(overrides, envTVDBAPIKey),
			OMDBAPIKey:   effectiveSecretPresent(overrides, envOMDBAPIKey),
			FanartAPIKey: effectiveSecretPresent(overrides, envFanartAPIKey),
		},
		RestartRecommended: restart,
		Help:               "Metadata provider keys were applied to the running server. Restart Plum if you changed listen address or database path.",
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
