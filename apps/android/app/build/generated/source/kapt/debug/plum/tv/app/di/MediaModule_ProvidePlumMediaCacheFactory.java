package plum.tv.app.di;

import android.content.Context;
import androidx.media3.datasource.cache.Cache;
import dagger.internal.DaggerGenerated;
import dagger.internal.Factory;
import dagger.internal.Preconditions;
import dagger.internal.QualifierMetadata;
import dagger.internal.ScopeMetadata;
import javax.annotation.processing.Generated;
import javax.inject.Provider;

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
    "cast"
})
public final class MediaModule_ProvidePlumMediaCacheFactory implements Factory<Cache> {
  private final Provider<Context> contextProvider;

  public MediaModule_ProvidePlumMediaCacheFactory(Provider<Context> contextProvider) {
    this.contextProvider = contextProvider;
  }

  @Override
  public Cache get() {
    return providePlumMediaCache(contextProvider.get());
  }

  public static MediaModule_ProvidePlumMediaCacheFactory create(Provider<Context> contextProvider) {
    return new MediaModule_ProvidePlumMediaCacheFactory(contextProvider);
  }

  public static Cache providePlumMediaCache(Context context) {
    return Preconditions.checkNotNullFromProvides(MediaModule.INSTANCE.providePlumMediaCache(context));
  }
}
