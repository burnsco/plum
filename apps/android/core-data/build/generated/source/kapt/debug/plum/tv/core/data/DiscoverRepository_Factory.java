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
public final class DiscoverRepository_Factory implements Factory<DiscoverRepository> {
  private final Provider<SessionRepository> sessionRepositoryProvider;

  public DiscoverRepository_Factory(Provider<SessionRepository> sessionRepositoryProvider) {
    this.sessionRepositoryProvider = sessionRepositoryProvider;
  }

  @Override
  public DiscoverRepository get() {
    return newInstance(sessionRepositoryProvider.get());
  }

  public static DiscoverRepository_Factory create(
      Provider<SessionRepository> sessionRepositoryProvider) {
    return new DiscoverRepository_Factory(sessionRepositoryProvider);
  }

  public static DiscoverRepository newInstance(SessionRepository sessionRepository) {
    return new DiscoverRepository(sessionRepository);
  }
}
