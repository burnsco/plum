package plum.tv.core.data;

import dagger.internal.DaggerGenerated;
import dagger.internal.Factory;
import dagger.internal.Provider;
import dagger.internal.QualifierMetadata;
import dagger.internal.ScopeMetadata;
import javax.annotation.processing.Generated;

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
public final class LibraryScanStatusPoller_Factory implements Factory<LibraryScanStatusPoller> {
  private final Provider<SessionRepository> sessionRepositoryProvider;

  private final Provider<LibraryCatalogRefreshCoordinator> catalogRefreshCoordinatorProvider;

  private LibraryScanStatusPoller_Factory(Provider<SessionRepository> sessionRepositoryProvider,
      Provider<LibraryCatalogRefreshCoordinator> catalogRefreshCoordinatorProvider) {
    this.sessionRepositoryProvider = sessionRepositoryProvider;
    this.catalogRefreshCoordinatorProvider = catalogRefreshCoordinatorProvider;
  }

  @Override
  public LibraryScanStatusPoller get() {
    return newInstance(sessionRepositoryProvider.get(), catalogRefreshCoordinatorProvider.get());
  }

  public static LibraryScanStatusPoller_Factory create(
      Provider<SessionRepository> sessionRepositoryProvider,
      Provider<LibraryCatalogRefreshCoordinator> catalogRefreshCoordinatorProvider) {
    return new LibraryScanStatusPoller_Factory(sessionRepositoryProvider, catalogRefreshCoordinatorProvider);
  }

  public static LibraryScanStatusPoller newInstance(SessionRepository sessionRepository,
      LibraryCatalogRefreshCoordinator catalogRefreshCoordinator) {
    return new LibraryScanStatusPoller(sessionRepository, catalogRefreshCoordinator);
  }
}
