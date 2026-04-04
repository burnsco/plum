package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type artworkProfile struct {
	Name   string
	Width  int
	Height int
}

func defaultArtworkProfile(kind string) artworkProfile {
	switch kind {
	case "backdrop":
		return artworkProfile{Name: "backdrop", Width: 1280, Height: 720}
	default:
		return artworkProfile{Name: "poster", Width: 500, Height: 750}
	}
}

func HandleServeArtwork(
	w http.ResponseWriter,
	r *http.Request,
	dbConn *sql.DB,
	globalID int,
	artworkDir string,
	artworkKind string,
) error {
	item, err := GetMediaByID(dbConn, globalID)
	if err != nil || item == nil {
		return ErrNotFound
	}
	source := ""
	switch artworkKind {
	case "poster":
		source = item.PosterPath
	case "backdrop":
		source = item.BackdropPath
	default:
		return ErrNotFound
	}
	return serveEntityArtwork(w, r, dbConn, "media", globalID, artworkDir, artworkKind, source)
}

func HandleServeShowArtwork(
	w http.ResponseWriter,
	r *http.Request,
	dbConn *sql.DB,
	showID int,
	artworkDir string,
	artworkKind string,
	source string,
) error {
	return serveEntityArtwork(w, r, dbConn, "show", showID, artworkDir, artworkKind, source)
}

func serveEntityArtwork(
	w http.ResponseWriter,
	r *http.Request,
	dbConn *sql.DB,
	entityKind string,
	entityID int,
	artworkDir string,
	artworkKind string,
	source string,
) error {
	source = strings.TrimSpace(source)
	if source == "" {
		return ErrNotFound
	}

	profile := defaultArtworkProfile(artworkKind)
	if requested := strings.TrimSpace(r.URL.Query().Get("profile")); requested != "" && requested == "original" {
		profile = artworkProfile{Name: "original"}
	}
	asset, err := ensureArtworkAsset(r.Context(), dbConn, entityKind, entityID, artworkKind, source, artworkDir)
	if err != nil {
		return err
	}

	absPath := filepath.Join(artworkDir, asset.originalRelPath)
	contentType := asset.mimeType
	if profile.Name != "original" {
		relPath, err := ensureArtworkVariant(dbConn, asset, artworkDir, profile)
		if err != nil {
			return err
		}
		absPath = filepath.Join(artworkDir, relPath)
		contentType = "image/jpeg"
	}
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(absPath))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Cache-Control", "public, max-age=86400, stale-while-revalidate=604800")
	w.Header().Set("Content-Type", contentType)
	http.ServeFile(w, r, absPath)
	return nil
}

type artworkAssetRecord struct {
	id              int
	sourceURL       string
	kind            string
	contentHash     string
	mimeType        string
	width           int
	height          int
	originalRelPath string
}

