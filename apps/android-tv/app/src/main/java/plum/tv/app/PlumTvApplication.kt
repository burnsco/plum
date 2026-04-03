package plum.tv.app

import android.app.Application
import android.app.ActivityManager
import android.util.Log
import coil.ImageLoader
import coil.ImageLoaderFactory
import coil.disk.DiskCache
import coil.memory.MemoryCache
import dagger.hilt.EntryPoint
import dagger.hilt.EntryPoints
import dagger.hilt.InstallIn
import dagger.hilt.android.HiltAndroidApp
import dagger.hilt.components.SingletonComponent
import java.io.File
import okhttp3.OkHttpClient
import okio.Path.Companion.toOkioPath

@HiltAndroidApp
class PlumTvApplication : Application(), ImageLoaderFactory {
    override fun onCreate() {
        super.onCreate()
        Log.i("PlumTV", "application start")
    }

    @EntryPoint
    @InstallIn(SingletonComponent::class)
    interface CoilEntryPoint {
        fun okHttpClient(): OkHttpClient
    }

    override fun newImageLoader(): ImageLoader {
        val ep = EntryPoints.get(this, CoilEntryPoint::class.java)
        val lowRam = getSystemService(ActivityManager::class.java)?.isLowRamDevice == true
        val memoryCachePercent = if (lowRam) 0.12 else 0.22
        val diskCacheSizeBytes = if (lowRam) 128L * 1024 * 1024 else 256L * 1024 * 1024
        val allowHardwareBitmaps = !lowRam
        return ImageLoader.Builder(this)
            .okHttpClient(ep.okHttpClient())
            .allowHardware(allowHardwareBitmaps)
            .memoryCache {
                MemoryCache.Builder(this)
                    .maxSizePercent(memoryCachePercent)
                    .build()
            }
            .diskCache {
                DiskCache.Builder()
                    .directory(File(cacheDir, "plum_coil_disk").apply { mkdirs() }.toOkioPath())
                    .maxSizeBytes(diskCacheSizeBytes)
                    .build()
            }
            .crossfade(durationMillis = 100)
            .build()
    }
}
