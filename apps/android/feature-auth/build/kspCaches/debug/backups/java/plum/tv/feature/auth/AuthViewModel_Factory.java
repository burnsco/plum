package plum.tv.feature.auth;

import dagger.internal.DaggerGenerated;
import dagger.internal.Factory;
import dagger.internal.Provider;
import dagger.internal.QualifierMetadata;
import dagger.internal.ScopeMetadata;
import javax.annotation.processing.Generated;
import plum.tv.core.data.BrowseRepository;
import plum.tv.core.data.LibraryCatalogRefreshCoordinator;
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
    "cast",
    "deprecation",
    "nullness:initialization.field.uninitialized"
})
public final class AuthViewModel_Factory implements Factory<AuthViewModel> {
  private final Provider<SessionRepository> sessionRepositoryProvider;

  private final Provider<BrowseRepository> browseRepositoryProvider;

  private final Provider<LibraryCatalogRefreshCoordinator> catalogRefreshCoordinatorProvider;

  private AuthViewModel_Factory(Provider<SessionRepository> sessionRepositoryProvider,
      Provider<BrowseRepository> browseRepositoryProvider,
      Provider<LibraryCatalogRefreshCoordinator> catalogRefreshCoordinatorProvider) {
    this.sessionRepositoryProvider = sessionRepositoryProvider;
    this.browseRepositoryProvider = browseRepositoryProvider;
    this.catalogRefreshCoordinatorProvider = catalogRefreshCoordinatorProvider;
  }

  @Override
  public AuthViewModel get() {
    return newInstance(sessionRepositoryProvider.get(), browseRepositoryProvider.get(), catalogRefreshCoordinatorProvider.get());
  }

  public static AuthViewModel_Factory create(Provider<SessionRepository> sessionRepositoryProvider,
      Provider<BrowseRepository> browseRepositoryProvider,
      Provider<LibraryCatalogRefreshCoordinator> catalogRefreshCoordinatorProvider) {
    return new AuthViewModel_Factory(sessionRepositoryProvider, browseRepositoryProvider, catalogRefreshCoordinatorProvider);
  }

  public static AuthViewModel newInstance(SessionRepository sessionRepository,
      BrowseRepository browseRepository,
      LibraryCatalogRefreshCoordinator catalogRefreshCoordinator) {
    return new AuthViewModel(sessionRepository, browseRepository, catalogRefreshCoordinator);
  }
}
