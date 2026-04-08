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
public final class SessionPreferences_Factory implements Factory<SessionPreferences> {
  private final Provider<Context> contextProvider;

  private final Provider<CoroutineScope> scopeProvider;

  private SessionPreferences_Factory(Provider<Context> contextProvider,
      Provider<CoroutineScope> scopeProvider) {
    this.contextProvider = contextProvider;
    this.scopeProvider = scopeProvider;
  }

  @Override
  public SessionPreferences get() {
    return newInstance(contextProvider.get(), scopeProvider.get());
  }

  public static SessionPreferences_Factory create(Provider<Context> contextProvider,
      Provider<CoroutineScope> scopeProvider) {
    return new SessionPreferences_Factory(contextProvider, scopeProvider);
  }

  public static SessionPreferences newInstance(Context context, CoroutineScope scope) {
    return new SessionPreferences(context, scope);
  }
}
