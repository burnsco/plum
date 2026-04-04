package plum.tv.app

import android.app.ActivityManager
import android.app.Application
import android.content.Context
import android.util.Log
import coil3.ImageLoader
import coil3.SingletonImageLoader
import coil3.disk.DiskCache
import coil3.memory.MemoryCache
import coil3.network.okhttp.OkHttpNetworkFetcherFactory
import dagger.hilt.EntryPoint
import dagger.hilt.EntryPoints
import dagger.hilt.InstallIn
import dagger.hilt.android.HiltAndroidApp
import dagger.hilt.components.SingletonComponent
import java.io.File
import okhttp3.OkHttpClient
import okio.Path.Companion.toOkioPath

@HiltAndroidApp
class PlumTvApplication : Application() {
    @EntryPoint
    @InstallIn(SingletonComponent::class)
    interface CoilEntryPoint {
        fun okHttpClient(): OkHttpClient
    }

    override fun onCreate() {
        super.onCreate()
        Log.i("PlumTV", "application start")
        SingletonImageLoader.setSafe { context: Context ->
            val ep = EntryPoints.get(this@PlumTvApplication, CoilEntryPoint::class.java)
            val lowRam = context.getSystemService(ActivityManager::class.java)?.isLowRamDevice == true
            val memoryCachePercent = if (lowRam) 0.12 else 0.28
            val diskCacheSizeBytes = if (lowRam) 128L * 1024 * 1024 else 512L * 1024 * 1024
            ImageLoader.Builder(context)
                .components {
                    add(OkHttpNetworkFetcherFactory(callFactory = { ep.okHttpClient() }))
                }
                .memoryCache {
                    MemoryCache.Builder()
                        .maxSizePercent(context, memoryCachePercent)
                        .build()
                }
                .diskCache {
                    DiskCache.Builder()
                        .directory(File(context.cacheDir, "plum_coil_disk").apply { mkdirs() }.toOkioPath())
                        .maxSizeBytes(diskCacheSizeBytes)
                        .build()
                }
                .build()
        }
    }
}
