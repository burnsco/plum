package plum.tv.core.data;

import com.squareup.moshi.Moshi;
import dagger.internal.DaggerGenerated;
import dagger.internal.Factory;
import dagger.internal.QualifierMetadata;
import dagger.internal.ScopeMetadata;
import javax.annotation.processing.Generated;
import javax.inject.Provider;
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
    "cast"
})
public final class PlumWebSocketManager_Factory implements Factory<PlumWebSocketManager> {
  private final Provider<OkHttpClient> okHttpClientProvider;

  private final Provider<SessionPreferences> prefsProvider;

  private final Provider<AuthTokenBridge> tokenBridgeProvider;

  private final Provider<Moshi> moshiProvider;

  public PlumWebSocketManager_Factory(Provider<OkHttpClient> okHttpClientProvider,
      Provider<SessionPreferences> prefsProvider, Provider<AuthTokenBridge> tokenBridgeProvider,
      Provider<Moshi> moshiProvider) {
    this.okHttpClientProvider = okHttpClientProvider;
    this.prefsProvider = prefsProvider;
    this.tokenBridgeProvider = tokenBridgeProvider;
    this.moshiProvider = moshiProvider;
  }

  @Override
  public PlumWebSocketManager get() {
    return newInstance(okHttpClientProvider.get(), prefsProvider.get(), tokenBridgeProvider.get(), moshiProvider.get());
  }

  public static PlumWebSocketManager_Factory create(Provider<OkHttpClient> okHttpClientProvider,
      Provider<SessionPreferences> prefsProvider, Provider<AuthTokenBridge> tokenBridgeProvider,
      Provider<Moshi> moshiProvider) {
    return new PlumWebSocketManager_Factory(okHttpClientProvider, prefsProvider, tokenBridgeProvider, moshiProvider);
  }

  public static PlumWebSocketManager newInstance(OkHttpClient okHttpClient,
      SessionPreferences prefs, AuthTokenBridge tokenBridge, Moshi moshi) {
    return new PlumWebSocketManager(okHttpClient, prefs, tokenBridge, moshi);
  }
}
