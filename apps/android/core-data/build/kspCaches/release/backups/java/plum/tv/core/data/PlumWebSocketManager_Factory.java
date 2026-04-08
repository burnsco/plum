package plum.tv.core.data;

import com.squareup.moshi.Moshi;
import dagger.internal.DaggerGenerated;
import dagger.internal.Factory;
import dagger.internal.Provider;
import dagger.internal.QualifierMetadata;
import dagger.internal.ScopeMetadata;
import javax.annotation.processing.Generated;
import okhttp3.OkHttpClient;

@ScopeMetadata("javax.inject.Singleton")
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
public final class PlumWebSocketManager_Factory implements Factory<PlumWebSocketManager> {
  private final Provider<OkHttpClient> okHttpClientProvider;

  private final Provider<SessionPreferences> prefsProvider;

  private final Provider<AuthTokenBridge> tokenBridgeProvider;

  private final Provider<Moshi> moshiProvider;

  private final Provider<LibraryCatalogRefreshCoordinator> catalogRefreshCoordinatorProvider;

  private PlumWebSocketManager_Factory(Provider<OkHttpClient> okHttpClientProvider,
      Provider<SessionPreferences> prefsProvider, Provider<AuthTokenBridge> tokenBridgeProvider,
      Provider<Moshi> moshiProvider,
      Provider<LibraryCatalogRefreshCoordinator> catalogRefreshCoordinatorProvider) {
    this.okHttpClientProvider = okHttpClientProvider;
    this.prefsProvider = prefsProvider;
    this.tokenBridgeProvider = tokenBridgeProvider;
    this.moshiProvider = moshiProvider;
    this.catalogRefreshCoordinatorProvider = catalogRefreshCoordinatorProvider;
  }

  @Override
  public PlumWebSocketManager get() {
    return newInstance(okHttpClientProvider.get(), prefsProvider.get(), tokenBridgeProvider.get(), moshiProvider.get(), catalogRefreshCoordinatorProvider.get());
  }

  public static PlumWebSocketManager_Factory create(Provider<OkHttpClient> okHttpClientProvider,
      Provider<SessionPreferences> prefsProvider, Provider<AuthTokenBridge> tokenBridgeProvider,
      Provider<Moshi> moshiProvider,
      Provider<LibraryCatalogRefreshCoordinator> catalogRefreshCoordinatorProvider) {
    return new PlumWebSocketManager_Factory(okHttpClientProvider, prefsProvider, tokenBridgeProvider, moshiProvider, catalogRefreshCoordinatorProvider);
  }

  public static PlumWebSocketManager newInstance(OkHttpClient okHttpClient,
      SessionPreferences prefs, AuthTokenBridge tokenBridge, Moshi moshi,
      LibraryCatalogRefreshCoordinator catalogRefreshCoordinator) {
    return new PlumWebSocketManager(okHttpClient, prefs, tokenBridge, moshi, catalogRefreshCoordinator);
  }
}
