package plum.tv.core.data;

import dagger.internal.DaggerGenerated;
import dagger.internal.Factory;
import dagger.internal.QualifierMetadata;
import dagger.internal.ScopeMetadata;
import javax.annotation.processing.Generated;

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
public final class AuthTokenBridge_Factory implements Factory<AuthTokenBridge> {
  @Override
  public AuthTokenBridge get() {
    return newInstance();
  }

  public static AuthTokenBridge_Factory create() {
    return InstanceHolder.INSTANCE;
  }

  public static AuthTokenBridge newInstance() {
    return new AuthTokenBridge();
  }

  private static final class InstanceHolder {
    private static final AuthTokenBridge_Factory INSTANCE = new AuthTokenBridge_Factory();
  }
}
