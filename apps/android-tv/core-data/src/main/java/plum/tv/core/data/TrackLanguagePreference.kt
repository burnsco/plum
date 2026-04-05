package plum.tv.core.data

import java.util.Locale

/** Client-side track language defaults (aligned with web `PLAYER_WEB_TRACK_LANGUAGE_NONE`). */
object TrackLanguagePreference {
    /** Disables automatic subtitle selection (subtitles stay off until chosen). */
    const val NONE = "__none__"

    private val languageAliases =
        mapOf(
            "en" to "en",
            "eng" to "en",
            "english" to "en",
            "ja" to "ja",
            "jp" to "ja",
            "jpn" to "ja",
            "japanese" to "ja",
            "es" to "es",
            "spa" to "es",
            "spanish" to "es",
            "fr" to "fr",
            "fre" to "fr",
            "fra" to "fr",
            "french" to "fr",
            "de" to "de",
            "deu" to "de",
            "ger" to "de",
            "german" to "de",
            "it" to "it",
            "ita" to "it",
            "italian" to "it",
            "pt" to "pt",
            "por" to "pt",
            "portuguese" to "pt",
            "ko" to "ko",
            "kor" to "ko",
            "korean" to "ko",
            "zh" to "zh",
            "chi" to "zh",
            "zho" to "zh",
            "chinese" to "zh",
        )

    fun normalize(raw: String?): String {
        val normalized = raw?.trim()?.lowercase(Locale.US).orEmpty()
        if (normalized.isEmpty()) return ""
        return languageAliases[normalized]
            ?: normalized.split(Regex("[\\s_-]")).firstOrNull()?.takeIf { it.isNotEmpty() }
            ?: normalized
    }

    fun matchesLanguage(value: String?, preferredNormalized: String): Boolean {
        if (preferredNormalized.isEmpty()) return false
        val v = normalize(value)
        if (v.isEmpty()) return false
        return v == preferredNormalized
    }

    fun subtitleLabelMatchesHint(trackLabel: String, hint: String): Boolean {
        val a = trackLabel.trim().lowercase(Locale.US)
        val b = hint.trim().lowercase(Locale.US)
        if (b.isEmpty()) return true
        return a == b || a.contains(b) || b.contains(a)
    }
}
