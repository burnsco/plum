package plum.tv.app;

import android.content.Context;
import androidx.lifecycle.SavedStateHandle;
import androidx.media3.datasource.DataSource;
import dagger.internal.DaggerGenerated;
import dagger.internal.Factory;
import dagger.internal.Provider;
import dagger.internal.QualifierMetadata;
import dagger.internal.ScopeMetadata;
import javax.annotation.processing.Generated;
import kotlinx.coroutines.CoroutineScope;
import plum.tv.core.data.BrowseRepository;
import plum.tv.core.data.PlaybackRepository;
import plum.tv.core.data.PlayerSubtitlePreferences;
import plum.tv.core.data.PlumWebSocketManager;

@ScopeMetadata
@QualifierMetadata({
    "dagger.hilt.android.qualifiers.ApplicationContext",
    "plum.tv.core.data.di.ApplicationScope"
})
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
public final class PlayerViewModel_Factory implements Factory<PlayerViewModel> {
  private final Provider<Context> appContextProvider;

  private final Provider<CoroutineScope> applicationScopeProvider;

  private final Provider<DataSource.Factory> dataSourceFactoryProvider;

  private final Provider<BrowseRepository> browseRepositoryProvider;

  private final Provider<PlaybackRepository> playbackRepositoryProvider;

  private final Provider<PlumWebSocketManager> wsManagerProvider;

  private final Provider<PlayerSubtitlePreferences> playerSubtitlePreferencesProvider;

  private final Provider<SavedStateHandle> savedStateHandleProvider;

  private PlayerViewModel_Factory(Provider<Context> appContextProvider,
      Provider<CoroutineScope> applicationScopeProvider,
      Provider<DataSource.Factory> dataSourceFactoryProvider,
      Provider<BrowseRepository> browseRepositoryProvider,
      Provider<PlaybackRepository> playbackRepositoryProvider,
      Provider<PlumWebSocketManager> wsManagerProvider,
      Provider<PlayerSubtitlePreferences> playerSubtitlePreferencesProvider,
      Provider<SavedStateHandle> savedStateHandleProvider) {
    this.appContextProvider = appContextProvider;
    this.applicationScopeProvider = applicationScopeProvider;
    this.dataSourceFactoryProvider = dataSourceFactoryProvider;
    this.browseRepositoryProvider = browseRepositoryProvider;
    this.playbackRepositoryProvider = playbackRepositoryProvider;
    this.wsManagerProvider = wsManagerProvider;
    this.playerSubtitlePreferencesProvider = playerSubtitlePreferencesProvider;
    this.savedStateHandleProvider = savedStateHandleProvider;
  }

  @Override
  public PlayerViewModel get() {
    return newInstance(appContextProvider.get(), applicationScopeProvider.get(), dataSourceFactoryProvider.get(), browseRepositoryProvider.get(), playbackRepositoryProvider.get(), wsManagerProvider.get(), playerSubtitlePreferencesProvider.get(), savedStateHandleProvider.get());
  }

  public static PlayerViewModel_Factory create(Provider<Context> appContextProvider,
      Provider<CoroutineScope> applicationScopeProvider,
      Provider<DataSource.Factory> dataSourceFactoryProvider,
      Provider<BrowseRepository> browseRepositoryProvider,
      Provider<PlaybackRepository> playbackRepositoryProvider,
      Provider<PlumWebSocketManager> wsManagerProvider,
      Provider<PlayerSubtitlePreferences> playerSubtitlePreferencesProvider,
      Provider<SavedStateHandle> savedStateHandleProvider) {
    return new PlayerViewModel_Factory(appContextProvider, applicationScopeProvider, dataSourceFactoryProvider, browseRepositoryProvider, playbackRepositoryProvider, wsManagerProvider, playerSubtitlePreferencesProvider, savedStateHandleProvider);
  }

  public static PlayerViewModel newInstance(Context appContext, CoroutineScope applicationScope,
      DataSource.Factory dataSourceFactory, BrowseRepository browseRepository,
      PlaybackRepository playbackRepository, PlumWebSocketManager wsManager,
      PlayerSubtitlePreferences playerSubtitlePreferences, SavedStateHandle savedStateHandle) {
    return new PlayerViewModel(appContext, applicationScope, dataSourceFactory, browseRepository, playbackRepository, wsManager, playerSubtitlePreferences, savedStateHandle);
  }
}
