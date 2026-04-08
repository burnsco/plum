package plum.tv.core.data

import android.content.Context
import com.squareup.moshi.Moshi
import dagger.hilt.android.qualifiers.ApplicationContext
import java.io.File
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import plum.tv.core.network.CachedHomeDashboardEnvelope
import plum.tv.core.network.HomeDashboardJson

@Singleton
class HomeDashboardDiskCache @Inject constructor(
    @ApplicationContext context: Context,
    moshi: Moshi,
) {
    private val file = File(context.cacheDir, "plum_home_dashboard_cache.json")
    private val adapter = moshi.adapter(CachedHomeDashboardEnvelope::class.java)
    private val lock = Any()

    fun clear() {
        synchronized(lock) {
            if (file.exists()) {
                file.delete()
            }
        }
    }

    suspend fun read(serverUrl: String): HomeDashboardJson? =
        withContext(Dispatchers.IO) {
            val env =
                synchronized(lock) {
                    if (!file.exists()) return@withContext null
                    runCatching { adapter.fromJson(file.readText()) }.getOrNull()
                } ?: return@withContext null
            val normalized = serverUrl.trim().trimEnd('/')
            if (env.serverUrl != normalized) return@withContext null
            env.dashboard
        }

    suspend fun write(serverUrl: String, dashboard: HomeDashboardJson) =
        withContext(Dispatchers.IO) {
            val normalized = serverUrl.trim().trimEnd('/')
            val json =
                adapter.toJson(
                    CachedHomeDashboardEnvelope(serverUrl = normalized, dashboard = dashboard),
                )
            synchronized(lock) {
                file.writeText(json)
            }
        }
}
