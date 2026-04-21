package db

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEnsureArtworkAsset_UpdatesLinkWhenPosterSourceChanges(t *testing.T) {
	var bodyA, bodyB bytes.Buffer
	imgA := image.NewRGBA(image.Rect(0, 0, 2, 2))
	imgA.Set(0, 0, color.RGBA{R: 200, G: 10, B: 10, A: 255})
	if err := png.Encode(&bodyA, imgA); err != nil {
		t.Fatalf("encode png a: %v", err)
	}
	imgB := image.NewRGBA(image.Rect(0, 0, 2, 2))
	imgB.Set(0, 0, color.RGBA{R: 10, G: 200, B: 10, A: 255})
	if err := png.Encode(&bodyB, imgB); err != nil {
		t.Fatalf("encode png b: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		switch r.URL.Path {
		case "/a.png":
			_, _ = w.Write(bodyA.Bytes())
		case "/b.png":
			_, _ = w.Write(bodyB.Bytes())
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dbConn := newTestDB(t)
	t.Cleanup(func() { _ = dbConn.Close() })
	dir := t.TempDir()
	ctx := context.Background()

	urlA := server.URL + "/a.png"
	urlB := server.URL + "/b.png"

	first, err := ensureArtworkAsset(ctx, dbConn, "media", 4242, "poster", urlA, dir)
	if err != nil {
		t.Fatalf("first ensure: %v", err)
	}
	second, err := ensureArtworkAsset(ctx, dbConn, "media", 4242, "poster", urlB, dir)
	if err != nil {
		t.Fatalf("second ensure: %v", err)
	}
	if second.id == first.id {
		t.Fatalf("expected a new artwork asset after poster source change (first=%d second=%d)", first.id, second.id)
	}
	if strings.TrimSpace(second.sourceURL) != urlB {
		t.Fatalf("second asset source_url = %q, want %q", second.sourceURL, urlB)
	}
}

func TestDownloadArtworkSourceRejectsOversizedContentLength(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "20971521")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_, err := downloadArtworkSource(context.Background(), server.URL+"/poster.png")
	if err == nil || !strings.Contains(err.Error(), "response too large") {
		t.Fatalf("downloadArtworkSource err = %v, want response too large", err)
	}
}

func TestDownloadArtworkSourceRejectsOversizedStreamingBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = io.Copy(w, io.LimitReader(zeroReader{}, maxArtworkDownloadReadBytes))
	}))
	defer server.Close()

	_, err := downloadArtworkSource(context.Background(), server.URL+"/poster.png")
	if err == nil || !strings.Contains(err.Error(), "response too large") {
		t.Fatalf("downloadArtworkSource err = %v, want response too large", err)
	}
}

func TestArtworkHTTPClientHasTimeout(t *testing.T) {
	if artworkHTTPClient == nil {
		t.Fatal("artworkHTTPClient is nil")
	}
	if artworkHTTPClient.Timeout != artworkHTTPTimeout {
		t.Fatalf("timeout = %s, want %s", artworkHTTPClient.Timeout, artworkHTTPTimeout)
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}
