package plum.tv.core.data;

import android.content.Context;
import dagger.internal.DaggerGenerated;
import dagger.internal.Factory;
import dagger.internal.Provider;
import dagger.internal.QualifierMetadata;
import dagger.internal.ScopeMetadata;
import javax.annotation.processing.Generated;
import kotlinx.coroutines.CoroutineScope;

@ScopeMetadata("javax.inject.Singleton")
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
public final class PlayerSubtitlePreferences_Factory implements Factory<PlayerSubtitlePreferences> {
  private final Provider<Context> contextProvider;

  private final Provider<CoroutineScope> scopeProvider;

  private PlayerSubtitlePreferences_Factory(Provider<Context> contextProvider,
      Provider<CoroutineScope> scopeProvider) {
    this.contextProvider = contextProvider;
    this.scopeProvider = scopeProvider;
  }

  @Override
  public PlayerSubtitlePreferences get() {
    return newInstance(contextProvider.get(), scopeProvider.get());
  }

  public static PlayerSubtitlePreferences_Factory create(Provider<Context> contextProvider,
      Provider<CoroutineScope> scopeProvider) {
    return new PlayerSubtitlePreferences_Factory(contextProvider, scopeProvider);
  }

  public static PlayerSubtitlePreferences newInstance(Context context, CoroutineScope scope) {
    return new PlayerSubtitlePreferences(context, scope);
  }
}
