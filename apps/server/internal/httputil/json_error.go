package httputil

import (
	"encoding/json"
	"net/http"
)

// PlumJSONError is the standard Plum HTTP error envelope. Web (`createPlumApiClient` in
// @plum/shared) and Android (PlumHttpMessages) use `message` for display and `error` as a
// stable machine key for branching when present.
type PlumJSONError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func WritePlumJSONError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(PlumJSONError{Error: code, Message: message})
}

// PlumErrorCodeFromHTTPStatus maps common status codes to stable machine keys; callers may
// override with a more specific code when the handler already knows one.
func PlumErrorCodeFromHTTPStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	case http.StatusUnprocessableEntity:
		return "unprocessable_entity"
	case http.StatusTooManyRequests:
		return "rate_limited"
	case http.StatusServiceUnavailable:
		return "service_unavailable"
	default:
		if status >= 500 {
			return "internal_error"
		}
		if status >= 400 {
			return "client_error"
		}
		return "error"
	}
}
