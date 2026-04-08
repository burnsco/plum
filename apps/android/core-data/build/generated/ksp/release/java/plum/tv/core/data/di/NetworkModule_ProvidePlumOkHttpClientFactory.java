package plum.tv.core.data.di;

import android.content.Context;
import dagger.internal.DaggerGenerated;
import dagger.internal.Factory;
import dagger.internal.Preconditions;
import dagger.internal.Provider;
import dagger.internal.QualifierMetadata;
import dagger.internal.ScopeMetadata;
import javax.annotation.processing.Generated;
import okhttp3.OkHttpClient;
import plum.tv.core.data.AuthTokenBridge;

@ScopeMetadata("javax.inject.Singleton")
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
    "cast",
    "deprecation",
    "nullness:initialization.field.uninitialized"
})
public final class NetworkModule_ProvidePlumOkHttpClientFactory implements Factory<OkHttpClient> {
  private final Provider<Context> contextProvider;

  private final Provider<AuthTokenBridge> bridgeProvider;

  private NetworkModule_ProvidePlumOkHttpClientFactory(Provider<Context> contextProvider,
      Provider<AuthTokenBridge> bridgeProvider) {
    this.contextProvider = contextProvider;
    this.bridgeProvider = bridgeProvider;
  }

  @Override
  public OkHttpClient get() {
    return providePlumOkHttpClient(contextProvider.get(), bridgeProvider.get());
  }

  public static NetworkModule_ProvidePlumOkHttpClientFactory create(
      Provider<Context> contextProvider, Provider<AuthTokenBridge> bridgeProvider) {
    return new NetworkModule_ProvidePlumOkHttpClientFactory(contextProvider, bridgeProvider);
  }

  public static OkHttpClient providePlumOkHttpClient(Context context, AuthTokenBridge bridge) {
    return Preconditions.checkNotNullFromProvides(NetworkModule.INSTANCE.providePlumOkHttpClient(context, bridge));
  }
}
