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
public final class DiscoverDetailViewModel_Factory implements Factory<DiscoverDetailViewModel> {
  private final Provider<DiscoverRepository> repositoryProvider;

  public DiscoverDetailViewModel_Factory(Provider<DiscoverRepository> repositoryProvider) {
    this.repositoryProvider = repositoryProvider;
  }

  @Override
  public DiscoverDetailViewModel get() {
    return newInstance(repositoryProvider.get());
  }

  public static DiscoverDetailViewModel_Factory create(
      Provider<DiscoverRepository> repositoryProvider) {
    return new DiscoverDetailViewModel_Factory(repositoryProvider);
  }

  public static DiscoverDetailViewModel newInstance(DiscoverRepository repository) {
    return new DiscoverDetailViewModel(repository);
  }
}
