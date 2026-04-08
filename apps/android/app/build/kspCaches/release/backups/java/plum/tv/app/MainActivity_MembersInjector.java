package plum.tv.app;

import dagger.MembersInjector;
import dagger.internal.DaggerGenerated;
import dagger.internal.InjectedFieldSignature;
import dagger.internal.Provider;
import dagger.internal.QualifierMetadata;
import javax.annotation.processing.Generated;
import plum.tv.core.data.LibraryScanStatusPoller;
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
    "cast",
    "deprecation",
    "nullness:initialization.field.uninitialized"
})
public final class MainActivity_MembersInjector implements MembersInjector<MainActivity> {
  private final Provider<PlumWebSocketManager> webSocketManagerProvider;

  private final Provider<LibraryScanStatusPoller> libraryScanStatusPollerProvider;

  private final Provider<SessionPreferences> sessionPreferencesProvider;

  private MainActivity_MembersInjector(Provider<PlumWebSocketManager> webSocketManagerProvider,
      Provider<LibraryScanStatusPoller> libraryScanStatusPollerProvider,
      Provider<SessionPreferences> sessionPreferencesProvider) {
    this.webSocketManagerProvider = webSocketManagerProvider;
    this.libraryScanStatusPollerProvider = libraryScanStatusPollerProvider;
    this.sessionPreferencesProvider = sessionPreferencesProvider;
  }

  @Override
  public void injectMembers(MainActivity instance) {
    injectWebSocketManager(instance, webSocketManagerProvider.get());
    injectLibraryScanStatusPoller(instance, libraryScanStatusPollerProvider.get());
    injectSessionPreferences(instance, sessionPreferencesProvider.get());
  }

  public static MembersInjector<MainActivity> create(
      Provider<PlumWebSocketManager> webSocketManagerProvider,
      Provider<LibraryScanStatusPoller> libraryScanStatusPollerProvider,
      Provider<SessionPreferences> sessionPreferencesProvider) {
    return new MainActivity_MembersInjector(webSocketManagerProvider, libraryScanStatusPollerProvider, sessionPreferencesProvider);
  }

  @InjectedFieldSignature("plum.tv.app.MainActivity.webSocketManager")
  public static void injectWebSocketManager(MainActivity instance,
      PlumWebSocketManager webSocketManager) {
    instance.webSocketManager = webSocketManager;
  }

  @InjectedFieldSignature("plum.tv.app.MainActivity.libraryScanStatusPoller")
  public static void injectLibraryScanStatusPoller(MainActivity instance,
      LibraryScanStatusPoller libraryScanStatusPoller) {
    instance.libraryScanStatusPoller = libraryScanStatusPoller;
  }

  @InjectedFieldSignature("plum.tv.app.MainActivity.sessionPreferences")
  public static void injectSessionPreferences(MainActivity instance,
      SessionPreferences sessionPreferences) {
    instance.sessionPreferences = sessionPreferences;
  }
}
