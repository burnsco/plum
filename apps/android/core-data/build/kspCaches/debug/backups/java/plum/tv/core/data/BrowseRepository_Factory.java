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
public final class BrowseRepository_Factory implements Factory<BrowseRepository> {
  private final Provider<SessionRepository> sessionRepositoryProvider;

  private final Provider<HomeDashboardDiskCache> homeDashboardDiskCacheProvider;

  private BrowseRepository_Factory(Provider<SessionRepository> sessionRepositoryProvider,
      Provider<HomeDashboardDiskCache> homeDashboardDiskCacheProvider) {
    this.sessionRepositoryProvider = sessionRepositoryProvider;
    this.homeDashboardDiskCacheProvider = homeDashboardDiskCacheProvider;
  }

  @Override
  public BrowseRepository get() {
    return newInstance(sessionRepositoryProvider.get(), homeDashboardDiskCacheProvider.get());
  }

  public static BrowseRepository_Factory create(
      Provider<SessionRepository> sessionRepositoryProvider,
      Provider<HomeDashboardDiskCache> homeDashboardDiskCacheProvider) {
    return new BrowseRepository_Factory(sessionRepositoryProvider, homeDashboardDiskCacheProvider);
  }

  public static BrowseRepository newInstance(SessionRepository sessionRepository,
      HomeDashboardDiskCache homeDashboardDiskCache) {
    return new BrowseRepository(sessionRepository, homeDashboardDiskCache);
  }
}
