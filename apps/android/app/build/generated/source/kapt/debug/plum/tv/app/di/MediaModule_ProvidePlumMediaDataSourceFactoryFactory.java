package plum.tv.app.di;

import androidx.media3.datasource.DataSource;
import androidx.media3.datasource.cache.Cache;
import dagger.internal.DaggerGenerated;
import dagger.internal.Factory;
import dagger.internal.Preconditions;
import dagger.internal.QualifierMetadata;
import dagger.internal.ScopeMetadata;
import javax.annotation.processing.Generated;
import javax.inject.Provider;
import okhttp3.OkHttpClient;

@ScopeMetadata("javax.inject.Singleton")
@QualifierMetadata
@DaggerGenerated
@Generated(
    value = "dagger.internal.codegen.ComponentProcessor",
    comments = "https://dagger.dev"
)
@SuppressWarnings({
    "unchecked",
    "rawtypes",
    "KotlinInternal",
    "KotlinInternalInJava",
    "cast"
})
public final class MediaModule_ProvidePlumMediaDataSourceFactoryFactory implements Factory<DataSource.Factory> {
  private final Provider<OkHttpClient> okHttpClientProvider;

  private final Provider<Cache> mediaCacheProvider;

  public MediaModule_ProvidePlumMediaDataSourceFactoryFactory(
      Provider<OkHttpClient> okHttpClientProvider, Provider<Cache> mediaCacheProvider) {
    this.okHttpClientProvider = okHttpClientProvider;
    this.mediaCacheProvider = mediaCacheProvider;
  }

  @Override
  public DataSource.Factory get() {
    return providePlumMediaDataSourceFactory(okHttpClientProvider.get(), mediaCacheProvider.get());
  }

  public static MediaModule_ProvidePlumMediaDataSourceFactoryFactory create(
      Provider<OkHttpClient> okHttpClientProvider, Provider<Cache> mediaCacheProvider) {
    return new MediaModule_ProvidePlumMediaDataSourceFactoryFactory(okHttpClientProvider, mediaCacheProvider);
  }

  public static DataSource.Factory providePlumMediaDataSourceFactory(OkHttpClient okHttpClient,
      Cache mediaCache) {
    return Preconditions.checkNotNullFromProvides(MediaModule.INSTANCE.providePlumMediaDataSourceFactory(okHttpClient, mediaCache));
  }
}
