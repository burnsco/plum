package db

import (
	"path/filepath"
	"slices"
	"strings"
)

var subtitleQualifierAliases = map[string]string{
	"forced":          "forced",
	"forc":            "forced",
	"sdh":             "hearing_impaired",
	"hi":              "hearing_impaired",
	"cc":              "hearing_impaired",
	"hearing":         "hearing_impaired",
	"hearingimpaired": "hearing_impaired",
	"signs":           "signs",
	"sign":            "signs",
	"songs":           "signs",
	"dubtitles":       "dubtitles",
}

var subtitleLanguageAliases = map[string]string{
	"en":         "en",
	"eng":        "en",
	"english":    "en",
	"english us": "en",
	"english uk": "en",
	"ja":         "ja",
	"jp":         "ja",
	"jpn":        "ja",
	"japanese":   "ja",
	"es":         "es",
	"spa":        "es",
	"spanish":    "es",
	"fr":         "fr",
	"fra":        "fr",
	"fre":        "fr",
	"french":     "fr",
	"de":         "de",
	"deu":        "de",
	"ger":        "de",
	"german":     "de",
	"it":         "it",
	"ita":        "it",
	"italian":    "it",
	"pt":         "pt",
	"por":        "pt",
	"portuguese": "pt",
	"ko":         "ko",
	"kor":        "ko",
	"korean":     "ko",
	"zh":         "zh",
	"chi":        "zh",
	"zho":        "zh",
	"chinese":    "zh",
}

var subtitleLanguageLabels = map[string]string{
	"en":  "English",
	"ja":  "Japanese",
	"es":  "Spanish",
	"fr":  "French",
	"de":  "German",
	"it":  "Italian",
	"pt":  "Portuguese",
	"ko":  "Korean",
	"zh":  "Chinese",
	"und": "Unknown",
}

type subtitleQualifiers struct {
	Forced          bool
	HearingImpaired bool
	Signs           bool
	Dubtitles       bool
}

func canonicalSubtitleLanguage(raw string) string {
	n := normalizeSubtitleToken(raw)
	if n == "" {
		return ""
	}
	if mapped, ok := subtitleLanguageAliases[n]; ok {
		return mapped
	}
	if parts := strings.Fields(n); len(parts) > 0 {
		if mapped, ok := subtitleLanguageAliases[parts[0]]; ok {
			return mapped
		}
		if len(parts[0]) == 2 || len(parts[0]) == 3 {
			return parts[0]
		}
	}
	if len(n) == 2 || len(n) == 3 {
		return n
	}
	return ""
}

func displaySubtitleLanguage(raw string) string {
	lang := canonicalSubtitleLanguage(raw)
	if lang == "" {
		lang = normalizeSubtitleToken(raw)
	}
	if lang == "" {
		return "Unknown"
	}
	if label, ok := subtitleLanguageLabels[lang]; ok {
		return label
	}
	return strings.ToUpper(lang)
}

func normalizeSubtitleToken(raw string) string {
	n := strings.TrimSpace(strings.ToLower(raw))
	n = strings.ReplaceAll(n, "_", " ")
	n = strings.ReplaceAll(n, "-", " ")
	n = strings.Join(strings.Fields(n), " ")
	return n
}

func normalizeSubtitleFreeform(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, ".", " ")
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

func qualifierFromToken(raw string) string {
	n := normalizeSubtitleToken(raw)
	if n == "" {
		return ""
	}
	if mapped, ok := subtitleQualifierAliases[strings.ReplaceAll(n, " ", "")]; ok {
		return mapped
	}
	if mapped, ok := subtitleQualifierAliases[n]; ok {
		return mapped
	}
	return ""
}

func applySubtitleQualifierFlags(flags *subtitleQualifiers, qualifier string) {
	switch qualifier {
	case "forced":
		flags.Forced = true
	case "hearing_impaired":
		flags.HearingImpaired = true
	case "signs":
		flags.Signs = true
	case "dubtitles":
		flags.Dubtitles = true
	}
}

func subtitleQualifierLabels(flags subtitleQualifiers) []string {
	labels := make([]string, 0, 4)
	if flags.Forced {
		labels = append(labels, "Forced")
	}
	if flags.HearingImpaired {
		labels = append(labels, "SDH")
	}
	if flags.Signs {
		labels = append(labels, "Signs")
	}
	if flags.Dubtitles {
		labels = append(labels, "Dubtitles")
	}
	return labels
}

