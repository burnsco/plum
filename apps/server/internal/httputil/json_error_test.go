package httputil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWritePlumJSONError(t *testing.T) {
	rec := httptest.NewRecorder()
	WritePlumJSONError(rec, http.StatusNotFound, "not_found", "media missing")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", ct)
	}
	var body PlumJSONError
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error != "not_found" || body.Message != "media missing" {
		t.Fatalf("body = %#v", body)
	}
}

func TestPlumErrorCodeFromHTTPStatus(t *testing.T) {
	if c := PlumErrorCodeFromHTTPStatus(http.StatusTeapot); c != "client_error" {
		t.Fatalf("418 = %q", c)
	}
	if c := PlumErrorCodeFromHTTPStatus(http.StatusInternalServerError); c != "internal_error" {
		t.Fatalf("500 = %q", c)
	}
}
