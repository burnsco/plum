package plum.tv.feature.discover;

import dagger.internal.DaggerGenerated;
import dagger.internal.Factory;
import dagger.internal.Provider;
import dagger.internal.QualifierMetadata;
import dagger.internal.ScopeMetadata;
import javax.annotation.processing.Generated;
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
    "cast",
    "deprecation",
    "nullness:initialization.field.uninitialized"
})
public final class DownloadsViewModel_Factory implements Factory<DownloadsViewModel> {
  private final Provider<DiscoverRepository> repositoryProvider;

  private DownloadsViewModel_Factory(Provider<DiscoverRepository> repositoryProvider) {
    this.repositoryProvider = repositoryProvider;
  }

  @Override
  public DownloadsViewModel get() {
    return newInstance(repositoryProvider.get());
  }

  public static DownloadsViewModel_Factory create(Provider<DiscoverRepository> repositoryProvider) {
    return new DownloadsViewModel_Factory(repositoryProvider);
  }

  public static DownloadsViewModel newInstance(DiscoverRepository repository) {
    return new DownloadsViewModel(repository);
  }
}
