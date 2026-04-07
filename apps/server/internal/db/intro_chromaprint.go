package db

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/bits"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ErrChromaprintUnavailable means ffmpeg was built without the chromaprint muxer (common on minimal builds).
var ErrChromaprintUnavailable = errors.New("ffmpeg chromaprint muxer not available; use jellyfin-ffmpeg or an ffmpeg build with chromaprint")

const chromaprintIntroWindowSec = 120
const chromaprintHammingThreshold = 10 // max differing bits per uint32 subfingerprint pair

// IntroFingerprintCacheDir returns the directory for cached raw fingerprints.
// Set PLUM_INTRO_FINGERPRINT_DIR to override. Default: <dirname(plum.db)>/intro_fingerprints
func IntroFingerprintCacheDir() string {
	if p := strings.TrimSpace(os.Getenv("PLUM_INTRO_FINGERPRINT_DIR")); p != "" {
		return p
	}
	base := plumAuxDBFilePath()
	if base == "" || base == ":memory:" {
		return ""
	}
	dir := filepath.Dir(base)
	if dir == "" || dir == "." {
		return ""
	}
	return filepath.Join(dir, "intro_fingerprints")
}

// ChromaprintMuxersAvailable probes ffmpeg for the chromaprint muxer.
func ChromaprintMuxersAvailable() bool {
	cmd := exec.Command("ffmpeg", "-hide_banner", "-h", "muxer=chromaprint")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return false
	}
	return !strings.Contains(strings.ToLower(stderr.String()), "unknown format")
}

func fingerprintCachePath(cacheRoot, filePath string, fileSize int64, fileMod string) string {
	h := sha256.Sum256([]byte(strings.TrimSpace(filePath) + "\n" + strconv.FormatInt(fileSize, 10) + "\n" + strings.TrimSpace(fileMod)))
	return filepath.Join(cacheRoot, hex.EncodeToString(h[:16])+".rawfp")
}

