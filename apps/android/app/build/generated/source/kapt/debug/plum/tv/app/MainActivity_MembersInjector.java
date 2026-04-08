package plum.tv.app;

import dagger.MembersInjector;
import dagger.internal.DaggerGenerated;
import dagger.internal.InjectedFieldSignature;
import dagger.internal.QualifierMetadata;
import javax.annotation.processing.Generated;
import javax.inject.Provider;
import plum.tv.core.data.PlumWebSocketManager;
import plum.tv.core.data.SessionPreferences;

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
public final class MainActivity_MembersInjector implements MembersInjector<MainActivity> {
  private final Provider<PlumWebSocketManager> webSocketManagerProvider;

  private final Provider<SessionPreferences> sessionPreferencesProvider;

  public MainActivity_MembersInjector(Provider<PlumWebSocketManager> webSocketManagerProvider,
      Provider<SessionPreferences> sessionPreferencesProvider) {
    this.webSocketManagerProvider = webSocketManagerProvider;
    this.sessionPreferencesProvider = sessionPreferencesProvider;
  }

  public static MembersInjector<MainActivity> create(
      Provider<PlumWebSocketManager> webSocketManagerProvider,
      Provider<SessionPreferences> sessionPreferencesProvider) {
    return new MainActivity_MembersInjector(webSocketManagerProvider, sessionPreferencesProvider);
  }

  @Override
  public void injectMembers(MainActivity instance) {
    injectWebSocketManager(instance, webSocketManagerProvider.get());
    injectSessionPreferences(instance, sessionPreferencesProvider.get());
  }

  @InjectedFieldSignature("plum.tv.app.MainActivity.webSocketManager")
  public static void injectWebSocketManager(MainActivity instance,
      PlumWebSocketManager webSocketManager) {
    instance.webSocketManager = webSocketManager;
  }

  @InjectedFieldSignature("plum.tv.app.MainActivity.sessionPreferences")
  public static void injectSessionPreferences(MainActivity instance,
      SessionPreferences sessionPreferences) {
    instance.sessionPreferences = sessionPreferences;
  }
}
