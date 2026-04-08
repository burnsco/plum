package plum.tv.feature.library;

import androidx.lifecycle.SavedStateHandle;
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
public final class LibraryBrowseViewModel_Factory implements Factory<LibraryBrowseViewModel> {
  private final Provider<BrowseRepository> browseRepositoryProvider;

  private final Provider<LibraryCatalogRefreshCoordinator> catalogRefreshCoordinatorProvider;

  private final Provider<SavedStateHandle> savedStateHandleProvider;

  private LibraryBrowseViewModel_Factory(Provider<BrowseRepository> browseRepositoryProvider,
      Provider<LibraryCatalogRefreshCoordinator> catalogRefreshCoordinatorProvider,
      Provider<SavedStateHandle> savedStateHandleProvider) {
    this.browseRepositoryProvider = browseRepositoryProvider;
    this.catalogRefreshCoordinatorProvider = catalogRefreshCoordinatorProvider;
    this.savedStateHandleProvider = savedStateHandleProvider;
  }

  @Override
  public LibraryBrowseViewModel get() {
    return newInstance(browseRepositoryProvider.get(), catalogRefreshCoordinatorProvider.get(), savedStateHandleProvider.get());
  }

  public static LibraryBrowseViewModel_Factory create(
      Provider<BrowseRepository> browseRepositoryProvider,
      Provider<LibraryCatalogRefreshCoordinator> catalogRefreshCoordinatorProvider,
      Provider<SavedStateHandle> savedStateHandleProvider) {
    return new LibraryBrowseViewModel_Factory(browseRepositoryProvider, catalogRefreshCoordinatorProvider, savedStateHandleProvider);
  }

  public static LibraryBrowseViewModel newInstance(BrowseRepository browseRepository,
      LibraryCatalogRefreshCoordinator catalogRefreshCoordinator,
      SavedStateHandle savedStateHandle) {
    return new LibraryBrowseViewModel(browseRepository, catalogRefreshCoordinator, savedStateHandle);
  }
}
