package plum.tv.feature.library;

import dagger.internal.DaggerGenerated;
import dagger.internal.Factory;
import dagger.internal.QualifierMetadata;
import dagger.internal.ScopeMetadata;
import javax.annotation.processing.Generated;
import javax.inject.Provider;
import plum.tv.core.data.BrowseRepository;

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
    "cast"
})
public final class LibraryListViewModel_Factory implements Factory<LibraryListViewModel> {
  private final Provider<BrowseRepository> browseRepositoryProvider;

  public LibraryListViewModel_Factory(Provider<BrowseRepository> browseRepositoryProvider) {
    this.browseRepositoryProvider = browseRepositoryProvider;
  }

  @Override
  public LibraryListViewModel get() {
    return newInstance(browseRepositoryProvider.get());
  }

  public static LibraryListViewModel_Factory create(
      Provider<BrowseRepository> browseRepositoryProvider) {
    return new LibraryListViewModel_Factory(browseRepositoryProvider);
  }

  public static LibraryListViewModel newInstance(BrowseRepository browseRepository) {
    return new LibraryListViewModel(browseRepository);
  }
}
