package plum.tv.feature.details;

import androidx.lifecycle.SavedStateHandle;
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
public final class MovieDetailViewModel_Factory implements Factory<MovieDetailViewModel> {
  private final Provider<BrowseRepository> browseRepositoryProvider;

  private final Provider<SavedStateHandle> savedStateHandleProvider;

  public MovieDetailViewModel_Factory(Provider<BrowseRepository> browseRepositoryProvider,
      Provider<SavedStateHandle> savedStateHandleProvider) {
    this.browseRepositoryProvider = browseRepositoryProvider;
    this.savedStateHandleProvider = savedStateHandleProvider;
  }

  @Override
  public MovieDetailViewModel get() {
    return newInstance(browseRepositoryProvider.get(), savedStateHandleProvider.get());
  }

  public static MovieDetailViewModel_Factory create(
      Provider<BrowseRepository> browseRepositoryProvider,
      Provider<SavedStateHandle> savedStateHandleProvider) {
    return new MovieDetailViewModel_Factory(browseRepositoryProvider, savedStateHandleProvider);
  }

  public static MovieDetailViewModel newInstance(BrowseRepository browseRepository,
      SavedStateHandle savedStateHandle) {
    return new MovieDetailViewModel(browseRepository, savedStateHandle);
  }
}
