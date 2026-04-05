package httpapi

import (
	"encoding/json"
	"net/http"
)

// decodeRequestJSON decodes the request body into dst, rejecting unknown JSON fields.
// On error it writes "invalid json" with status 400 and returns false.
func decodeRequestJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return false
	}
	return true
}