func subtitleDisplayTitle(title, language string, flags subtitleQualifiers) string {
	cleanTitle := strings.TrimSpace(title)
	langLabel := strings.TrimSpace(displaySubtitleLanguage(language))
	flagLabels := subtitleQualifierLabels(flags)
	if cleanTitle == "" && langLabel == "Unknown" && len(flagLabels) == 0 {
		return ""
	}

	parts := make([]string, 0, 3)
	if cleanTitle != "" && !strings.EqualFold(cleanTitle, langLabel) {
		parts = append(parts, cleanTitle)
	}
	if langLabel != "" && (len(parts) == 0 || !strings.EqualFold(parts[len(parts)-1], langLabel)) {
		parts = append(parts, langLabel)
	}
	for _, label := range flagLabels {
		if slices.ContainsFunc(parts, func(existing string) bool {
			return strings.EqualFold(existing, label)
		}) {
			continue
		}
		parts = append(parts, label)
	}
	if len(parts) == 0 {
		return "Unknown"
	}
	return strings.Join(parts, " • ")
}

type parsedSidecarSubtitle struct {
	Language string
	Title    string
	Forced   bool
	Default  bool
	HI       bool
	SortKey  string
}

func parseSidecarSubtitleMetadata(videoPath, subtitleName string) (parsedSidecarSubtitle, bool) {
	base := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	ext := strings.ToLower(filepath.Ext(subtitleName))
	if ext == "" {
		return parsedSidecarSubtitle{}, false
	}
	stem := strings.TrimSuffix(subtitleName, ext)
	if stem != base && !strings.HasPrefix(stem, base+".") {
		return parsedSidecarSubtitle{}, false
	}
	rest := strings.TrimPrefix(stem, base)
	if strings.HasPrefix(rest, ".") {
		rest = rest[1:]
	}
	tokens := []string{}
	if rest != "" {
		tokens = strings.Split(rest, ".")
	}
	lang := "und"
	var flags subtitleQualifiers
	var extras []string
	for idx, token := range tokens {
		if token == "" {
			return parsedSidecarSubtitle{}, false
		}
		if idx == 0 {
			if parsedLang := canonicalSubtitleLanguage(token); parsedLang != "" {
				lang = parsedLang
				continue
			}
			if qualifier := qualifierFromToken(token); qualifier != "" {
				applySubtitleQualifierFlags(&flags, qualifier)
				continue
			}
			return parsedSidecarSubtitle{}, false
		}
		if qualifier := qualifierFromToken(token); qualifier != "" {
			applySubtitleQualifierFlags(&flags, qualifier)
			continue
		}
		if idx == 1 && lang == "und" {
			if parsedLang := canonicalSubtitleLanguage(token); parsedLang != "" {
				lang = parsedLang
				continue
			}
		}
		extras = append(extras, normalizeSubtitleFreeform(token))
	}
	if len(extras) > 0 {
		return parsedSidecarSubtitle{}, false
	}
	title := subtitleDisplayTitle("", lang, flags)
	return parsedSidecarSubtitle{
		Language: lang,
		Title:    title,
		Forced:   flags.Forced,
		HI:       flags.HearingImpaired,
		SortKey:  strings.ToLower(subtitleName),
	}, true
}

func normalizeEmbeddedSubtitleMetadata(title, language string, forced, hearingImpaired bool) (string, string, bool, bool) {
	lang := canonicalSubtitleLanguage(language)
	if lang == "" {
		lang = "und"
	}
	flags := subtitleQualifiers{
		Forced:          forced,
		HearingImpaired: hearingImpaired,
	}

	clean := normalizeSubtitleFreeform(title)
	if clean != "" {
		lower := normalizeSubtitleToken(clean)
		if parsedLang := canonicalSubtitleLanguage(lower); parsedLang != "" && parsedLang == lang {
			clean = ""
		} else {
			segments := strings.Fields(lower)
			filtered := make([]string, 0, len(segments))
			for _, segment := range segments {
				if parsedLang := canonicalSubtitleLanguage(segment); parsedLang != "" && parsedLang == lang {
					continue
				}
				if qualifier := qualifierFromToken(segment); qualifier != "" {
					applySubtitleQualifierFlags(&flags, qualifier)
					continue
				}
				filtered = append(filtered, segment)
			}
			if len(filtered) == 0 {
				clean = ""
			} else {
				clean = subtitleCaseWords(strings.Join(filtered, " "))
			}
		}
	}

	return subtitleDisplayTitle(clean, lang, flags), lang, flags.Forced, flags.HearingImpaired
}

func subtitleCaseWords(value string) string {
	words := strings.Fields(strings.TrimSpace(value))
	for i, word := range words {
		if word == "" {
			continue
		}
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}
