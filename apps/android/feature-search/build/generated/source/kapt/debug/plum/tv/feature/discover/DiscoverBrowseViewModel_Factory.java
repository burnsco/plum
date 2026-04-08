package plum.tv.feature.discover;

import dagger.internal.DaggerGenerated;
import dagger.internal.Factory;
import dagger.internal.QualifierMetadata;
import dagger.internal.ScopeMetadata;
import javax.annotation.processing.Generated;
import javax.inject.Provider;
import plum.tv.core.data.DiscoverRepository;

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
public final class DiscoverBrowseViewModel_Factory implements Factory<DiscoverBrowseViewModel> {
  private final Provider<DiscoverRepository> repositoryProvider;

  public DiscoverBrowseViewModel_Factory(Provider<DiscoverRepository> repositoryProvider) {
    this.repositoryProvider = repositoryProvider;
  }

  @Override
  public DiscoverBrowseViewModel get() {
    return newInstance(repositoryProvider.get());
  }

  public static DiscoverBrowseViewModel_Factory create(
      Provider<DiscoverRepository> repositoryProvider) {
    return new DiscoverBrowseViewModel_Factory(repositoryProvider);
  }

  public static DiscoverBrowseViewModel newInstance(DiscoverRepository repository) {
    return new DiscoverBrowseViewModel(repository);
  }
}
