package plum.tv.feature.auth;

import dagger.internal.DaggerGenerated;
import dagger.internal.Factory;
import dagger.internal.QualifierMetadata;
import dagger.internal.ScopeMetadata;
import javax.annotation.processing.Generated;
import javax.inject.Provider;
import plum.tv.core.data.BrowseRepository;
import plum.tv.core.data.SessionRepository;

@ScopeMetadata
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
public final class AuthViewModel_Factory implements Factory<AuthViewModel> {
  private final Provider<SessionRepository> sessionRepositoryProvider;

  private final Provider<BrowseRepository> browseRepositoryProvider;

  public AuthViewModel_Factory(Provider<SessionRepository> sessionRepositoryProvider,
      Provider<BrowseRepository> browseRepositoryProvider) {
    this.sessionRepositoryProvider = sessionRepositoryProvider;
    this.browseRepositoryProvider = browseRepositoryProvider;
  }

  @Override
  public AuthViewModel get() {
    return newInstance(sessionRepositoryProvider.get(), browseRepositoryProvider.get());
  }

  public static AuthViewModel_Factory create(Provider<SessionRepository> sessionRepositoryProvider,
      Provider<BrowseRepository> browseRepositoryProvider) {
    return new AuthViewModel_Factory(sessionRepositoryProvider, browseRepositoryProvider);
  }

  public static AuthViewModel newInstance(SessionRepository sessionRepository,
      BrowseRepository browseRepository) {
    return new AuthViewModel(sessionRepository, browseRepository);
  }
}
