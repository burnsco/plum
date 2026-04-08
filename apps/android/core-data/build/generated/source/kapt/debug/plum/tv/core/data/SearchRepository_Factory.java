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
public final class SearchRepository_Factory implements Factory<SearchRepository> {
  private final Provider<SessionRepository> sessionRepositoryProvider;

  public SearchRepository_Factory(Provider<SessionRepository> sessionRepositoryProvider) {
    this.sessionRepositoryProvider = sessionRepositoryProvider;
  }

  @Override
  public SearchRepository get() {
    return newInstance(sessionRepositoryProvider.get());
  }

  public static SearchRepository_Factory create(
      Provider<SessionRepository> sessionRepositoryProvider) {
    return new SearchRepository_Factory(sessionRepositoryProvider);
  }

  public static SearchRepository newInstance(SessionRepository sessionRepository) {
    return new SearchRepository(sessionRepository);
  }
}
