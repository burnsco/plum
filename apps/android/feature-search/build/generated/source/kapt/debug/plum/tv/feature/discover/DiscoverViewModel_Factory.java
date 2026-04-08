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
public final class DiscoverViewModel_Factory implements Factory<DiscoverViewModel> {
  private final Provider<DiscoverRepository> repositoryProvider;

  public DiscoverViewModel_Factory(Provider<DiscoverRepository> repositoryProvider) {
    this.repositoryProvider = repositoryProvider;
  }

  @Override
  public DiscoverViewModel get() {
    return newInstance(repositoryProvider.get());
  }

  public static DiscoverViewModel_Factory create(Provider<DiscoverRepository> repositoryProvider) {
    return new DiscoverViewModel_Factory(repositoryProvider);
  }

  public static DiscoverViewModel newInstance(DiscoverRepository repository) {
    return new DiscoverViewModel(repository);
  }
}