func ensureArtworkAsset(
	ctx context.Context,
	dbConn *sql.DB,
	entityKind string,
	entityID int,
	artworkKind string,
	source string,
	artworkDir string,
) (artworkAssetRecord, error) {
	source = strings.TrimSpace(source)
	wantedSource := artworkSourceURL(source, artworkKind)

	var asset artworkAssetRecord
	if err := dbConn.QueryRow(
		`SELECT a.id, a.source_url, a.artwork_kind, COALESCE(a.content_hash, ''), COALESCE(a.mime_type, ''),
		        COALESCE(a.width, 0), COALESCE(a.height, 0), a.original_rel_path
		   FROM artwork_links l
		   JOIN artwork_assets a ON a.id = l.asset_id
		  WHERE l.entity_kind = ? AND l.entity_id = ? AND l.artwork_kind = ?`,
		entityKind,
		entityID,
		artworkKind,
	).Scan(&asset.id, &asset.sourceURL, &asset.kind, &asset.contentHash, &asset.mimeType, &asset.width, &asset.height, &asset.originalRelPath); err == nil {
		linkedSource := strings.TrimSpace(asset.sourceURL)
		if wantedSource != "" && linkedSource == wantedSource {
			if _, statErr := os.Stat(filepath.Join(artworkDir, asset.originalRelPath)); statErr == nil {
				return asset, nil
			}
		}
	}

	sourceURL := wantedSource
	if sourceURL == "" {
		return asset, ErrNotFound
	}
	if err := dbConn.QueryRow(
		`SELECT id, source_url, artwork_kind, COALESCE(content_hash, ''), COALESCE(mime_type, ''), COALESCE(width, 0), COALESCE(height, 0), original_rel_path
		   FROM artwork_assets
		  WHERE source_url = ? AND artwork_kind = ?`,
		sourceURL,
		artworkKind,
	).Scan(&asset.id, &asset.sourceURL, &asset.kind, &asset.contentHash, &asset.mimeType, &asset.width, &asset.height, &asset.originalRelPath); err == nil {
		if _, statErr := os.Stat(filepath.Join(artworkDir, asset.originalRelPath)); statErr == nil {
			if err := linkArtworkAsset(dbConn, entityKind, entityID, artworkKind, asset.id); err != nil {
				return asset, err
			}
			return asset, nil
		}
	}

	downloaded, err := downloadArtworkSource(ctx, sourceURL)
	if err != nil {
		return asset, err
	}
	defer downloaded.cleanup()

	if err := os.MkdirAll(filepath.Join(artworkDir, "originals"), 0o755); err != nil {
		return asset, err
	}

	contentHash := sha256Hex(downloaded.bytes)
	relPath, mimeType := "", downloaded.contentType
	if err := dbConn.QueryRow(
		`SELECT original_rel_path, COALESCE(mime_type, '') FROM artwork_assets WHERE COALESCE(content_hash, '') = ? LIMIT 1`,
		contentHash,
	).Scan(&relPath, &mimeType); err == nil && relPath != "" {
		if _, statErr := os.Stat(filepath.Join(artworkDir, relPath)); statErr == nil {
			downloaded.persistedRelPath = relPath
		}
	}
	if downloaded.persistedRelPath == "" {
		ext := filepath.Ext(downloaded.filename)
		if ext == "" {
			ext = extensionForContentType(downloaded.contentType)
		}
		if ext == "" {
			ext = ".img"
		}
		downloaded.persistedRelPath = filepath.Join("originals", contentHash+ext)
		if err := os.WriteFile(filepath.Join(artworkDir, downloaded.persistedRelPath), downloaded.bytes, 0o644); err != nil {
			return asset, err
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if err := dbConn.QueryRow(
		`INSERT INTO artwork_assets (
source_url, artwork_kind, content_hash, mime_type, width, height, original_rel_path, last_fetched_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(source_url, artwork_kind) DO UPDATE SET
content_hash = excluded.content_hash,
mime_type = excluded.mime_type,
width = excluded.width,
height = excluded.height,
original_rel_path = excluded.original_rel_path,
last_fetched_at = excluded.last_fetched_at,
updated_at = excluded.updated_at
RETURNING id`,
		sourceURL,
		artworkKind,
		contentHash,
		mimeType,
		downloaded.width,
		downloaded.height,
		downloaded.persistedRelPath,
		now,
		now,
		now,
	).Scan(&asset.id); err != nil {
		return asset, err
	}
	asset.sourceURL = sourceURL
	asset.kind = artworkKind
	asset.contentHash = contentHash
	asset.mimeType = mimeType
	asset.width = downloaded.width
	asset.height = downloaded.height
	asset.originalRelPath = downloaded.persistedRelPath
	if err := linkArtworkAsset(dbConn, entityKind, entityID, artworkKind, asset.id); err != nil {
		return asset, err
	}
	return asset, nil
}

func linkArtworkAsset(dbConn *sql.DB, entityKind string, entityID int, artworkKind string, assetID int) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := dbConn.Exec(
		`INSERT INTO artwork_links (entity_kind, entity_id, artwork_kind, asset_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(entity_kind, entity_id, artwork_kind) DO UPDATE SET asset_id = excluded.asset_id, updated_at = excluded.updated_at`,
		entityKind,
		entityID,
		artworkKind,
		assetID,
		now,
		now,
	)
	return err
}

type downloadedArtwork struct {
	bytes            []byte
	filename         string
	contentType      string
	width            int
	height           int
	persistedRelPath string
	cleanup          func()
}

func downloadArtworkSource(ctx context.Context, sourceURL string) (downloadedArtwork, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return downloadedArtwork{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return downloadedArtwork{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return downloadedArtwork{}, fmt.Errorf("fetch artwork: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return downloadedArtwork{}, err
	}
	cfg, _, err := image.DecodeConfig(strings.NewReader(string(body)))
	if err != nil {
		cfg, _, err = image.DecodeConfig(bytesReader(body))
	}
	if err != nil {
		cfg = image.Config{}
	}
	return downloadedArtwork{
		bytes:       body,
		filename:    filepath.Base(req.URL.Path),
		contentType: resp.Header.Get("Content-Type"),
		width:       cfg.Width,
		height:      cfg.Height,
		cleanup:     func() {},
	}, nil
}

func ensureArtworkVariant(dbConn *sql.DB, asset artworkAssetRecord, artworkDir string, profile artworkProfile) (string, error) {
	var existingRelPath string
	if err := dbConn.QueryRow(`SELECT rel_path FROM artwork_variants WHERE asset_id = ? AND profile = ?`, asset.id, profile.Name).Scan(&existingRelPath); err == nil {
		if _, statErr := os.Stat(filepath.Join(artworkDir, existingRelPath)); statErr == nil {
			return existingRelPath, nil
		}
	}
	if err := os.MkdirAll(filepath.Join(artworkDir, "variants"), 0o755); err != nil {
		return "", err
	}

	sourceFile, err := os.Open(filepath.Join(artworkDir, asset.originalRelPath))
	if err != nil {
		return "", err
	}
	defer sourceFile.Close()
	img, _, err := image.Decode(sourceFile)
	if err != nil {
		return "", err
	}
	resized := resizeImage(img, profile.Width, profile.Height)
	relPath := filepath.Join("variants", fmt.Sprintf("%d-%s.jpg", asset.id, profile.Name))
	absPath := filepath.Join(artworkDir, relPath)
	out, err := os.Create(absPath)
	if err != nil {
		return "", err
	}
	if err := jpeg.Encode(out, resized, &jpeg.Options{Quality: 85}); err != nil {
		_ = out.Close()
		return "", err
	}
	if err := out.Close(); err != nil {
		return "", err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := dbConn.Exec(
		`INSERT INTO artwork_variants (asset_id, profile, rel_path, width, height, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(asset_id, profile) DO UPDATE SET rel_path = excluded.rel_path, width = excluded.width, height = excluded.height, updated_at = excluded.updated_at`,
		asset.id,
		profile.Name,
		relPath,
		profile.Width,
		profile.Height,
		now,
		now,
	); err != nil {
		return "", err
	}
	return relPath, nil
}

func artworkSourceURL(path string, artworkKind string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if strings.HasPrefix(path, "/") {
		switch artworkKind {
		case "backdrop":
			return "https://image.tmdb.org/t/p/w780" + path
		case "poster":
			fallthrough
		default:
			return "https://image.tmdb.org/t/p/w500" + path
		}
	}
	return path
}

func extensionForContentType(contentType string) string {
	if contentType == "" {
		return ""
	}
	exts, _ := mime.ExtensionsByType(strings.Split(contentType, ";")[0])
	if len(exts) == 0 {
		return ""
	}
	return exts[0]
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func resizeImage(src image.Image, width, height int) image.Image {
	if width <= 0 || height <= 0 {
		return src
	}
	srcBounds := src.Bounds()
	if srcBounds.Dx() <= 0 || srcBounds.Dy() <= 0 {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		srcY := srcBounds.Min.Y + y*srcBounds.Dy()/height
		for x := 0; x < width; x++ {
			srcX := srcBounds.Min.X + x*srcBounds.Dx()/width
			dst.Set(x, y, color.RGBAModel.Convert(src.At(srcX, srcY)))
		}
	}
	return dst
}

type byteReader struct {
	data []byte
	pos  int
}

func (r *byteReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func bytesReader(data []byte) io.Reader {
	return &byteReader{data: data}
}
