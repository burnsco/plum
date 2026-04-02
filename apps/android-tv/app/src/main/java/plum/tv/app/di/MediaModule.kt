package plum.tv.app.di

import androidx.media3.datasource.DataSource
import androidx.media3.datasource.okhttp.OkHttpDataSource
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import javax.inject.Singleton
import okhttp3.OkHttpClient

@Module
@InstallIn(SingletonComponent::class)
object MediaModule {
    @Provides
    @Singleton
    fun providePlumMediaDataSourceFactory(okHttpClient: OkHttpClient): DataSource.Factory =
        OkHttpDataSource.Factory(okHttpClient)
}
