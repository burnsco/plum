package plum.tv.core.data

import android.content.Context
import android.graphics.Color
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.CoroutineScope
import org.json.JSONObject
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.stateIn
import plum.tv.core.data.di.ApplicationScope

private val Context.playerPrefsDataStore: DataStore<Preferences> by preferencesDataStore(name = "plum_player")

@Singleton
class PlayerSubtitlePreferences @Inject constructor(
    @param:ApplicationContext private val context: Context,
    @param:ApplicationScope private val scope: CoroutineScope,
) {
    private val store get() = context.playerPrefsDataStore

    private object Keys {
        val subtitleSize = stringPreferencesKey("subtitle_size")
        val subtitlePosition = stringPreferencesKey("subtitle_position")
        val subtitleColor = stringPreferencesKey("subtitle_color")
        val videoAspectRatio = stringPreferencesKey("video_aspect_ratio")
        val defaultAudioLanguage = stringPreferencesKey("default_audio_language")
        val defaultSubtitleLanguage = stringPreferencesKey("default_subtitle_language")
        val defaultSubtitleLabelHint = stringPreferencesKey("default_subtitle_label_hint")
        val showTrackLanguageOverrides = stringPreferencesKey("show_track_language_overrides_json")
    }

    /** Sparse overrides per `"$libraryId:$showKey"`; null fields inherit from [manualTrackLanguages]. */
    data class ShowTrackLanguageOverride(
        val defaultAudioLanguage: String? = null,
        val defaultSubtitleLanguage: String? = null,
        val defaultSubtitleLabelHint: String? = null,
    )

    data class ManualTrackLanguagePreferences(
        /** Normalized BCP-style code; empty = follow server / first track. */
        val defaultAudioLanguage: String = "",
        val defaultSubtitleLanguage: String = "",
        val defaultSubtitleLabelHint: String = "",
    )

    val manualTrackLanguages: StateFlow<ManualTrackLanguagePreferences> =
        store.data
            .map { prefs ->
                ManualTrackLanguagePreferences(
                    defaultAudioLanguage = prefs[Keys.defaultAudioLanguage]?.trim().orEmpty(),
                    defaultSubtitleLanguage = prefs[Keys.defaultSubtitleLanguage]?.trim().orEmpty(),
                    defaultSubtitleLabelHint = prefs[Keys.defaultSubtitleLabelHint]?.trim().orEmpty(),
                )
            }
            .stateIn(scope, SharingStarted.Eagerly, ManualTrackLanguagePreferences())

    val showTrackLanguageOverrides: StateFlow<Map<String, ShowTrackLanguageOverride>> =
        store.data
            .map { prefs -> parseShowTrackLanguageOverrides(prefs[Keys.showTrackLanguageOverrides]) }
            .stateIn(scope, SharingStarted.Eagerly, emptyMap())

    val appearance: StateFlow<SubtitleAppearance> = store.data
        .map { prefs ->
            SubtitleAppearance(
                size = SubtitleSize.fromStorage(prefs[Keys.subtitleSize]),
                position = SubtitlePosition.fromStorage(prefs[Keys.subtitlePosition]),
                colorHex = normalizeColorHex(prefs[Keys.subtitleColor]),
            )
        }
        .stateIn(scope, SharingStarted.Eagerly, SubtitleAppearance.DEFAULT)

    val videoAspectRatioMode: StateFlow<VideoAspectRatioMode> = store.data
        .map { prefs -> VideoAspectRatioMode.fromStorage(prefs[Keys.videoAspectRatio]) }
        .stateIn(scope, SharingStarted.Eagerly, VideoAspectRatioMode.AUTO)

    suspend fun setAppearance(value: SubtitleAppearance) {
        store.edit {
            it[Keys.subtitleSize] = value.size.storageValue
            it[Keys.subtitlePosition] = value.position.storageValue
            it[Keys.subtitleColor] = normalizeColorHex(value.colorHex)
        }
    }

    suspend fun setVideoAspectRatioMode(mode: VideoAspectRatioMode) {
        store.edit { it[Keys.videoAspectRatio] = mode.storageValue }
    }

    suspend fun setDefaultAudioLanguage(languageNormalized: String) {
        store.edit { it[Keys.defaultAudioLanguage] = languageNormalized.trim() }
    }

    suspend fun setDefaultSubtitlePreference(language: String, labelHint: String) {
        store.edit {
            it[Keys.defaultSubtitleLanguage] = language.trim()
            it[Keys.defaultSubtitleLabelHint] = labelHint.trim()
        }
    }

    suspend fun mergeShowTrackLanguageOverride(
        compositeKey: String,
        defaultAudioLanguage: String? = null,
        defaultSubtitleLanguage: String? = null,
        defaultSubtitleLabelHint: String? = null,
    ) {
        if (compositeKey.isBlank()) return
        store.edit { pref ->
            val cur = parseShowTrackLanguageOverrides(pref[Keys.showTrackLanguageOverrides]).toMutableMap()
            val prev = cur[compositeKey] ?: ShowTrackLanguageOverride()
            val next =
                ShowTrackLanguageOverride(
                    defaultAudioLanguage = defaultAudioLanguage ?: prev.defaultAudioLanguage,
                    defaultSubtitleLanguage = defaultSubtitleLanguage ?: prev.defaultSubtitleLanguage,
                    defaultSubtitleLabelHint = defaultSubtitleLabelHint ?: prev.defaultSubtitleLabelHint,
                )
            if (next.defaultAudioLanguage == null &&
                next.defaultSubtitleLanguage == null &&
                next.defaultSubtitleLabelHint == null
            ) {
                cur.remove(compositeKey)
            } else {
                cur[compositeKey] = next
            }
            val encoded = serializeShowTrackLanguageOverrides(cur)
            if (encoded.isEmpty()) {
                pref.remove(Keys.showTrackLanguageOverrides)
            } else {
                pref[Keys.showTrackLanguageOverrides] = encoded
            }
        }
    }

    private fun parseShowTrackLanguageOverrides(raw: String?): Map<String, ShowTrackLanguageOverride> {
        if (raw.isNullOrBlank()) return emptyMap()
        return runCatching {
            val root = JSONObject(raw)
            buildMap {
                for (k in root.keys()) {
                    val o = root.optJSONObject(k) ?: continue
                    val audio = o.optString("a", "").trim().takeIf { it.isNotEmpty() }
                    val sub = o.optString("s", "").trim().takeIf { it.isNotEmpty() }
                    val hint = o.optString("l", "").trim().takeIf { it.isNotEmpty() }
                    if (audio != null || sub != null || hint != null) {
                        put(
                            k,
                            ShowTrackLanguageOverride(
                                defaultAudioLanguage = audio,
                                defaultSubtitleLanguage = sub,
                                defaultSubtitleLabelHint = hint,
                            ),
                        )
                    }
                }
            }
        }.getOrElse { emptyMap() }
    }

    private fun serializeShowTrackLanguageOverrides(map: Map<String, ShowTrackLanguageOverride>): String {
        if (map.isEmpty()) return ""
        val root = JSONObject()
        for ((k, v) in map) {
            val o = JSONObject()
            v.defaultAudioLanguage?.trim()?.takeIf { it.isNotEmpty() }?.let { o.put("a", it) }
            v.defaultSubtitleLanguage?.trim()?.takeIf { it.isNotEmpty() }?.let { o.put("s", it) }
            v.defaultSubtitleLabelHint?.trim()?.takeIf { it.isNotEmpty() }?.let { o.put("l", it) }
            if (o.length() > 0) {
                root.put(k, o)
            }
        }
        return if (root.length() == 0) "" else root.toString()
    }

    private fun normalizeColorHex(raw: String?): String {
        val trimmed = raw?.trim().orEmpty()
        if (trimmed.isEmpty()) return SubtitleAppearance.DEFAULT.colorHex
        val withHash = if (trimmed.startsWith("#")) trimmed else "#$trimmed"
        return if (runCatching { Color.parseColor(withHash) }.isSuccess) {
            withHash.lowercase()
        } else {
            SubtitleAppearance.DEFAULT.colorHex
        }
    }
}
