package plum.tv.core.data;

import dagger.internal.DaggerGenerated;
import dagger.internal.Factory;
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
public final class PlaybackRepository_Factory implements Factory<PlaybackRepository> {
  private final Provider<SessionRepository> sessionRepositoryProvider;

  private final Provider<OkHttpClient> okHttpClientProvider;

  private PlaybackRepository_Factory(Provider<SessionRepository> sessionRepositoryProvider,
      Provider<OkHttpClient> okHttpClientProvider) {
    this.sessionRepositoryProvider = sessionRepositoryProvider;
    this.okHttpClientProvider = okHttpClientProvider;
  }

  @Override
  public PlaybackRepository get() {
    return newInstance(sessionRepositoryProvider.get(), okHttpClientProvider.get());
  }

  public static PlaybackRepository_Factory create(
      Provider<SessionRepository> sessionRepositoryProvider,
      Provider<OkHttpClient> okHttpClientProvider) {
    return new PlaybackRepository_Factory(sessionRepositoryProvider, okHttpClientProvider);
  }

  public static PlaybackRepository newInstance(SessionRepository sessionRepository,
      OkHttpClient okHttpClient) {
    return new PlaybackRepository(sessionRepository, okHttpClient);
  }
}
