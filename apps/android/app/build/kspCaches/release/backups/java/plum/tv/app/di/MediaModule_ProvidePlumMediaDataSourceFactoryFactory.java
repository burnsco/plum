package plum.tv.app.di;

import androidx.media3.datasource.DataSource;
import dagger.internal.DaggerGenerated;
import dagger.internal.Factory;
import dagger.internal.Preconditions;
import dagger.internal.Provider;
import dagger.internal.QualifierMetadata;
import dagger.internal.ScopeMetadata;
import javax.annotation.processing.Generated;
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
    "cast",
    "deprecation",
    "nullness:initialization.field.uninitialized"
})
public final class MediaModule_ProvidePlumMediaDataSourceFactoryFactory implements Factory<DataSource.Factory> {
  private final Provider<OkHttpClient> okHttpClientProvider;

  private MediaModule_ProvidePlumMediaDataSourceFactoryFactory(
      Provider<OkHttpClient> okHttpClientProvider) {
    this.okHttpClientProvider = okHttpClientProvider;
  }

  @Override
  public DataSource.Factory get() {
    return providePlumMediaDataSourceFactory(okHttpClientProvider.get());
  }

  public static MediaModule_ProvidePlumMediaDataSourceFactoryFactory create(
      Provider<OkHttpClient> okHttpClientProvider) {
    return new MediaModule_ProvidePlumMediaDataSourceFactoryFactory(okHttpClientProvider);
  }

  public static DataSource.Factory providePlumMediaDataSourceFactory(OkHttpClient okHttpClient) {
    return Preconditions.checkNotNullFromProvides(MediaModule.INSTANCE.providePlumMediaDataSourceFactory(okHttpClient));
  }
}
