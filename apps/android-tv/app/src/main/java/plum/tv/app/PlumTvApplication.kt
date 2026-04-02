package plum.tv.app

import android.app.Application
import coil.ImageLoader
import coil.ImageLoaderFactory
import dagger.hilt.EntryPoint
import dagger.hilt.EntryPoints
import dagger.hilt.InstallIn
import dagger.hilt.android.HiltAndroidApp
import dagger.hilt.components.SingletonComponent
import okhttp3.OkHttpClient

@HiltAndroidApp
class PlumTvApplication : Application(), ImageLoaderFactory {

    @EntryPoint
    @InstallIn(SingletonComponent::class)
    interface CoilEntryPoint {
        fun okHttpClient(): OkHttpClient
    }

    override fun newImageLoader(): ImageLoader {
        val ep = EntryPoints.get(this, CoilEntryPoint::class.java)
        return ImageLoader.Builder(this)
            .callFactory(ep.okHttpClient())
            .crossfade(true)
            .build()
    }
}
