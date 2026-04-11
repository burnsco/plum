package transcoder

import (
	"encoding/hex"
	"fmt"
	"path"
	"strings"

	"plum/internal/db"
)

const hlsSubtitleGroupID = "subs"

// HlsWebSubtitle describes one text subtitle track exposed as an HLS WebVTT rendition
// (virtual media playlist under the playback revision directory).
type HlsWebSubtitle struct {
	LogicalID    string
	PlaylistFile string // e.g. plum_subs_656d623a33.m3u8
	MediaID      int
	VTTPath      string // root-relative URL to full WebVTT (e.g. /api/media/12/subtitles/embedded/3)
	DisplayName  string
	Language     string
	Default      bool
	Forced       bool
	Autoselect   bool
}

func hlsSubtitlePlaylistFileForLogicalID(logicalID string) string {
	return fmt.Sprintf("plum_subs_%s.m3u8", hex.EncodeToString([]byte(logicalID)))
}

func parseVirtualSubtitlePlaylistName(fileBase string) (logicalID string, ok bool) {
	fileBase = path.Base(fileBase)
	if !strings.HasSuffix(fileBase, ".m3u8") || !strings.HasPrefix(fileBase, "plum_subs_") {
		return "", false
	}
	encoded := strings.TrimSuffix(strings.TrimPrefix(fileBase, "plum_subs_"), ".m3u8")
	if encoded == "" {
		return "", false
	}
	decoded, err := hex.DecodeString(encoded)
	if err != nil || len(decoded) == 0 {
		return "", false
	}
	return string(decoded), true
}

// CollectHlsWebSubtitles lists sidecar and embedded text subtitles suitable for WebVTT delivery.
func CollectHlsWebSubtitles(media db.MediaItem) []HlsWebSubtitle {
	out := make([]HlsWebSubtitle, 0)

	for _, s := range media.Subtitles {
		if !sidecarSubtitleHlsEligible(s) {
			continue
		}
		name := strings.TrimSpace(s.Title)
		if name == "" {
			name = subtitleDisplayLabel("", s.Language, fmt.Sprintf("Subtitle %d", s.ID))
		}
		out = append(out, HlsWebSubtitle{
			LogicalID:    db.SidecarSubtitleLogicalID(s.ID),
			PlaylistFile: hlsSubtitlePlaylistFileForLogicalID(db.SidecarSubtitleLogicalID(s.ID)),
			MediaID:      media.ID,
			VTTPath:      fmt.Sprintf("/api/subtitles/%d", s.ID),
			DisplayName:  name,
			Language:     normalizeHlsLanguage(s.Language),
			Default:      s.Default,
			Forced:       s.Forced,
			Autoselect:   s.Default || s.Forced || !s.HearingImpaired,
		})
	}

	for _, e := range media.EmbeddedSubtitles {
		if !db.EmbeddedSubtitleWebVTTDeliveryEligible(e) {
			continue
		}
		name := strings.TrimSpace(e.Title)
		if name == "" {
			name = subtitleDisplayLabel("", e.Language, fmt.Sprintf("Embedded %d", e.StreamIndex))
		}
		out = append(out, HlsWebSubtitle{
			LogicalID:    db.EmbeddedSubtitleLogicalID(e.StreamIndex),
			PlaylistFile: hlsSubtitlePlaylistFileForLogicalID(db.EmbeddedSubtitleLogicalID(e.StreamIndex)),
			MediaID:      media.ID,
			VTTPath:      fmt.Sprintf("/api/media/%d/subtitles/embedded/%d", media.ID, e.StreamIndex),
			DisplayName:  name,
			Language:     normalizeHlsLanguage(e.Language),
			Default:      e.Default,
			Forced:       e.Forced,
			Autoselect:   e.Default || e.Forced || !e.HearingImpaired,
		})
	}

	return out
}

func sidecarSubtitleHlsEligible(s db.Subtitle) bool {
	switch strings.ToLower(strings.TrimSpace(s.Format)) {
	case "vtt", "srt", "ass", "ssa":
		return true
	default:
		return false
	}
}

