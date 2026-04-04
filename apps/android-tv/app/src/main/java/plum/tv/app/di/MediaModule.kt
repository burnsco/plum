package plum.tv.app.di

import androidx.media3.common.util.UnstableApi
import androidx.media3.datasource.DataSource
import androidx.media3.datasource.okhttp.OkHttpDataSource
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import javax.inject.Singleton
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import okhttp3.OkHttpClient
import java.util.concurrent.TimeUnit
import plum.tv.core.data.di.ApplicationScope

@Module
@InstallIn(SingletonComponent::class)
@UnstableApi
object MediaModule {
    private val mediaReadTimeoutMs = TimeUnit.MINUTES.toMillis(10)
    private val mediaCallTimeoutMs = TimeUnit.MINUTES.toMillis(15)

    @Provides
    @Singleton
    @ApplicationScope
    fun provideApplicationScope(): CoroutineScope =
        CoroutineScope(SupervisorJob() + Dispatchers.Default)

    /**
     * ExoPlayer reads HLS playlists and many segment GETs through this factory.
     *
     * We intentionally **do not** wrap [OkHttpDataSource] with [androidx.media3.datasource.cache.CacheDataSource]:
     * SimpleCache + live HLS (revolving playlists, auth on every segment, varying URLs) has repeatedly
     * surfaced as [androidx.media3.common.PlaybackException.ERROR_CODE_IO_UNSPECIFIED] in the field.
     * OkHttp's response [okhttp3.Cache] is also disabled on the clone below — large streaming bodies
     * must not be written to the HTTP disk cache.
     *
     * Read and call timeouts are long but finite so a dead TCP session cannot hang the player forever.
     */
    @Provides
    @Singleton
    fun providePlumMediaDataSourceFactory(
        okHttpClient: OkHttpClient,
    ): DataSource.Factory {
        val mediaClient =
            okHttpClient.newBuilder()
                .cache(null)
                .connectTimeout(20, TimeUnit.SECONDS)
                .readTimeout(mediaReadTimeoutMs, TimeUnit.MILLISECONDS)
                .callTimeout(mediaCallTimeoutMs, TimeUnit.MILLISECONDS)
                .build()

        return OkHttpDataSource.Factory(mediaClient)
    }
}
