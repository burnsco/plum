package transcoder

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"plum/internal/db"
)

const hlsSubtitleGroupID = "subs"

// HlsWebSubtitle describes one text subtitle track exposed as an HLS WebVTT rendition
// (virtual media playlist under the playback revision directory).
type HlsWebSubtitle struct {
	PlaylistFile string // e.g. plum_subs_emb_3.m3u8
	MediaID      int
	VTTPath      string // root-relative URL to full WebVTT (e.g. /api/media/12/subtitles/embedded/3)
	DisplayName  string
	Language     string
}

// CollectHlsWebSubtitles lists sidecar and embedded text subtitles suitable for WebVTT delivery.
func CollectHlsWebSubtitles(media db.MediaItem) []HlsWebSubtitle {
	out := make([]HlsWebSubtitle, 0)

	for _, s := range media.Subtitles {
		if !sidecarSubtitleHlsEligible(s) {
			continue
		}
		out = append(out, HlsWebSubtitle{
			PlaylistFile: fmt.Sprintf("plum_subs_ext_%d.m3u8", s.ID),
			MediaID:      media.ID,
			VTTPath:      fmt.Sprintf("/api/subtitles/%d", s.ID),
			DisplayName:  subtitleDisplayLabel(s.Title, s.Language, fmt.Sprintf("Subtitle %d", s.ID)),
			Language:     normalizeHlsLanguage(s.Language),
		})
	}

	for _, e := range media.EmbeddedSubtitles {
		if !db.EmbeddedSubtitleWebVTTDeliveryEligible(e) {
			continue
		}
		out = append(out, HlsWebSubtitle{
			PlaylistFile: fmt.Sprintf("plum_subs_emb_%d.m3u8", e.StreamIndex),
			MediaID:      media.ID,
			VTTPath:      fmt.Sprintf("/api/media/%d/subtitles/embedded/%d", media.ID, e.StreamIndex),
			DisplayName:  subtitleDisplayLabel(e.Title, e.Language, fmt.Sprintf("Embedded %d", e.StreamIndex)),
			Language:     normalizeHlsLanguage(e.Language),
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

// InjectHlsSubtitleRenditions prepends #EXT-X-MEDIA subtitle entries and adds SUBTITLES="subs"
// to each #EXT-X-STREAM-INF line. If the playlist is not a multivariant master (no STREAM-INF),
// body is returned unchanged.
func InjectHlsSubtitleRenditions(body string, tracks []HlsWebSubtitle) string {
	if len(tracks) == 0 {
		return body
	}
	if strings.Contains(body, "TYPE=SUBTITLES") {
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
			`#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID=%s,NAME=%s,DEFAULT=NO,FORCED=NO,AUTOSELECT=YES,LANGUAGE=%s,URI=%s`,
			hlsQuoteAttr(hlsSubtitleGroupID),
			hlsQuoteAttr(t.DisplayName),
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

// ParseVirtualSubtitlePlaylistName returns (embedded|external, numeric id, true) for plum_subs_*.m3u8.
func ParseVirtualSubtitlePlaylistName(fileBase string) (kind string, id int, ok bool) {
	fileBase = path.Base(fileBase)
	if !strings.HasSuffix(fileBase, ".m3u8") || !strings.HasPrefix(fileBase, "plum_subs_") {
		return "", 0, false
	}
	rest := strings.TrimSuffix(strings.TrimPrefix(fileBase, "plum_subs_"), ".m3u8")
	if strings.HasPrefix(rest, "emb_") {
		n, err := strconv.Atoi(strings.TrimPrefix(rest, "emb_"))
		if err != nil {
			return "", 0, false
		}
		return "emb", n, true
	}
	if strings.HasPrefix(rest, "ext_") {
		n, err := strconv.Atoi(strings.TrimPrefix(rest, "ext_"))
		if err != nil {
			return "", 0, false
		}
		return "ext", n, true
	}
	return "", 0, false
}
