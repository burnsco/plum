package plum.tv.core.data;

import dagger.internal.DaggerGenerated;
import dagger.internal.Factory;
import dagger.internal.QualifierMetadata;
import dagger.internal.ScopeMetadata;
import javax.annotation.processing.Generated;
import javax.inject.Provider;

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
public final class PlaybackRepository_Factory implements Factory<PlaybackRepository> {
  private final Provider<SessionRepository> sessionRepositoryProvider;

  public PlaybackRepository_Factory(Provider<SessionRepository> sessionRepositoryProvider) {
    this.sessionRepositoryProvider = sessionRepositoryProvider;
  }

  @Override
  public PlaybackRepository get() {
    return newInstance(sessionRepositoryProvider.get());
  }

  public static PlaybackRepository_Factory create(
      Provider<SessionRepository> sessionRepositoryProvider) {
    return new PlaybackRepository_Factory(sessionRepositoryProvider);
  }

  public static PlaybackRepository newInstance(SessionRepository sessionRepository) {
    return new PlaybackRepository(sessionRepository);
  }
}