// ReadChromaprintRawFingerprint extracts the first maxSeconds of audio as raw chromaprint uint32 stream.
func ReadChromaprintRawFingerprint(ctx context.Context, videoPath string, maxSeconds int, cacheRoot string) ([]uint32, error) {
	if maxSeconds <= 0 {
		maxSeconds = chromaprintIntroWindowSec
	}
	if !ChromaprintMuxersAvailable() {
		return nil, ErrChromaprintUnavailable
	}
	st, err := os.Stat(videoPath)
	if err != nil {
		return nil, err
	}
	mod := st.ModTime().UTC().Format(time.RFC3339Nano)
	cacheFile := ""
	if cacheRoot != "" {
		cacheFile = fingerprintCachePath(cacheRoot, videoPath, st.Size(), mod)
		if raw, rerr := os.ReadFile(cacheFile); rerr == nil && len(raw) >= 4 {
			return decodeRawFingerprint(raw), nil
		}
	}

	probeCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	args := []string{
		"-hide_banner", "-nostdin", "-loglevel", "error",
		"-i", videoPath,
		"-t", strconv.Itoa(maxSeconds),
		"-map", "0:a:0?",
		"-vn",
		"-ac", "1", "-ar", "44100",
		"-f", "chromaprint", "-fp_format", "raw",
		"pipe:1",
	}
	cmd := exec.CommandContext(probeCtx, "ffmpeg", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg chromaprint: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	raw := stdout.Bytes()
	if len(raw) < 4 {
		return nil, fmt.Errorf("empty chromaprint output")
	}
	if cacheFile != "" {
		_ = os.MkdirAll(filepath.Dir(cacheFile), 0o755)
		_ = os.WriteFile(cacheFile, raw, 0o644)
	}
	return decodeRawFingerprint(raw), nil
}

func decodeRawFingerprint(raw []byte) []uint32 {
	n := len(raw) / 4
	out := make([]uint32, n)
	for i := 0; i < n; i++ {
		out[i] = binary.LittleEndian.Uint32(raw[i*4 : i*4+4])
	}
	return out
}

func hammingBits(a, b uint32) int {
	return bits.OnesCount32(a ^ b)
}

// introEndFromPair estimates intro end (seconds) from two raw fingerprints of the same window length.
func introEndFromPair(fpA, fpB []uint32, windowSec float64) (float64, bool) {
	n := len(fpA)
	if len(fpB) < n {
		n = len(fpB)
	}
	if n < 32 {
		return 0, false
	}
	minIntro := 16
	var diverge int
	found := false
	for i := 0; i < n; i++ {
		if hammingBits(fpA[i], fpB[i]) > chromaprintHammingThreshold {
			if i >= minIntro {
				diverge = i
				found = true
				break
			}
		}
	}
	if !found || diverge < minIntro {
		return 0, false
	}
	secPer := windowSec / float64(n)
	end := float64(diverge) * secPer
	if end < 5 || end > windowSec*0.95 {
		return 0, false
	}
	return end, true
}

func medianFloat(xs []float64) (float64, bool) {
	if len(xs) == 0 {
		return 0, false
	}
	sort.Float64s(xs)
	mid := len(xs) / 2
	if len(xs)%2 == 1 {
		return xs[mid], true
	}
	return (xs[mid-1] + xs[mid]) / 2, true
}

// RunChromaprintIntroForSeason fingerprints episodes in one season and writes a shared intro end estimate
// to each unlocked primary media_files row (intro start 0).
func RunChromaprintIntroForSeason(ctx context.Context, dbConn *sql.DB, items []MediaItem, cacheRoot string) (int, error) {
	if len(items) < 2 {
		return 0, nil
	}
	if cacheRoot == "" {
		return 0, fmt.Errorf("intro fingerprint cache directory unset (configure PLUM_INTRO_FINGERPRINT_DIR or use a file-backed database)")
	}
	type fpRec struct {
		item MediaItem
		fp   []uint32
	}
	recs := make([]fpRec, 0, len(items))
	for _, it := range items {
		if it.Missing || it.ID <= 0 {
			continue
		}
		path, err := ResolveMediaSourcePath(dbConn, it)
		if err != nil {
			continue
		}
		fp, err := ReadChromaprintRawFingerprint(ctx, path, chromaprintIntroWindowSec, cacheRoot)
		if err != nil {
			continue
		}
		recs = append(recs, fpRec{item: it, fp: fp})
	}
	if len(recs) < 2 {
		return 0, nil
	}
	var estimates []float64
	for i := 0; i < len(recs); i++ {
		for j := i + 1; j < len(recs); j++ {
			if end, ok := introEndFromPair(recs[i].fp, recs[j].fp, chromaprintIntroWindowSec); ok {
				estimates = append(estimates, end)
			}
		}
	}
	med, ok := medianFloat(estimates)
	if !ok {
		return 0, nil
	}
	z := 0.0
	updated := 0
	for _, r := range recs {
		locked, err := MediaFileIntroLocked(ctx, dbConn, r.item.ID)
		if err != nil || locked {
			continue
		}
		if err := UpdateMediaFileIntroFromChromaprint(ctx, dbConn, r.item.ID, z, med); err != nil {
			continue
		}
		updated++
	}
	return updated, nil
}

// RunChromaprintIntroScanForLibrary runs chromaprint intro detection per show and season for TV/anime libraries.
func RunChromaprintIntroScanForLibrary(ctx context.Context, dbConn *sql.DB, libraryID int, showKeyFilter string, cacheRoot string) (shows int, episodesUpdated int, err error) {
	if !ChromaprintMuxersAvailable() {
		return 0, 0, ErrChromaprintUnavailable
	}
	if strings.TrimSpace(cacheRoot) == "" {
		return 0, 0, fmt.Errorf("intro fingerprint cache directory unset (set PLUM_INTRO_FINGERPRINT_DIR or use a file-backed database path)")
	}
	var typ string
	if err := dbConn.QueryRow(`SELECT type FROM libraries WHERE id = ?`, libraryID).Scan(&typ); err != nil {
		return 0, 0, err
	}
	if typ != LibraryTypeTV && typ != LibraryTypeAnime {
		return 0, 0, nil
	}
	showsRows, err := ListIntroScanShowSummaries(dbConn, libraryID)
	if err != nil {
		return 0, 0, err
	}
	for _, sh := range showsRows {
		if showKeyFilter != "" && sh.ShowKey != showKeyFilter {
			continue
		}
		items, err := GetMediaByLibraryIDAndShowKey(dbConn, libraryID, typ, sh.ShowKey)
		if err != nil {
			continue
		}
		items, err = attachMediaFilesBatch(dbConn, items)
		if err != nil {
			continue
		}
		bySeason := make(map[int][]MediaItem)
		for _, it := range items {
			if it.Missing {
				continue
			}
			bySeason[it.Season] = append(bySeason[it.Season], it)
		}
		showWorked := false
		for _, group := range bySeason {
			n, rerr := RunChromaprintIntroForSeason(ctx, dbConn, group, cacheRoot)
			if rerr != nil {
				return shows, episodesUpdated, rerr
			}
			if n > 0 {
				episodesUpdated += n
				showWorked = true
			}
		}
		if showWorked {
			shows++
		}
	}
	return shows, episodesUpdated, nil
}
