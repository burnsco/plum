package plum.tv.app;

import android.content.Context;
import androidx.lifecycle.SavedStateHandle;
import androidx.media3.datasource.DataSource;
import dagger.internal.DaggerGenerated;
import dagger.internal.Factory;
import dagger.internal.QualifierMetadata;
import dagger.internal.ScopeMetadata;
import javax.annotation.processing.Generated;
import javax.inject.Provider;
import plum.tv.core.data.BrowseRepository;
import plum.tv.core.data.PlaybackRepository;
import plum.tv.core.data.PlumWebSocketManager;

@ScopeMetadata
@QualifierMetadata("dagger.hilt.android.qualifiers.ApplicationContext")
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
public final class PlayerViewModel_Factory implements Factory<PlayerViewModel> {
  private final Provider<Context> appContextProvider;

  private final Provider<DataSource.Factory> dataSourceFactoryProvider;

  private final Provider<BrowseRepository> browseRepositoryProvider;

  private final Provider<PlaybackRepository> playbackRepositoryProvider;

  private final Provider<PlumWebSocketManager> wsManagerProvider;

  private final Provider<SavedStateHandle> savedStateHandleProvider;

  public PlayerViewModel_Factory(Provider<Context> appContextProvider,
      Provider<DataSource.Factory> dataSourceFactoryProvider,
      Provider<BrowseRepository> browseRepositoryProvider,
      Provider<PlaybackRepository> playbackRepositoryProvider,
      Provider<PlumWebSocketManager> wsManagerProvider,
      Provider<SavedStateHandle> savedStateHandleProvider) {
    this.appContextProvider = appContextProvider;
    this.dataSourceFactoryProvider = dataSourceFactoryProvider;
    this.browseRepositoryProvider = browseRepositoryProvider;
    this.playbackRepositoryProvider = playbackRepositoryProvider;
    this.wsManagerProvider = wsManagerProvider;
    this.savedStateHandleProvider = savedStateHandleProvider;
  }

  @Override
  public PlayerViewModel get() {
    return newInstance(appContextProvider.get(), dataSourceFactoryProvider.get(), browseRepositoryProvider.get(), playbackRepositoryProvider.get(), wsManagerProvider.get(), savedStateHandleProvider.get());
  }

  public static PlayerViewModel_Factory create(Provider<Context> appContextProvider,
      Provider<DataSource.Factory> dataSourceFactoryProvider,
      Provider<BrowseRepository> browseRepositoryProvider,
      Provider<PlaybackRepository> playbackRepositoryProvider,
      Provider<PlumWebSocketManager> wsManagerProvider,
      Provider<SavedStateHandle> savedStateHandleProvider) {
    return new PlayerViewModel_Factory(appContextProvider, dataSourceFactoryProvider, browseRepositoryProvider, playbackRepositoryProvider, wsManagerProvider, savedStateHandleProvider);
  }

  public static PlayerViewModel newInstance(Context appContext,
      DataSource.Factory dataSourceFactory, BrowseRepository browseRepository,
      PlaybackRepository playbackRepository, PlumWebSocketManager wsManager,
      SavedStateHandle savedStateHandle) {
    return new PlayerViewModel(appContext, dataSourceFactory, browseRepository, playbackRepository, wsManager, savedStateHandle);
  }
}
