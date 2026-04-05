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
    }

    val appearance: StateFlow<SubtitleAppearance> = store.data
        .map { prefs ->
            SubtitleAppearance(
                size = SubtitleSize.fromStorage(prefs[Keys.subtitleSize]),
                position = SubtitlePosition.fromStorage(prefs[Keys.subtitlePosition]),
                colorHex = normalizeColorHex(prefs[Keys.subtitleColor]),
            )
        }
        .stateIn(scope, SharingStarted.Eagerly, SubtitleAppearance.DEFAULT)

    suspend fun setAppearance(value: SubtitleAppearance) {
        store.edit {
            it[Keys.subtitleSize] = value.size.storageValue
            it[Keys.subtitlePosition] = value.position.storageValue
            it[Keys.subtitleColor] = normalizeColorHex(value.colorHex)
        }
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
