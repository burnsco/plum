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
public final class LibraryCatalogRefreshCoordinator_Factory implements Factory<LibraryCatalogRefreshCoordinator> {
  private final Provider<BrowseRepository> browseRepositoryProvider;

  private LibraryCatalogRefreshCoordinator_Factory(
      Provider<BrowseRepository> browseRepositoryProvider) {
    this.browseRepositoryProvider = browseRepositoryProvider;
  }

  @Override
  public LibraryCatalogRefreshCoordinator get() {
    return newInstance(browseRepositoryProvider.get());
  }

  public static LibraryCatalogRefreshCoordinator_Factory create(
      Provider<BrowseRepository> browseRepositoryProvider) {
    return new LibraryCatalogRefreshCoordinator_Factory(browseRepositoryProvider);
  }

  public static LibraryCatalogRefreshCoordinator newInstance(BrowseRepository browseRepository) {
    return new LibraryCatalogRefreshCoordinator(browseRepository);
  }
}
