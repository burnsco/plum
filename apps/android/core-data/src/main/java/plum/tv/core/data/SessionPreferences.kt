package plum.tv.core.data

import android.content.Context
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

private val Context.sessionDataStore: DataStore<Preferences> by preferencesDataStore(name = "plum_session")

@Singleton
class SessionPreferences @Inject constructor(
    @ApplicationContext private val context: Context,
    @ApplicationScope private val scope: CoroutineScope,
) {
    private val store get() = context.sessionDataStore

    private object Keys {
        val serverUrl = stringPreferencesKey("server_url")
        val sessionToken = stringPreferencesKey("session_token")
    }

    // Shared StateFlow so all collectors share a single DataStore subscription and .value reads are non-blocking.
    val serverUrl: StateFlow<String?> = store.data
        .map { it[Keys.serverUrl] }
        .stateIn(scope, SharingStarted.Eagerly, null)

    val sessionToken: StateFlow<String?> = store.data
        .map { it[Keys.sessionToken] }
        .stateIn(scope, SharingStarted.Eagerly, null)

    suspend fun setServerUrl(value: String) {
        store.edit { it[Keys.serverUrl] = value.trim().trimEnd('/') }
    }

    suspend fun setSessionToken(value: String?) {
        store.edit {
            if (value.isNullOrEmpty()) it.remove(Keys.sessionToken) else it[Keys.sessionToken] = value
        }
    }

    suspend fun clearSession() {
        store.edit { it.remove(Keys.sessionToken) }
    }
}
