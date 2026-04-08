package plum.tv.feature.details;

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
public final class MovieDetailViewModel_Factory implements Factory<MovieDetailViewModel> {
  private final Provider<BrowseRepository> browseRepositoryProvider;

  private final Provider<LibraryCatalogRefreshCoordinator> catalogRefreshCoordinatorProvider;

  private final Provider<SavedStateHandle> savedStateHandleProvider;

  private MovieDetailViewModel_Factory(Provider<BrowseRepository> browseRepositoryProvider,
      Provider<LibraryCatalogRefreshCoordinator> catalogRefreshCoordinatorProvider,
      Provider<SavedStateHandle> savedStateHandleProvider) {
    this.browseRepositoryProvider = browseRepositoryProvider;
    this.catalogRefreshCoordinatorProvider = catalogRefreshCoordinatorProvider;
    this.savedStateHandleProvider = savedStateHandleProvider;
  }

  @Override
  public MovieDetailViewModel get() {
    return newInstance(browseRepositoryProvider.get(), catalogRefreshCoordinatorProvider.get(), savedStateHandleProvider.get());
  }

  public static MovieDetailViewModel_Factory create(
      Provider<BrowseRepository> browseRepositoryProvider,
      Provider<LibraryCatalogRefreshCoordinator> catalogRefreshCoordinatorProvider,
      Provider<SavedStateHandle> savedStateHandleProvider) {
    return new MovieDetailViewModel_Factory(browseRepositoryProvider, catalogRefreshCoordinatorProvider, savedStateHandleProvider);
  }

  public static MovieDetailViewModel newInstance(BrowseRepository browseRepository,
      LibraryCatalogRefreshCoordinator catalogRefreshCoordinator,
      SavedStateHandle savedStateHandle) {
    return new MovieDetailViewModel(browseRepository, catalogRefreshCoordinator, savedStateHandle);
  }
}
