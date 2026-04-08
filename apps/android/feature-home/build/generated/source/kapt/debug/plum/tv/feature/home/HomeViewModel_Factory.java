package plum.tv.feature.home;

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
public final class HomeViewModel_Factory implements Factory<HomeViewModel> {
  private final Provider<BrowseRepository> browseRepositoryProvider;

  public HomeViewModel_Factory(Provider<BrowseRepository> browseRepositoryProvider) {
    this.browseRepositoryProvider = browseRepositoryProvider;
  }

  @Override
  public HomeViewModel get() {
    return newInstance(browseRepositoryProvider.get());
  }

  public static HomeViewModel_Factory create(Provider<BrowseRepository> browseRepositoryProvider) {
    return new HomeViewModel_Factory(browseRepositoryProvider);
  }

  public static HomeViewModel newInstance(BrowseRepository browseRepository) {
    return new HomeViewModel(browseRepository);
  }
}
