package plum.tv.feature.library;

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
public final class LibraryBrowseViewModel_Factory implements Factory<LibraryBrowseViewModel> {
  private final Provider<BrowseRepository> browseRepositoryProvider;

  private final Provider<SavedStateHandle> savedStateHandleProvider;

  public LibraryBrowseViewModel_Factory(Provider<BrowseRepository> browseRepositoryProvider,
      Provider<SavedStateHandle> savedStateHandleProvider) {
    this.browseRepositoryProvider = browseRepositoryProvider;
    this.savedStateHandleProvider = savedStateHandleProvider;
  }

  @Override
  public LibraryBrowseViewModel get() {
    return newInstance(browseRepositoryProvider.get(), savedStateHandleProvider.get());
  }

  public static LibraryBrowseViewModel_Factory create(
      Provider<BrowseRepository> browseRepositoryProvider,
      Provider<SavedStateHandle> savedStateHandleProvider) {
    return new LibraryBrowseViewModel_Factory(browseRepositoryProvider, savedStateHandleProvider);
  }

  public static LibraryBrowseViewModel newInstance(BrowseRepository browseRepository,
      SavedStateHandle savedStateHandle) {
    return new LibraryBrowseViewModel(browseRepository, savedStateHandle);
  }
}
