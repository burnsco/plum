package plum.tv.core.data.di

import android.app.ActivityManager
import android.content.Context
import android.os.SystemClock
import android.util.Log
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.android.qualifiers.ApplicationContext
import dagger.hilt.components.SingletonComponent
import javax.inject.Singleton
import okhttp3.Cache
import okhttp3.Interceptor
import okhttp3.OkHttpClient
import java.io.File
import java.util.concurrent.TimeUnit
import plum.tv.core.data.AuthTokenBridge

@Module
@InstallIn(SingletonComponent::class)
object NetworkModule {
    private const val TAG = "PlumTV"
    private const val HTTP_CACHE_SIZE_LOW_RAM_BYTES = 32L * 1024 * 1024
    private const val HTTP_CACHE_SIZE_NORMAL_BYTES = 64L * 1024 * 1024

    @Provides
    @Singleton
    fun providePlumOkHttpClient(
        @ApplicationContext context: Context,
        bridge: AuthTokenBridge,
    ): OkHttpClient {
        val auth = Interceptor { chain ->
            val token = bridge.bearerToken()
            val req =
                if (token.isNullOrEmpty()) {
                    chain.request()
                } else {
                    chain.request().newBuilder().header("Authorization", "Bearer $token").build()
                }
            chain.proceed(req)
        }
        // /api/home and discover shelf/browse aggregate server-side and can be slow on large libraries;
        // all other endpoints should fail fast at 30s so the UI doesn't appear hung.
        val slowEndpoints = Interceptor { chain ->
            val path = chain.request().url.encodedPath
            val isSlowEndpoint =
                path.endsWith("/home") ||
                    path.contains("/home?") ||
                    path.endsWith("/discover") ||
                    path.startsWith("/api/discover/browse")
            if (isSlowEndpoint) {
                chain.withReadTimeout(90, TimeUnit.SECONDS).proceed(chain.request())
            } else {
                chain.proceed(chain.request())
            }
        }
        val httpCache = buildHttpCache(context)
        return OkHttpClient.Builder()
            .connectTimeout(20, TimeUnit.SECONDS)
            .readTimeout(30, TimeUnit.SECONDS)
            .writeTimeout(30, TimeUnit.SECONDS)
            .cache(httpCache)
            .addInterceptor(auth)
            .addInterceptor(slowEndpoints)
            .apply {
                val debugLoggingEnabled =
                    (context.applicationInfo.flags and android.content.pm.ApplicationInfo.FLAG_DEBUGGABLE) != 0
                if (debugLoggingEnabled) {
                    addInterceptor { chain ->
                        val request = chain.request()
                        val startedAt = SystemClock.elapsedRealtime()
                        Log.d(TAG, "http request method=${request.method} path=${request.url.encodedPath}")
                        try {
                            val response = chain.proceed(request)
                            val elapsedMs = SystemClock.elapsedRealtime() - startedAt
                            Log.d(
                                TAG,
                                "http response method=${request.method} path=${request.url.encodedPath} code=${response.code} durationMs=${elapsedMs}",
                            )
                            response
                        } catch (t: Throwable) {
                            val elapsedMs = SystemClock.elapsedRealtime() - startedAt
                            Log.w(
                                TAG,
                                "http failure method=${request.method} path=${request.url.encodedPath} durationMs=${elapsedMs} error=${t.message}",
                                t,
                            )
                            throw t
                        }
                    }
                }
            }
            .build()
    }

    private fun buildHttpCache(context: Context): Cache {
        val lowRam = context.getSystemService(ActivityManager::class.java)?.isLowRamDevice == true
        val sizeBytes = if (lowRam) HTTP_CACHE_SIZE_LOW_RAM_BYTES else HTTP_CACHE_SIZE_NORMAL_BYTES
        return Cache(File(context.cacheDir, "plum_http_cache"), sizeBytes)
    }
}
