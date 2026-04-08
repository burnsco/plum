package plum.tv.core.data;

import android.content.Context;
import com.squareup.moshi.Moshi;
import dagger.internal.DaggerGenerated;
import dagger.internal.Factory;
import dagger.internal.Provider;
import dagger.internal.QualifierMetadata;
import dagger.internal.ScopeMetadata;
import javax.annotation.processing.Generated;

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
public final class HomeDashboardDiskCache_Factory implements Factory<HomeDashboardDiskCache> {
  private final Provider<Context> contextProvider;

  private final Provider<Moshi> moshiProvider;

  private HomeDashboardDiskCache_Factory(Provider<Context> contextProvider,
      Provider<Moshi> moshiProvider) {
    this.contextProvider = contextProvider;
    this.moshiProvider = moshiProvider;
  }

  @Override
  public HomeDashboardDiskCache get() {
    return newInstance(contextProvider.get(), moshiProvider.get());
  }

  public static HomeDashboardDiskCache_Factory create(Provider<Context> contextProvider,
      Provider<Moshi> moshiProvider) {
    return new HomeDashboardDiskCache_Factory(contextProvider, moshiProvider);
  }

  public static HomeDashboardDiskCache newInstance(Context context, Moshi moshi) {
    return new HomeDashboardDiskCache(context, moshi);
  }
}
