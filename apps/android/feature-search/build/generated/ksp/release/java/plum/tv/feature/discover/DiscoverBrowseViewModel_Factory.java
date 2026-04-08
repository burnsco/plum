package plum.tv.feature.discover;

import dagger.internal.DaggerGenerated;
import dagger.internal.Factory;
import dagger.internal.Provider;
import dagger.internal.QualifierMetadata;
import dagger.internal.ScopeMetadata;
import javax.annotation.processing.Generated;
import plum.tv.core.data.DiscoverRepository;
import plum.tv.core.data.LibraryCatalogRefreshCoordinator;

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
public final class DiscoverBrowseViewModel_Factory implements Factory<DiscoverBrowseViewModel> {
  private final Provider<DiscoverRepository> repositoryProvider;

  private final Provider<LibraryCatalogRefreshCoordinator> catalogRefreshCoordinatorProvider;

  private DiscoverBrowseViewModel_Factory(Provider<DiscoverRepository> repositoryProvider,
      Provider<LibraryCatalogRefreshCoordinator> catalogRefreshCoordinatorProvider) {
    this.repositoryProvider = repositoryProvider;
    this.catalogRefreshCoordinatorProvider = catalogRefreshCoordinatorProvider;
  }

  @Override
  public DiscoverBrowseViewModel get() {
    return newInstance(repositoryProvider.get(), catalogRefreshCoordinatorProvider.get());
  }

  public static DiscoverBrowseViewModel_Factory create(
      Provider<DiscoverRepository> repositoryProvider,
      Provider<LibraryCatalogRefreshCoordinator> catalogRefreshCoordinatorProvider) {
    return new DiscoverBrowseViewModel_Factory(repositoryProvider, catalogRefreshCoordinatorProvider);
  }

  public static DiscoverBrowseViewModel newInstance(DiscoverRepository repository,
      LibraryCatalogRefreshCoordinator catalogRefreshCoordinator) {
    return new DiscoverBrowseViewModel(repository, catalogRefreshCoordinator);
  }
}
