package plum.tv.app;

import dagger.internal.DaggerGenerated;
import dagger.internal.Factory;
import dagger.internal.Provider;
import dagger.internal.QualifierMetadata;
import dagger.internal.ScopeMetadata;
import javax.annotation.processing.Generated;
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
    "cast",
    "deprecation",
    "nullness:initialization.field.uninitialized"
})
public final class MainNavViewModel_Factory implements Factory<MainNavViewModel> {
  private final Provider<BrowseRepository> browseRepositoryProvider;

  private MainNavViewModel_Factory(Provider<BrowseRepository> browseRepositoryProvider) {
    this.browseRepositoryProvider = browseRepositoryProvider;
  }

  @Override
  public MainNavViewModel get() {
    return newInstance(browseRepositoryProvider.get());
  }

  public static MainNavViewModel_Factory create(
      Provider<BrowseRepository> browseRepositoryProvider) {
    return new MainNavViewModel_Factory(browseRepositoryProvider);
  }

  public static MainNavViewModel newInstance(BrowseRepository browseRepository) {
    return new MainNavViewModel(browseRepository);
  }
}
