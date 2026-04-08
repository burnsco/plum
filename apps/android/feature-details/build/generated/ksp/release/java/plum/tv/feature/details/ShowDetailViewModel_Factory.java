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
public final class ShowDetailViewModel_Factory implements Factory<ShowDetailViewModel> {
  private final Provider<BrowseRepository> browseRepositoryProvider;

  private final Provider<LibraryCatalogRefreshCoordinator> catalogRefreshCoordinatorProvider;

  private final Provider<SavedStateHandle> savedStateHandleProvider;

  private ShowDetailViewModel_Factory(Provider<BrowseRepository> browseRepositoryProvider,
      Provider<LibraryCatalogRefreshCoordinator> catalogRefreshCoordinatorProvider,
      Provider<SavedStateHandle> savedStateHandleProvider) {
    this.browseRepositoryProvider = browseRepositoryProvider;
    this.catalogRefreshCoordinatorProvider = catalogRefreshCoordinatorProvider;
    this.savedStateHandleProvider = savedStateHandleProvider;
  }

  @Override
  public ShowDetailViewModel get() {
    return newInstance(browseRepositoryProvider.get(), catalogRefreshCoordinatorProvider.get(), savedStateHandleProvider.get());
  }

  public static ShowDetailViewModel_Factory create(
      Provider<BrowseRepository> browseRepositoryProvider,
      Provider<LibraryCatalogRefreshCoordinator> catalogRefreshCoordinatorProvider,
      Provider<SavedStateHandle> savedStateHandleProvider) {
    return new ShowDetailViewModel_Factory(browseRepositoryProvider, catalogRefreshCoordinatorProvider, savedStateHandleProvider);
  }

  public static ShowDetailViewModel newInstance(BrowseRepository browseRepository,
      LibraryCatalogRefreshCoordinator catalogRefreshCoordinator,
      SavedStateHandle savedStateHandle) {
    return new ShowDetailViewModel(browseRepository, catalogRefreshCoordinator, savedStateHandle);
  }
}