func subtitleDisplayLabel(title, language, fallback string) string {
	t := strings.TrimSpace(title)
	lang := strings.TrimSpace(language)
	if t != "" && lang != "" && !strings.EqualFold(t, lang) {
		return t + " • " + lang
	}
	if t != "" {
		return t
	}
	if lang != "" {
		return lang
	}
	return fallback
}

func normalizeHlsLanguage(lang string) string {
	lang = strings.TrimSpace(lang)
	if lang == "" {
		return "und"
	}
	// HLS attributes: keep alphanumerics + hyphen; trim junk.
	var b strings.Builder
	for _, r := range lang {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	s := b.String()
	if s == "" {
		return "und"
	}
	return strings.ToLower(s)
}

func hlsQuoteAttr(value string) string {
	escaped := strings.ReplaceAll(value, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}

func hlsYesNo(value bool) string {
	if value {
		return "YES"
	}
	return "NO"
}

// InjectHlsSubtitleRenditions prepends #EXT-X-MEDIA subtitle entries and adds SUBTITLES="subs"
// to each #EXT-X-STREAM-INF line. If the playlist is not a multivariant master (no STREAM-INF),
// body is returned unchanged.
func InjectHlsSubtitleRenditions(body string, tracks []HlsWebSubtitle) string {
	if len(tracks) == 0 {
		return body
	}
	if strings.Contains(body, fmt.Sprintf(`TYPE=SUBTITLES,GROUP-ID=%s`, hlsQuoteAttr(hlsSubtitleGroupID))) {
		return body
	}
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	hasStreamInf := false
	for _, line := range lines {
		if strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			hasStreamInf = true
			break
		}
	}
	if !hasStreamInf {
		return body
	}

	var mediaLines []string
	for _, t := range tracks {
		uri := path.Base(t.PlaylistFile)
		line := fmt.Sprintf(
			`#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID=%s,NAME=%s,DEFAULT=%s,FORCED=%s,AUTOSELECT=%s,LANGUAGE=%s,URI=%s`,
			hlsQuoteAttr(hlsSubtitleGroupID),
			hlsQuoteAttr(t.DisplayName),
			hlsYesNo(t.Default),
			hlsYesNo(t.Forced),
			hlsYesNo(t.Autoselect),
			hlsQuoteAttr(t.Language),
			hlsQuoteAttr(uri),
		)
		mediaLines = append(mediaLines, line)
	}

	var out strings.Builder
	insertedMedia := false
	for _, line := range lines {
		if !insertedMedia && strings.HasPrefix(line, "#EXTM3U") {
			out.WriteString(line)
			out.WriteByte('\n')
			for _, ml := range mediaLines {
				out.WriteString(ml)
				out.WriteByte('\n')
			}
			insertedMedia = true
			continue
		}
		if strings.HasPrefix(line, "#EXT-X-STREAM-INF:") && !strings.Contains(line, "SUBTITLES=") {
			trimmed := strings.TrimSuffix(line, "\r")
			out.WriteString(trimmed)
			out.WriteString(fmt.Sprintf(`,SUBTITLES=%s`, hlsQuoteAttr(hlsSubtitleGroupID)))
			out.WriteByte('\n')
			continue
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return strings.TrimSuffix(out.String(), "\n")
}

// BuildWebVttSubtitleMediaPlaylist returns a minimal HLS media playlist pointing at one WebVTT resource.
func BuildWebVttSubtitleMediaPlaylist(vttURL string, durationSeconds int) string {
	dur := durationSeconds
	if dur <= 0 {
		dur = 3600
	}
	ext := fmt.Sprintf("%.3f", float64(dur))
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	b.WriteString("#EXT-X-VERSION:3\n")
	b.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", dur))
	b.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")
	b.WriteString(fmt.Sprintf("#EXTINF:%s,\n", ext))
	b.WriteString(vttURL)
	b.WriteString("\n")
	b.WriteString("#EXT-X-ENDLIST\n")
	return b.String()
}

// ParseVirtualSubtitlePlaylistName returns the logical subtitle id for plum_subs_*.m3u8.
func ParseVirtualSubtitlePlaylistName(fileBase string) (logicalID string, ok bool) {
	return parseVirtualSubtitlePlaylistName(fileBase)
}
