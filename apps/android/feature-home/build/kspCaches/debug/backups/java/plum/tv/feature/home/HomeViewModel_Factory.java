package plum.tv.feature.home;

import dagger.internal.DaggerGenerated;
import dagger.internal.Factory;
import dagger.internal.Provider;
import dagger.internal.QualifierMetadata;
import dagger.internal.ScopeMetadata;
import javax.annotation.processing.Generated;
import plum.tv.core.data.BrowseRepository;
import plum.tv.core.data.HomeDashboardDiskCache;
import plum.tv.core.data.LibraryCatalogRefreshCoordinator;
import plum.tv.core.data.SessionPreferences;

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
public final class HomeViewModel_Factory implements Factory<HomeViewModel> {
  private final Provider<BrowseRepository> browseRepositoryProvider;

  private final Provider<HomeDashboardDiskCache> homeDashboardDiskCacheProvider;

  private final Provider<SessionPreferences> sessionPreferencesProvider;

  private final Provider<LibraryCatalogRefreshCoordinator> catalogRefreshCoordinatorProvider;

  private HomeViewModel_Factory(Provider<BrowseRepository> browseRepositoryProvider,
      Provider<HomeDashboardDiskCache> homeDashboardDiskCacheProvider,
      Provider<SessionPreferences> sessionPreferencesProvider,
      Provider<LibraryCatalogRefreshCoordinator> catalogRefreshCoordinatorProvider) {
    this.browseRepositoryProvider = browseRepositoryProvider;
    this.homeDashboardDiskCacheProvider = homeDashboardDiskCacheProvider;
    this.sessionPreferencesProvider = sessionPreferencesProvider;
    this.catalogRefreshCoordinatorProvider = catalogRefreshCoordinatorProvider;
  }

  @Override
  public HomeViewModel get() {
    return newInstance(browseRepositoryProvider.get(), homeDashboardDiskCacheProvider.get(), sessionPreferencesProvider.get(), catalogRefreshCoordinatorProvider.get());
  }

  public static HomeViewModel_Factory create(Provider<BrowseRepository> browseRepositoryProvider,
      Provider<HomeDashboardDiskCache> homeDashboardDiskCacheProvider,
      Provider<SessionPreferences> sessionPreferencesProvider,
      Provider<LibraryCatalogRefreshCoordinator> catalogRefreshCoordinatorProvider) {
    return new HomeViewModel_Factory(browseRepositoryProvider, homeDashboardDiskCacheProvider, sessionPreferencesProvider, catalogRefreshCoordinatorProvider);
  }

  public static HomeViewModel newInstance(BrowseRepository browseRepository,
      HomeDashboardDiskCache homeDashboardDiskCache, SessionPreferences sessionPreferences,
      LibraryCatalogRefreshCoordinator catalogRefreshCoordinator) {
    return new HomeViewModel(browseRepository, homeDashboardDiskCache, sessionPreferences, catalogRefreshCoordinator);
  }
}
