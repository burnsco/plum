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
public final class BrowseRepository_Factory implements Factory<BrowseRepository> {
  private final Provider<SessionRepository> sessionRepositoryProvider;

  public BrowseRepository_Factory(Provider<SessionRepository> sessionRepositoryProvider) {
    this.sessionRepositoryProvider = sessionRepositoryProvider;
  }

  @Override
  public BrowseRepository get() {
    return newInstance(sessionRepositoryProvider.get());
  }

  public static BrowseRepository_Factory create(
      Provider<SessionRepository> sessionRepositoryProvider) {
    return new BrowseRepository_Factory(sessionRepositoryProvider);
  }

  public static BrowseRepository newInstance(SessionRepository sessionRepository) {
    return new BrowseRepository(sessionRepository);
  }
}
