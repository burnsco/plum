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
public final class ShowDetailViewModel_Factory implements Factory<ShowDetailViewModel> {
  private final Provider<BrowseRepository> browseRepositoryProvider;

  private final Provider<SavedStateHandle> savedStateHandleProvider;

  public ShowDetailViewModel_Factory(Provider<BrowseRepository> browseRepositoryProvider,
      Provider<SavedStateHandle> savedStateHandleProvider) {
    this.browseRepositoryProvider = browseRepositoryProvider;
    this.savedStateHandleProvider = savedStateHandleProvider;
  }

  @Override
  public ShowDetailViewModel get() {
    return newInstance(browseRepositoryProvider.get(), savedStateHandleProvider.get());
  }

  public static ShowDetailViewModel_Factory create(
      Provider<BrowseRepository> browseRepositoryProvider,
      Provider<SavedStateHandle> savedStateHandleProvider) {
    return new ShowDetailViewModel_Factory(browseRepositoryProvider, savedStateHandleProvider);
  }

  public static ShowDetailViewModel newInstance(BrowseRepository browseRepository,
      SavedStateHandle savedStateHandle) {
    return new ShowDetailViewModel(browseRepository, savedStateHandle);
  }
}
