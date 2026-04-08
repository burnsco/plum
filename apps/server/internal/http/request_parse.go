package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// parsePathInt parses raw as a base-10 integer. On parse failure it writes message with
// http.StatusBadRequest. Unlike chiURLIntParam, zero and negative values are allowed.
func parsePathInt(w http.ResponseWriter, raw string, message string) (int, bool) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		http.Error(w, message, http.StatusBadRequest)
		return 0, false
	}
	return value, true
}

// chiURLIntParam parses chi.URLParam(r, key) as a positive integer. On failure it writes
// http.StatusBadRequest with a short plain-text body and returns (0, false).
func chiURLIntParam(w http.ResponseWriter, r *http.Request, key, label string) (int, bool) {
	raw := strings.TrimSpace(chi.URLParam(r, key))
	if raw == "" {
		http.Error(w, "missing "+label, http.StatusBadRequest)
		return 0, false
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		http.Error(w, "invalid "+label, http.StatusBadRequest)
		return 0, false
	}
	return n, true
}

// chiURLIntParamInvalidID is like chiURLIntParam but always responds with "invalid id" on failure
// (matches handlers that did not distinguish empty from malformed).
func chiURLIntParamInvalidID(w http.ResponseWriter, r *http.Request, key string) (int, bool) {
	raw := strings.TrimSpace(chi.URLParam(r, key))
	if raw == "" {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return 0, false
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return 0, false
	}
	return n, true
}
