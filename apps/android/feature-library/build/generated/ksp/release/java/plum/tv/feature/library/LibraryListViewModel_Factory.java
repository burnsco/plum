package plum.tv.feature.library;

import dagger.internal.DaggerGenerated;
import dagger.internal.Factory;
import dagger.internal.Provider;
import dagger.internal.QualifierMetadata;
import dagger.internal.ScopeMetadata;
import javax.annotation.processing.Generated;
import plum.tv.core.data.BrowseRepository;
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
public final class LibraryListViewModel_Factory implements Factory<LibraryListViewModel> {
  private final Provider<BrowseRepository> browseRepositoryProvider;

  private final Provider<LibraryCatalogRefreshCoordinator> catalogRefreshCoordinatorProvider;

  private LibraryListViewModel_Factory(Provider<BrowseRepository> browseRepositoryProvider,
      Provider<LibraryCatalogRefreshCoordinator> catalogRefreshCoordinatorProvider) {
    this.browseRepositoryProvider = browseRepositoryProvider;
    this.catalogRefreshCoordinatorProvider = catalogRefreshCoordinatorProvider;
  }

  @Override
  public LibraryListViewModel get() {
    return newInstance(browseRepositoryProvider.get(), catalogRefreshCoordinatorProvider.get());
  }

  public static LibraryListViewModel_Factory create(
      Provider<BrowseRepository> browseRepositoryProvider,
      Provider<LibraryCatalogRefreshCoordinator> catalogRefreshCoordinatorProvider) {
    return new LibraryListViewModel_Factory(browseRepositoryProvider, catalogRefreshCoordinatorProvider);
  }

  public static LibraryListViewModel newInstance(BrowseRepository browseRepository,
      LibraryCatalogRefreshCoordinator catalogRefreshCoordinator) {
    return new LibraryListViewModel(browseRepository, catalogRefreshCoordinator);
  }
}
