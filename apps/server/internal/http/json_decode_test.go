package httpapi

import (
	"net/http/httptest"
	"strings"
	"testing"
)

type decodeJSONTestPayload struct {
	Email string `json:"email"`
}

func TestDecodeRequestJSONRejectsTrailingGarbage(t *testing.T) {
	req := httptest.NewRequest("POST", "/test", strings.NewReader(`{"email":"x"}JUNK`))
	w := httptest.NewRecorder()
	var payload decodeJSONTestPayload

	ok := decodeRequestJSON(w, req, &payload)
	if ok {
		t.Fatal("expected decodeRequestJSON to fail for trailing garbage")
	}
}

func TestDecodeRequestJSONRejectsMultipleValues(t *testing.T) {
	req := httptest.NewRequest("POST", "/test", strings.NewReader(`{"email":"x"} {"email":"y"}`))
	w := httptest.NewRecorder()
	var payload decodeJSONTestPayload

	ok := decodeRequestJSON(w, req, &payload)
	if ok {
		t.Fatal("expected decodeRequestJSON to fail for multiple JSON values")
	}
}

func TestDecodeRequestJSONAcceptsSingleObject(t *testing.T) {
	req := httptest.NewRequest("POST", "/test", strings.NewReader(`{"email":"x"}`))
	w := httptest.NewRecorder()
	var payload decodeJSONTestPayload

	ok := decodeRequestJSON(w, req, &payload)
	if !ok {
		t.Fatal("expected decodeRequestJSON to succeed for valid JSON")
	}
	if payload.Email != "x" {
		t.Fatalf("expected payload email to be decoded, got %q", payload.Email)
	}
}
