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
public final class SessionRepository_Factory implements Factory<SessionRepository> {
  private final Provider<SessionPreferences> prefsProvider;

  private final Provider<Moshi> moshiProvider;

  private final Provider<AuthTokenBridge> tokenBridgeProvider;

  private final Provider<OkHttpClient> okHttpClientProvider;

  private SessionRepository_Factory(Provider<SessionPreferences> prefsProvider,
      Provider<Moshi> moshiProvider, Provider<AuthTokenBridge> tokenBridgeProvider,
      Provider<OkHttpClient> okHttpClientProvider) {
    this.prefsProvider = prefsProvider;
    this.moshiProvider = moshiProvider;
    this.tokenBridgeProvider = tokenBridgeProvider;
    this.okHttpClientProvider = okHttpClientProvider;
  }

  @Override
  public SessionRepository get() {
    return newInstance(prefsProvider.get(), moshiProvider.get(), tokenBridgeProvider.get(), okHttpClientProvider.get());
  }

  public static SessionRepository_Factory create(Provider<SessionPreferences> prefsProvider,
      Provider<Moshi> moshiProvider, Provider<AuthTokenBridge> tokenBridgeProvider,
      Provider<OkHttpClient> okHttpClientProvider) {
    return new SessionRepository_Factory(prefsProvider, moshiProvider, tokenBridgeProvider, okHttpClientProvider);
  }

  public static SessionRepository newInstance(SessionPreferences prefs, Moshi moshi,
      AuthTokenBridge tokenBridge, OkHttpClient okHttpClient) {
    return new SessionRepository(prefs, moshi, tokenBridge, okHttpClient);
  }
}
