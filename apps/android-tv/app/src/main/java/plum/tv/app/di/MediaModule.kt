package plum.tv.app.di

import android.app.ActivityManager
import android.content.Context
import androidx.media3.common.util.UnstableApi
import androidx.media3.database.StandaloneDatabaseProvider
import androidx.media3.datasource.DataSource
import androidx.media3.datasource.FileDataSource
import androidx.media3.datasource.cache.Cache
import androidx.media3.datasource.cache.CacheDataSink
import androidx.media3.datasource.cache.CacheDataSource
import androidx.media3.datasource.cache.LeastRecentlyUsedCacheEvictor
import androidx.media3.datasource.cache.SimpleCache
import androidx.media3.datasource.okhttp.OkHttpDataSource
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.android.qualifiers.ApplicationContext
import dagger.hilt.components.SingletonComponent
import javax.inject.Singleton
import okhttp3.OkHttpClient
import java.io.File
import java.util.concurrent.TimeUnit
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import plum.tv.core.data.di.ApplicationScope

@Module
@InstallIn(SingletonComponent::class)
@UnstableApi
object MediaModule {
    private const val MEDIA_CACHE_SIZE_LOW_RAM_BYTES = 128L * 1024 * 1024
    private const val MEDIA_CACHE_SIZE_NORMAL_BYTES = 256L * 1024 * 1024

    @Provides
    @Singleton
    @ApplicationScope
    fun provideApplicationScope(): CoroutineScope =
        CoroutineScope(SupervisorJob() + Dispatchers.Default)

    @Provides
    @Singleton
    fun providePlumMediaCache(
        @ApplicationContext context: Context,
    ): Cache {
        val lowRam = context.getSystemService(ActivityManager::class.java)?.isLowRamDevice == true
        val cacheSizeBytes = if (lowRam) MEDIA_CACHE_SIZE_LOW_RAM_BYTES else MEDIA_CACHE_SIZE_NORMAL_BYTES
        return SimpleCache(
            File(context.cacheDir, "plum_media_cache").apply { mkdirs() },
            LeastRecentlyUsedCacheEvictor(cacheSizeBytes),
            StandaloneDatabaseProvider(context),
        )
    }

    @Provides
    @Singleton
    fun providePlumMediaDataSourceFactory(
        okHttpClient: OkHttpClient,
        mediaCache: Cache,
    ): DataSource.Factory {
        val mediaClient =
            okHttpClient.newBuilder()
                .connectTimeout(15, TimeUnit.SECONDS)
                // Media streams can take a while to deliver first bytes or pause between chunks.
                .readTimeout(0, TimeUnit.MILLISECONDS)
                .callTimeout(0, TimeUnit.MILLISECONDS)
                .build()

        val upstreamFactory = OkHttpDataSource.Factory(mediaClient)
        val cacheWriteFactory =
            CacheDataSink.Factory()
                .setCache(mediaCache)
        return CacheDataSource.Factory()
            .setCache(mediaCache)
            .setCacheReadDataSourceFactory(FileDataSource.Factory())
            .setCacheWriteDataSinkFactory(cacheWriteFactory)
            .setUpstreamDataSourceFactory(upstreamFactory)
            .setFlags(CacheDataSource.FLAG_IGNORE_CACHE_ON_ERROR)
    }
}
