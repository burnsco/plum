package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"plum/internal/arr"
	"plum/internal/db"
	"plum/internal/dotenv"
)

type MediaStackSettingsHandler struct {
	DB  *sql.DB
	Arr *arr.Service
}

func (h *MediaStackSettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	settings, err := db.GetEffectiveMediaStackSettings(h.DB)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(settings)
}

func (h *MediaStackSettingsHandler) Put(w http.ResponseWriter, r *http.Request) {
	var payload db.MediaStackSettings
	if !decodeRequestJSON(w, r, &payload) {
		return
	}

	settings, err := db.SaveMediaStackSettings(h.DB, payload)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, db.ErrMediaStackServiceIncomplete) ||
			errors.Is(err, db.ErrMediaStackRootFolderRequired) ||
			errors.Is(err, db.ErrMediaStackQualityProfileInvalid) {
			status = http.StatusBadRequest
		}
		http.Error(w, err.Error(), status)
		return
	}
	if h.Arr != nil {
		h.Arr.Invalidate()
	}

	if writePath, err := dotenv.ResolveWritePath(); err != nil {
		slog.Warn("resolve .env for media stack sync", "error", err)
	} else if !dotenv.IsWritablePath(writePath) {
		slog.Info("skip media stack .env sync: not writable", "path", writePath)
	} else if err := dotenv.Upsert(writePath, db.MediaStackSettingsEnvPairs(settings)); err != nil {
		slog.Warn("sync media stack to .env", "error", err)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(settings)
}

func (h *MediaStackSettingsHandler) Validate(w http.ResponseWriter, r *http.Request) {
	var payload db.MediaStackSettings
	if !decodeRequestJSON(w, r, &payload) {
		return
	}
	if h.Arr == nil {
		http.Error(w, "media stack unavailable", http.StatusServiceUnavailable)
		return
	}

	result, err := h.Arr.Validate(r.Context(), payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
