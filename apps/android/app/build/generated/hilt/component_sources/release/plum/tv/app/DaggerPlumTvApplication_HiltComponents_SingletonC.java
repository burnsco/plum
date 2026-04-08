package plum.tv.app;

import android.app.Activity;
import android.app.Service;
import android.view.View;
import androidx.fragment.app.Fragment;
import androidx.lifecycle.SavedStateHandle;
import androidx.lifecycle.ViewModel;
import androidx.media3.datasource.DataSource;
import com.google.common.collect.ImmutableMap;
import com.google.common.collect.ImmutableSet;
import com.squareup.moshi.Moshi;
import dagger.hilt.android.ActivityRetainedLifecycle;
import dagger.hilt.android.ViewModelLifecycle;
import dagger.hilt.android.internal.builders.ActivityComponentBuilder;
import dagger.hilt.android.internal.builders.ActivityRetainedComponentBuilder;
import dagger.hilt.android.internal.builders.FragmentComponentBuilder;
import dagger.hilt.android.internal.builders.ServiceComponentBuilder;
import dagger.hilt.android.internal.builders.ViewComponentBuilder;
import dagger.hilt.android.internal.builders.ViewModelComponentBuilder;
import dagger.hilt.android.internal.builders.ViewWithFragmentComponentBuilder;
import dagger.hilt.android.internal.lifecycle.DefaultViewModelFactories;
import dagger.hilt.android.internal.lifecycle.DefaultViewModelFactories_InternalFactoryFactory_Factory;
import dagger.hilt.android.internal.managers.ActivityRetainedComponentManager_LifecycleModule_ProvideActivityRetainedLifecycleFactory;
import dagger.hilt.android.internal.managers.SavedStateHandleHolder;
import dagger.hilt.android.internal.modules.ApplicationContextModule;
import dagger.hilt.android.internal.modules.ApplicationContextModule_ProvideContextFactory;
import dagger.internal.DaggerGenerated;
import dagger.internal.DoubleCheck;
import dagger.internal.LazyClassKeyMap;
import dagger.internal.Preconditions;
import dagger.internal.Provider;
import java.util.Map;
import java.util.Set;
import javax.annotation.processing.Generated;
import kotlinx.coroutines.CoroutineScope;
import okhttp3.OkHttpClient;
import plum.tv.app.di.MediaModule_ProvideApplicationScopeFactory;
import plum.tv.app.di.MediaModule_ProvidePlumMediaDataSourceFactoryFactory;
import plum.tv.core.data.AuthTokenBridge;
import plum.tv.core.data.BrowseRepository;
import plum.tv.core.data.DiscoverRepository;
import plum.tv.core.data.HomeDashboardDiskCache;
import plum.tv.core.data.LibraryCatalogRefreshCoordinator;
import plum.tv.core.data.LibraryScanStatusPoller;
import plum.tv.core.data.PlaybackRepository;
import plum.tv.core.data.PlayerSubtitlePreferences;
import plum.tv.core.data.PlumWebSocketManager;
import plum.tv.core.data.SearchRepository;
import plum.tv.core.data.SessionPreferences;
import plum.tv.core.data.SessionRepository;
import plum.tv.core.data.di.DataModule_ProvideMoshiFactory;
import plum.tv.core.data.di.NetworkModule_ProvidePlumOkHttpClientFactory;
import plum.tv.feature.auth.AuthViewModel;
import plum.tv.feature.auth.AuthViewModel_HiltModules;
import plum.tv.feature.auth.AuthViewModel_HiltModules_BindsModule_Binds_LazyMapKey;
import plum.tv.feature.auth.AuthViewModel_HiltModules_KeyModule_Provide_LazyMapKey;
import plum.tv.feature.details.MovieDetailViewModel;
import plum.tv.feature.details.MovieDetailViewModel_HiltModules;
import plum.tv.feature.details.MovieDetailViewModel_HiltModules_BindsModule_Binds_LazyMapKey;
import plum.tv.feature.details.MovieDetailViewModel_HiltModules_KeyModule_Provide_LazyMapKey;
import plum.tv.feature.details.ShowDetailViewModel;
import plum.tv.feature.details.ShowDetailViewModel_HiltModules;
import plum.tv.feature.details.ShowDetailViewModel_HiltModules_BindsModule_Binds_LazyMapKey;
import plum.tv.feature.details.ShowDetailViewModel_HiltModules_KeyModule_Provide_LazyMapKey;
import plum.tv.feature.discover.DiscoverBrowseViewModel;
import plum.tv.feature.discover.DiscoverBrowseViewModel_HiltModules;
import plum.tv.feature.discover.DiscoverBrowseViewModel_HiltModules_BindsModule_Binds_LazyMapKey;
import plum.tv.feature.discover.DiscoverBrowseViewModel_HiltModules_KeyModule_Provide_LazyMapKey;
import plum.tv.feature.discover.DiscoverDetailViewModel;
import plum.tv.feature.discover.DiscoverDetailViewModel_HiltModules;
import plum.tv.feature.discover.DiscoverDetailViewModel_HiltModules_BindsModule_Binds_LazyMapKey;
import plum.tv.feature.discover.DiscoverDetailViewModel_HiltModules_KeyModule_Provide_LazyMapKey;
import plum.tv.feature.discover.DiscoverViewModel;
import plum.tv.feature.discover.DiscoverViewModel_HiltModules;
import plum.tv.feature.discover.DiscoverViewModel_HiltModules_BindsModule_Binds_LazyMapKey;
import plum.tv.feature.discover.DiscoverViewModel_HiltModules_KeyModule_Provide_LazyMapKey;
import plum.tv.feature.discover.DownloadsViewModel;
import plum.tv.feature.discover.DownloadsViewModel_HiltModules;
import plum.tv.feature.discover.DownloadsViewModel_HiltModules_BindsModule_Binds_LazyMapKey;
import plum.tv.feature.discover.DownloadsViewModel_HiltModules_KeyModule_Provide_LazyMapKey;
import plum.tv.feature.home.HomeViewModel;
import plum.tv.feature.home.HomeViewModel_HiltModules;
import plum.tv.feature.home.HomeViewModel_HiltModules_BindsModule_Binds_LazyMapKey;
import plum.tv.feature.home.HomeViewModel_HiltModules_KeyModule_Provide_LazyMapKey;
import plum.tv.feature.library.LibraryBrowseViewModel;
import plum.tv.feature.library.LibraryBrowseViewModel_HiltModules;
import plum.tv.feature.library.LibraryBrowseViewModel_HiltModules_BindsModule_Binds_LazyMapKey;
import plum.tv.feature.library.LibraryBrowseViewModel_HiltModules_KeyModule_Provide_LazyMapKey;
import plum.tv.feature.library.LibraryListViewModel;
import plum.tv.feature.library.LibraryListViewModel_HiltModules;
import plum.tv.feature.library.LibraryListViewModel_HiltModules_BindsModule_Binds_LazyMapKey;
import plum.tv.feature.library.LibraryListViewModel_HiltModules_KeyModule_Provide_LazyMapKey;
import plum.tv.feature.search.SearchViewModel;
import plum.tv.feature.search.SearchViewModel_HiltModules;
import plum.tv.feature.search.SearchViewModel_HiltModules_BindsModule_Binds_LazyMapKey;
import plum.tv.feature.search.SearchViewModel_HiltModules_KeyModule_Provide_LazyMapKey;

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
public final class DaggerPlumTvApplication_HiltComponents_SingletonC {
  private DaggerPlumTvApplication_HiltComponents_SingletonC() {
  }

  public static Builder builder() {
    return new Builder();
  }

  public static final class Builder {
    private ApplicationContextModule applicationContextModule;

    private Builder() {
    }

    public Builder applicationContextModule(ApplicationContextModule applicationContextModule) {
      this.applicationContextModule = Preconditions.checkNotNull(applicationContextModule);
      return this;
    }

    public PlumTvApplication_HiltComponents.SingletonC build() {
      Preconditions.checkBuilderRequirement(applicationContextModule, ApplicationContextModule.class);
      return new SingletonCImpl(applicationContextModule);
    }
  }

  private static final class ActivityRetainedCBuilder implements PlumTvApplication_HiltComponents.ActivityRetainedC.Builder {
    private final SingletonCImpl singletonCImpl;

    private SavedStateHandleHolder savedStateHandleHolder;

    private ActivityRetainedCBuilder(SingletonCImpl singletonCImpl) {
      this.singletonCImpl = singletonCImpl;
    }

    @Override
    public ActivityRetainedCBuilder savedStateHandleHolder(
        SavedStateHandleHolder savedStateHandleHolder) {
      this.savedStateHandleHolder = Preconditions.checkNotNull(savedStateHandleHolder);
      return this;
    }

    @Override
    public PlumTvApplication_HiltComponents.ActivityRetainedC build() {
      Preconditions.checkBuilderRequirement(savedStateHandleHolder, SavedStateHandleHolder.class);
      return new ActivityRetainedCImpl(singletonCImpl, savedStateHandleHolder);
    }
  }

  private static final class ActivityCBuilder implements PlumTvApplication_HiltComponents.ActivityC.Builder {
    private final SingletonCImpl singletonCImpl;

    private final ActivityRetainedCImpl activityRetainedCImpl;

    private Activity activity;

    private ActivityCBuilder(SingletonCImpl singletonCImpl,
        ActivityRetainedCImpl activityRetainedCImpl) {
      this.singletonCImpl = singletonCImpl;
      this.activityRetainedCImpl = activityRetainedCImpl;
    }

    @Override
    public ActivityCBuilder activity(Activity activity) {
      this.activity = Preconditions.checkNotNull(activity);
      return this;
    }

    @Override
    public PlumTvApplication_HiltComponents.ActivityC build() {
      Preconditions.checkBuilderRequirement(activity, Activity.class);
      return new ActivityCImpl(singletonCImpl, activityRetainedCImpl, activity);
    }
  }

  private static final class FragmentCBuilder implements PlumTvApplication_HiltComponents.FragmentC.Builder {
    private final SingletonCImpl singletonCImpl;

    private final ActivityRetainedCImpl activityRetainedCImpl;

    private final ActivityCImpl activityCImpl;

    private Fragment fragment;

    private FragmentCBuilder(SingletonCImpl singletonCImpl,
        ActivityRetainedCImpl activityRetainedCImpl, ActivityCImpl activityCImpl) {
      this.singletonCImpl = singletonCImpl;
      this.activityRetainedCImpl = activityRetainedCImpl;
      this.activityCImpl = activityCImpl;
    }

    @Override
    public FragmentCBuilder fragment(Fragment fragment) {
      this.fragment = Preconditions.checkNotNull(fragment);
      return this;
    }

    @Override
    public PlumTvApplication_HiltComponents.FragmentC build() {
      Preconditions.checkBuilderRequirement(fragment, Fragment.class);
      return new FragmentCImpl(singletonCImpl, activityRetainedCImpl, activityCImpl, fragment);
    }
  }

  private static final class ViewWithFragmentCBuilder implements PlumTvApplication_HiltComponents.ViewWithFragmentC.Builder {
    private final SingletonCImpl singletonCImpl;

    private final ActivityRetainedCImpl activityRetainedCImpl;

    private final ActivityCImpl activityCImpl;

    private final FragmentCImpl fragmentCImpl;

    private View view;

    private ViewWithFragmentCBuilder(SingletonCImpl singletonCImpl,
        ActivityRetainedCImpl activityRetainedCImpl, ActivityCImpl activityCImpl,
        FragmentCImpl fragmentCImpl) {
      this.singletonCImpl = singletonCImpl;
      this.activityRetainedCImpl = activityRetainedCImpl;
      this.activityCImpl = activityCImpl;
      this.fragmentCImpl = fragmentCImpl;
    }

    @Override
    public ViewWithFragmentCBuilder view(View view) {
      this.view = Preconditions.checkNotNull(view);
      return this;
    }

    @Override
    public PlumTvApplication_HiltComponents.ViewWithFragmentC build() {
      Preconditions.checkBuilderRequirement(view, View.class);
      return new ViewWithFragmentCImpl(singletonCImpl, activityRetainedCImpl, activityCImpl, fragmentCImpl, view);
    }
  }

  private static final class ViewCBuilder implements PlumTvApplication_HiltComponents.ViewC.Builder {
    private final SingletonCImpl singletonCImpl;

    private final ActivityRetainedCImpl activityRetainedCImpl;

    private final ActivityCImpl activityCImpl;

    private View view;

    private ViewCBuilder(SingletonCImpl singletonCImpl, ActivityRetainedCImpl activityRetainedCImpl,
        ActivityCImpl activityCImpl) {
      this.singletonCImpl = singletonCImpl;
      this.activityRetainedCImpl = activityRetainedCImpl;
      this.activityCImpl = activityCImpl;
    }

    @Override
    public ViewCBuilder view(View view) {
      this.view = Preconditions.checkNotNull(view);
      return this;
    }

    @Override
    public PlumTvApplication_HiltComponents.ViewC build() {
      Preconditions.checkBuilderRequirement(view, View.class);
      return new ViewCImpl(singletonCImpl, activityRetainedCImpl, activityCImpl, view);
    }
  }

  private static final class ViewModelCBuilder implements PlumTvApplication_HiltComponents.ViewModelC.Builder {
    private final SingletonCImpl singletonCImpl;

    private final ActivityRetainedCImpl activityRetainedCImpl;

    private SavedStateHandle savedStateHandle;

    private ViewModelLifecycle viewModelLifecycle;

    private ViewModelCBuilder(SingletonCImpl singletonCImpl,
        ActivityRetainedCImpl activityRetainedCImpl) {
      this.singletonCImpl = singletonCImpl;
      this.activityRetainedCImpl = activityRetainedCImpl;
    }

    @Override
    public ViewModelCBuilder savedStateHandle(SavedStateHandle handle) {
      this.savedStateHandle = Preconditions.checkNotNull(handle);
      return this;
    }

    @Override
    public ViewModelCBuilder viewModelLifecycle(ViewModelLifecycle viewModelLifecycle) {
      this.viewModelLifecycle = Preconditions.checkNotNull(viewModelLifecycle);
      return this;
    }

    @Override
    public PlumTvApplication_HiltComponents.ViewModelC build() {
      Preconditions.checkBuilderRequirement(savedStateHandle, SavedStateHandle.class);
      Preconditions.checkBuilderRequirement(viewModelLifecycle, ViewModelLifecycle.class);
      return new ViewModelCImpl(singletonCImpl, activityRetainedCImpl, savedStateHandle, viewModelLifecycle);
    }
  }

  private static final class ServiceCBuilder implements PlumTvApplication_HiltComponents.ServiceC.Builder {
    private final SingletonCImpl singletonCImpl;

    private Service service;

    private ServiceCBuilder(SingletonCImpl singletonCImpl) {
      this.singletonCImpl = singletonCImpl;
    }

    @Override
    public ServiceCBuilder service(Service service) {
      this.service = Preconditions.checkNotNull(service);
      return this;
    }

    @Override
    public PlumTvApplication_HiltComponents.ServiceC build() {
      Preconditions.checkBuilderRequirement(service, Service.class);
      return new ServiceCImpl(singletonCImpl, service);
    }
  }

  private static final class ViewWithFragmentCImpl extends PlumTvApplication_HiltComponents.ViewWithFragmentC {
    private final SingletonCImpl singletonCImpl;

    private final ActivityRetainedCImpl activityRetainedCImpl;

    private final ActivityCImpl activityCImpl;

    private final FragmentCImpl fragmentCImpl;

    private final ViewWithFragmentCImpl viewWithFragmentCImpl = this;

    ViewWithFragmentCImpl(SingletonCImpl singletonCImpl,
        ActivityRetainedCImpl activityRetainedCImpl, ActivityCImpl activityCImpl,
        FragmentCImpl fragmentCImpl, View viewParam) {
      this.singletonCImpl = singletonCImpl;
      this.activityRetainedCImpl = activityRetainedCImpl;
      this.activityCImpl = activityCImpl;
      this.fragmentCImpl = fragmentCImpl;


    }
  }

  private static final class FragmentCImpl extends PlumTvApplication_HiltComponents.FragmentC {
    private final SingletonCImpl singletonCImpl;

    private final ActivityRetainedCImpl activityRetainedCImpl;

    private final ActivityCImpl activityCImpl;

    private final FragmentCImpl fragmentCImpl = this;

    FragmentCImpl(SingletonCImpl singletonCImpl, ActivityRetainedCImpl activityRetainedCImpl,
        ActivityCImpl activityCImpl, Fragment fragmentParam) {
      this.singletonCImpl = singletonCImpl;
      this.activityRetainedCImpl = activityRetainedCImpl;
      this.activityCImpl = activityCImpl;


    }

    @Override
    public DefaultViewModelFactories.InternalFactoryFactory getHiltInternalFactoryFactory() {
      return activityCImpl.getHiltInternalFactoryFactory();
    }

    @Override
    public ViewWithFragmentComponentBuilder viewWithFragmentComponentBuilder() {
      return new ViewWithFragmentCBuilder(singletonCImpl, activityRetainedCImpl, activityCImpl, fragmentCImpl);
    }
  }

  private static final class ViewCImpl extends PlumTvApplication_HiltComponents.ViewC {
    private final SingletonCImpl singletonCImpl;

    private final ActivityRetainedCImpl activityRetainedCImpl;

    private final ActivityCImpl activityCImpl;

    private final ViewCImpl viewCImpl = this;

    ViewCImpl(SingletonCImpl singletonCImpl, ActivityRetainedCImpl activityRetainedCImpl,
        ActivityCImpl activityCImpl, View viewParam) {
      this.singletonCImpl = singletonCImpl;
      this.activityRetainedCImpl = activityRetainedCImpl;
      this.activityCImpl = activityCImpl;


    }
  }

  private static final class ActivityCImpl extends PlumTvApplication_HiltComponents.ActivityC {
    private final SingletonCImpl singletonCImpl;

    private final ActivityRetainedCImpl activityRetainedCImpl;

    private final ActivityCImpl activityCImpl = this;

    ActivityCImpl(SingletonCImpl singletonCImpl, ActivityRetainedCImpl activityRetainedCImpl,
        Activity activityParam) {
      this.singletonCImpl = singletonCImpl;
      this.activityRetainedCImpl = activityRetainedCImpl;


    }

    ImmutableMap keySetMapOfClassOfAndBooleanBuilder() {
      ImmutableMap.Builder mapBuilder = ImmutableMap.<String, Boolean>builderWithExpectedSize(13);
      mapBuilder.put(AuthViewModel_HiltModules_KeyModule_Provide_LazyMapKey.lazyClassKeyName, AuthViewModel_HiltModules.KeyModule.provide());
      mapBuilder.put(DiscoverBrowseViewModel_HiltModules_KeyModule_Provide_LazyMapKey.lazyClassKeyName, DiscoverBrowseViewModel_HiltModules.KeyModule.provide());
      mapBuilder.put(DiscoverDetailViewModel_HiltModules_KeyModule_Provide_LazyMapKey.lazyClassKeyName, DiscoverDetailViewModel_HiltModules.KeyModule.provide());
      mapBuilder.put(DiscoverViewModel_HiltModules_KeyModule_Provide_LazyMapKey.lazyClassKeyName, DiscoverViewModel_HiltModules.KeyModule.provide());
      mapBuilder.put(DownloadsViewModel_HiltModules_KeyModule_Provide_LazyMapKey.lazyClassKeyName, DownloadsViewModel_HiltModules.KeyModule.provide());
      mapBuilder.put(HomeViewModel_HiltModules_KeyModule_Provide_LazyMapKey.lazyClassKeyName, HomeViewModel_HiltModules.KeyModule.provide());
      mapBuilder.put(LibraryBrowseViewModel_HiltModules_KeyModule_Provide_LazyMapKey.lazyClassKeyName, LibraryBrowseViewModel_HiltModules.KeyModule.provide());
      mapBuilder.put(LibraryListViewModel_HiltModules_KeyModule_Provide_LazyMapKey.lazyClassKeyName, LibraryListViewModel_HiltModules.KeyModule.provide());
      mapBuilder.put(MainNavViewModel_HiltModules_KeyModule_Provide_LazyMapKey.lazyClassKeyName, MainNavViewModel_HiltModules.KeyModule.provide());
      mapBuilder.put(MovieDetailViewModel_HiltModules_KeyModule_Provide_LazyMapKey.lazyClassKeyName, MovieDetailViewModel_HiltModules.KeyModule.provide());
      mapBuilder.put(PlayerViewModel_HiltModules_KeyModule_Provide_LazyMapKey.lazyClassKeyName, PlayerViewModel_HiltModules.KeyModule.provide());
      mapBuilder.put(SearchViewModel_HiltModules_KeyModule_Provide_LazyMapKey.lazyClassKeyName, SearchViewModel_HiltModules.KeyModule.provide());
      mapBuilder.put(ShowDetailViewModel_HiltModules_KeyModule_Provide_LazyMapKey.lazyClassKeyName, ShowDetailViewModel_HiltModules.KeyModule.provide());
      return mapBuilder.build();
    }

    @Override
    public DefaultViewModelFactories.InternalFactoryFactory getHiltInternalFactoryFactory() {
      return DefaultViewModelFactories_InternalFactoryFactory_Factory.newInstance(getViewModelKeys(), new ViewModelCBuilder(singletonCImpl, activityRetainedCImpl));
    }

    @Override
    public Map<Class<?>, Boolean> getViewModelKeys() {
      return LazyClassKeyMap.<Boolean>of(keySetMapOfClassOfAndBooleanBuilder());
    }

    @Override
    public ViewModelComponentBuilder getViewModelComponentBuilder() {
      return new ViewModelCBuilder(singletonCImpl, activityRetainedCImpl);
    }

    @Override
    public FragmentComponentBuilder fragmentComponentBuilder() {
      return new FragmentCBuilder(singletonCImpl, activityRetainedCImpl, activityCImpl);
    }

    @Override
    public ViewComponentBuilder viewComponentBuilder() {
      return new ViewCBuilder(singletonCImpl, activityRetainedCImpl, activityCImpl);
    }

    @Override
    public void injectMainActivity(MainActivity arg0) {
      injectMainActivity2(arg0);
    }

    private MainActivity injectMainActivity2(MainActivity instance) {
      MainActivity_MembersInjector.injectWebSocketManager(instance, singletonCImpl.plumWebSocketManagerProvider.get());
      MainActivity_MembersInjector.injectLibraryScanStatusPoller(instance, singletonCImpl.libraryScanStatusPollerProvider.get());
      MainActivity_MembersInjector.injectSessionPreferences(instance, singletonCImpl.sessionPreferencesProvider.get());
      return instance;
    }
  }

  private static final class ViewModelCImpl extends PlumTvApplication_HiltComponents.ViewModelC {
    private final SavedStateHandle savedStateHandle;

    private final SingletonCImpl singletonCImpl;

    private final ActivityRetainedCImpl activityRetainedCImpl;

    private final ViewModelCImpl viewModelCImpl = this;

    Provider<AuthViewModel> authViewModelProvider;

    Provider<DiscoverBrowseViewModel> discoverBrowseViewModelProvider;

    Provider<DiscoverDetailViewModel> discoverDetailViewModelProvider;

    Provider<DiscoverViewModel> discoverViewModelProvider;

    Provider<DownloadsViewModel> downloadsViewModelProvider;

    Provider<HomeViewModel> homeViewModelProvider;

    Provider<LibraryBrowseViewModel> libraryBrowseViewModelProvider;

    Provider<LibraryListViewModel> libraryListViewModelProvider;

    Provider<MainNavViewModel> mainNavViewModelProvider;

    Provider<MovieDetailViewModel> movieDetailViewModelProvider;

    Provider<PlayerViewModel> playerViewModelProvider;

    Provider<SearchViewModel> searchViewModelProvider;

    Provider<ShowDetailViewModel> showDetailViewModelProvider;

    ViewModelCImpl(SingletonCImpl singletonCImpl, ActivityRetainedCImpl activityRetainedCImpl,
        SavedStateHandle savedStateHandleParam, ViewModelLifecycle viewModelLifecycleParam) {
      this.singletonCImpl = singletonCImpl;
      this.activityRetainedCImpl = activityRetainedCImpl;
      this.savedStateHandle = savedStateHandleParam;
      initialize(savedStateHandleParam, viewModelLifecycleParam);

    }

    ImmutableMap hiltViewModelMapMapOfClassOfAndProviderOfViewModelBuilder() {
      ImmutableMap.Builder mapBuilder = ImmutableMap.<String, javax.inject.Provider<ViewModel>>builderWithExpectedSize(13);
      mapBuilder.put(AuthViewModel_HiltModules_BindsModule_Binds_LazyMapKey.lazyClassKeyName, ((Provider) (authViewModelProvider)));
      mapBuilder.put(DiscoverBrowseViewModel_HiltModules_BindsModule_Binds_LazyMapKey.lazyClassKeyName, ((Provider) (discoverBrowseViewModelProvider)));
      mapBuilder.put(DiscoverDetailViewModel_HiltModules_BindsModule_Binds_LazyMapKey.lazyClassKeyName, ((Provider) (discoverDetailViewModelProvider)));
      mapBuilder.put(DiscoverViewModel_HiltModules_BindsModule_Binds_LazyMapKey.lazyClassKeyName, ((Provider) (discoverViewModelProvider)));
      mapBuilder.put(DownloadsViewModel_HiltModules_BindsModule_Binds_LazyMapKey.lazyClassKeyName, ((Provider) (downloadsViewModelProvider)));
      mapBuilder.put(HomeViewModel_HiltModules_BindsModule_Binds_LazyMapKey.lazyClassKeyName, ((Provider) (homeViewModelProvider)));
      mapBuilder.put(LibraryBrowseViewModel_HiltModules_BindsModule_Binds_LazyMapKey.lazyClassKeyName, ((Provider) (libraryBrowseViewModelProvider)));
      mapBuilder.put(LibraryListViewModel_HiltModules_BindsModule_Binds_LazyMapKey.lazyClassKeyName, ((Provider) (libraryListViewModelProvider)));
      mapBuilder.put(MainNavViewModel_HiltModules_BindsModule_Binds_LazyMapKey.lazyClassKeyName, ((Provider) (mainNavViewModelProvider)));
      mapBuilder.put(MovieDetailViewModel_HiltModules_BindsModule_Binds_LazyMapKey.lazyClassKeyName, ((Provider) (movieDetailViewModelProvider)));
      mapBuilder.put(PlayerViewModel_HiltModules_BindsModule_Binds_LazyMapKey.lazyClassKeyName, ((Provider) (playerViewModelProvider)));
      mapBuilder.put(SearchViewModel_HiltModules_BindsModule_Binds_LazyMapKey.lazyClassKeyName, ((Provider) (searchViewModelProvider)));
      mapBuilder.put(ShowDetailViewModel_HiltModules_BindsModule_Binds_LazyMapKey.lazyClassKeyName, ((Provider) (showDetailViewModelProvider)));
      return mapBuilder.build();
    }

    @SuppressWarnings("unchecked")
    private void initialize(final SavedStateHandle savedStateHandleParam,
        final ViewModelLifecycle viewModelLifecycleParam) {
      this.authViewModelProvider = new SwitchingProvider<>(singletonCImpl, activityRetainedCImpl, viewModelCImpl, 0);
      this.discoverBrowseViewModelProvider = new SwitchingProvider<>(singletonCImpl, activityRetainedCImpl, viewModelCImpl, 1);
      this.discoverDetailViewModelProvider = new SwitchingProvider<>(singletonCImpl, activityRetainedCImpl, viewModelCImpl, 2);
      this.discoverViewModelProvider = new SwitchingProvider<>(singletonCImpl, activityRetainedCImpl, viewModelCImpl, 3);
      this.downloadsViewModelProvider = new SwitchingProvider<>(singletonCImpl, activityRetainedCImpl, viewModelCImpl, 4);
      this.homeViewModelProvider = new SwitchingProvider<>(singletonCImpl, activityRetainedCImpl, viewModelCImpl, 5);
      this.libraryBrowseViewModelProvider = new SwitchingProvider<>(singletonCImpl, activityRetainedCImpl, viewModelCImpl, 6);
      this.libraryListViewModelProvider = new SwitchingProvider<>(singletonCImpl, activityRetainedCImpl, viewModelCImpl, 7);
      this.mainNavViewModelProvider = new SwitchingProvider<>(singletonCImpl, activityRetainedCImpl, viewModelCImpl, 8);
      this.movieDetailViewModelProvider = new SwitchingProvider<>(singletonCImpl, activityRetainedCImpl, viewModelCImpl, 9);
      this.playerViewModelProvider = new SwitchingProvider<>(singletonCImpl, activityRetainedCImpl, viewModelCImpl, 10);
      this.searchViewModelProvider = new SwitchingProvider<>(singletonCImpl, activityRetainedCImpl, viewModelCImpl, 11);
      this.showDetailViewModelProvider = new SwitchingProvider<>(singletonCImpl, activityRetainedCImpl, viewModelCImpl, 12);
    }

    @Override
    public Map<Class<?>, javax.inject.Provider<ViewModel>> getHiltViewModelMap() {
      return LazyClassKeyMap.<javax.inject.Provider<ViewModel>>of(hiltViewModelMapMapOfClassOfAndProviderOfViewModelBuilder());
    }

    @Override
    public Map<Class<?>, Object> getHiltViewModelAssistedMap() {
      return ImmutableMap.<Class<?>, Object>of();
    }

    private static final class SwitchingProvider<T> implements Provider<T> {
      private final SingletonCImpl singletonCImpl;

      private final ActivityRetainedCImpl activityRetainedCImpl;

      private final ViewModelCImpl viewModelCImpl;

      private final int id;

      SwitchingProvider(SingletonCImpl singletonCImpl, ActivityRetainedCImpl activityRetainedCImpl,
          ViewModelCImpl viewModelCImpl, int id) {
        this.singletonCImpl = singletonCImpl;
        this.activityRetainedCImpl = activityRetainedCImpl;
        this.viewModelCImpl = viewModelCImpl;
        this.id = id;
      }

      @Override
      @SuppressWarnings("unchecked")
      public T get() {
        switch (id) {
          case 0: // plum.tv.feature.auth.AuthViewModel
          return (T) new AuthViewModel(singletonCImpl.sessionRepositoryProvider.get(), singletonCImpl.browseRepositoryProvider.get(), singletonCImpl.libraryCatalogRefreshCoordinatorProvider.get());

          case 1: // plum.tv.feature.discover.DiscoverBrowseViewModel
          return (T) new DiscoverBrowseViewModel(singletonCImpl.discoverRepositoryProvider.get(), singletonCImpl.libraryCatalogRefreshCoordinatorProvider.get());

          case 2: // plum.tv.feature.discover.DiscoverDetailViewModel
          return (T) new DiscoverDetailViewModel(singletonCImpl.discoverRepositoryProvider.get(), singletonCImpl.libraryCatalogRefreshCoordinatorProvider.get());

          case 3: // plum.tv.feature.discover.DiscoverViewModel
          return (T) new DiscoverViewModel(singletonCImpl.discoverRepositoryProvider.get(), singletonCImpl.libraryCatalogRefreshCoordinatorProvider.get());

          case 4: // plum.tv.feature.discover.DownloadsViewModel
          return (T) new DownloadsViewModel(singletonCImpl.discoverRepositoryProvider.get());

          case 5: // plum.tv.feature.home.HomeViewModel
          return (T) new HomeViewModel(singletonCImpl.browseRepositoryProvider.get(), singletonCImpl.homeDashboardDiskCacheProvider.get(), singletonCImpl.sessionPreferencesProvider.get(), singletonCImpl.libraryCatalogRefreshCoordinatorProvider.get());

          case 6: // plum.tv.feature.library.LibraryBrowseViewModel
          return (T) new LibraryBrowseViewModel(singletonCImpl.browseRepositoryProvider.get(), singletonCImpl.libraryCatalogRefreshCoordinatorProvider.get(), viewModelCImpl.savedStateHandle);

          case 7: // plum.tv.feature.library.LibraryListViewModel
          return (T) new LibraryListViewModel(singletonCImpl.browseRepositoryProvider.get(), singletonCImpl.libraryCatalogRefreshCoordinatorProvider.get());

          case 8: // plum.tv.app.MainNavViewModel
          return (T) new MainNavViewModel(singletonCImpl.browseRepositoryProvider.get());

          case 9: // plum.tv.feature.details.MovieDetailViewModel
          return (T) new MovieDetailViewModel(singletonCImpl.browseRepositoryProvider.get(), singletonCImpl.libraryCatalogRefreshCoordinatorProvider.get(), viewModelCImpl.savedStateHandle);

          case 10: // plum.tv.app.PlayerViewModel
          return (T) new PlayerViewModel(ApplicationContextModule_ProvideContextFactory.provideContext(singletonCImpl.applicationContextModule), singletonCImpl.provideApplicationScopeProvider.get(), singletonCImpl.providePlumMediaDataSourceFactoryProvider.get(), singletonCImpl.browseRepositoryProvider.get(), singletonCImpl.playbackRepositoryProvider.get(), singletonCImpl.plumWebSocketManagerProvider.get(), singletonCImpl.playerSubtitlePreferencesProvider.get(), viewModelCImpl.savedStateHandle);

          case 11: // plum.tv.feature.search.SearchViewModel
          return (T) new SearchViewModel(singletonCImpl.searchRepositoryProvider.get());

          case 12: // plum.tv.feature.details.ShowDetailViewModel
          return (T) new ShowDetailViewModel(singletonCImpl.browseRepositoryProvider.get(), singletonCImpl.libraryCatalogRefreshCoordinatorProvider.get(), viewModelCImpl.savedStateHandle);

          default: throw new AssertionError(id);
        }
      }
    }
  }

  private static final class ActivityRetainedCImpl extends PlumTvApplication_HiltComponents.ActivityRetainedC {
    private final SingletonCImpl singletonCImpl;

    private final ActivityRetainedCImpl activityRetainedCImpl = this;

    Provider<ActivityRetainedLifecycle> provideActivityRetainedLifecycleProvider;

    ActivityRetainedCImpl(SingletonCImpl singletonCImpl,
        SavedStateHandleHolder savedStateHandleHolderParam) {
      this.singletonCImpl = singletonCImpl;

      initialize(savedStateHandleHolderParam);

    }

    @SuppressWarnings("unchecked")
    private void initialize(final SavedStateHandleHolder savedStateHandleHolderParam) {
      this.provideActivityRetainedLifecycleProvider = DoubleCheck.provider(new SwitchingProvider<ActivityRetainedLifecycle>(singletonCImpl, activityRetainedCImpl, 0));
    }

    @Override
    public ActivityComponentBuilder activityComponentBuilder() {
      return new ActivityCBuilder(singletonCImpl, activityRetainedCImpl);
    }

    @Override
    public ActivityRetainedLifecycle getActivityRetainedLifecycle() {
      return provideActivityRetainedLifecycleProvider.get();
    }

    private static final class SwitchingProvider<T> implements Provider<T> {
      private final SingletonCImpl singletonCImpl;

      private final ActivityRetainedCImpl activityRetainedCImpl;

      private final int id;

      SwitchingProvider(SingletonCImpl singletonCImpl, ActivityRetainedCImpl activityRetainedCImpl,
          int id) {
        this.singletonCImpl = singletonCImpl;
        this.activityRetainedCImpl = activityRetainedCImpl;
        this.id = id;
      }

      @Override
      @SuppressWarnings("unchecked")
      public T get() {
        switch (id) {
          case 0: // dagger.hilt.android.ActivityRetainedLifecycle
          return (T) ActivityRetainedComponentManager_LifecycleModule_ProvideActivityRetainedLifecycleFactory.provideActivityRetainedLifecycle();

          default: throw new AssertionError(id);
        }
      }
    }
  }

  private static final class ServiceCImpl extends PlumTvApplication_HiltComponents.ServiceC {
    private final SingletonCImpl singletonCImpl;

    private final ServiceCImpl serviceCImpl = this;

    ServiceCImpl(SingletonCImpl singletonCImpl, Service serviceParam) {
      this.singletonCImpl = singletonCImpl;


    }
  }

  private static final class SingletonCImpl extends PlumTvApplication_HiltComponents.SingletonC {
    private final ApplicationContextModule applicationContextModule;

    private final SingletonCImpl singletonCImpl = this;

    Provider<AuthTokenBridge> authTokenBridgeProvider;

    Provider<OkHttpClient> providePlumOkHttpClientProvider;

    Provider<CoroutineScope> provideApplicationScopeProvider;

    Provider<SessionPreferences> sessionPreferencesProvider;

    Provider<Moshi> provideMoshiProvider;

    Provider<SessionRepository> sessionRepositoryProvider;

    Provider<HomeDashboardDiskCache> homeDashboardDiskCacheProvider;

    Provider<BrowseRepository> browseRepositoryProvider;

    Provider<LibraryCatalogRefreshCoordinator> libraryCatalogRefreshCoordinatorProvider;

    Provider<PlumWebSocketManager> plumWebSocketManagerProvider;

    Provider<LibraryScanStatusPoller> libraryScanStatusPollerProvider;

    Provider<DiscoverRepository> discoverRepositoryProvider;

    Provider<DataSource.Factory> providePlumMediaDataSourceFactoryProvider;

    Provider<PlaybackRepository> playbackRepositoryProvider;

    Provider<PlayerSubtitlePreferences> playerSubtitlePreferencesProvider;

    Provider<SearchRepository> searchRepositoryProvider;

    SingletonCImpl(ApplicationContextModule applicationContextModuleParam) {
      this.applicationContextModule = applicationContextModuleParam;
      initialize(applicationContextModuleParam);

    }

    @SuppressWarnings("unchecked")
    private void initialize(final ApplicationContextModule applicationContextModuleParam) {
      this.authTokenBridgeProvider = DoubleCheck.provider(new SwitchingProvider<AuthTokenBridge>(singletonCImpl, 1));
      this.providePlumOkHttpClientProvider = DoubleCheck.provider(new SwitchingProvider<OkHttpClient>(singletonCImpl, 0));
      this.provideApplicationScopeProvider = DoubleCheck.provider(new SwitchingProvider<CoroutineScope>(singletonCImpl, 4));
      this.sessionPreferencesProvider = DoubleCheck.provider(new SwitchingProvider<SessionPreferences>(singletonCImpl, 3));
      this.provideMoshiProvider = DoubleCheck.provider(new SwitchingProvider<Moshi>(singletonCImpl, 5));
      this.sessionRepositoryProvider = DoubleCheck.provider(new SwitchingProvider<SessionRepository>(singletonCImpl, 8));
      this.homeDashboardDiskCacheProvider = DoubleCheck.provider(new SwitchingProvider<HomeDashboardDiskCache>(singletonCImpl, 9));
      this.browseRepositoryProvider = DoubleCheck.provider(new SwitchingProvider<BrowseRepository>(singletonCImpl, 7));
      this.libraryCatalogRefreshCoordinatorProvider = DoubleCheck.provider(new SwitchingProvider<LibraryCatalogRefreshCoordinator>(singletonCImpl, 6));
      this.plumWebSocketManagerProvider = DoubleCheck.provider(new SwitchingProvider<PlumWebSocketManager>(singletonCImpl, 2));
      this.libraryScanStatusPollerProvider = DoubleCheck.provider(new SwitchingProvider<LibraryScanStatusPoller>(singletonCImpl, 10));
      this.discoverRepositoryProvider = DoubleCheck.provider(new SwitchingProvider<DiscoverRepository>(singletonCImpl, 11));
      this.providePlumMediaDataSourceFactoryProvider = DoubleCheck.provider(new SwitchingProvider<DataSource.Factory>(singletonCImpl, 12));
      this.playbackRepositoryProvider = DoubleCheck.provider(new SwitchingProvider<PlaybackRepository>(singletonCImpl, 13));
      this.playerSubtitlePreferencesProvider = DoubleCheck.provider(new SwitchingProvider<PlayerSubtitlePreferences>(singletonCImpl, 14));
      this.searchRepositoryProvider = DoubleCheck.provider(new SwitchingProvider<SearchRepository>(singletonCImpl, 15));
    }

    @Override
    public Set<Boolean> getDisableFragmentGetContextFix() {
      return ImmutableSet.<Boolean>of();
    }

    @Override
    public ActivityRetainedComponentBuilder retainedComponentBuilder() {
      return new ActivityRetainedCBuilder(singletonCImpl);
    }

    @Override
    public ServiceComponentBuilder serviceComponentBuilder() {
      return new ServiceCBuilder(singletonCImpl);
    }

    @Override
    public OkHttpClient okHttpClient() {
      return providePlumOkHttpClientProvider.get();
    }

    @Override
    public void injectPlumTvApplication(PlumTvApplication plumTvApplication) {
    }

    private static final class SwitchingProvider<T> implements Provider<T> {
      private final SingletonCImpl singletonCImpl;

      private final int id;

      SwitchingProvider(SingletonCImpl singletonCImpl, int id) {
        this.singletonCImpl = singletonCImpl;
        this.id = id;
      }

      @Override
      @SuppressWarnings("unchecked")
      public T get() {
        switch (id) {
          case 0: // okhttp3.OkHttpClient
          return (T) NetworkModule_ProvidePlumOkHttpClientFactory.providePlumOkHttpClient(ApplicationContextModule_ProvideContextFactory.provideContext(singletonCImpl.applicationContextModule), singletonCImpl.authTokenBridgeProvider.get());

          case 1: // plum.tv.core.data.AuthTokenBridge
          return (T) new AuthTokenBridge();

          case 2: // plum.tv.core.data.PlumWebSocketManager
          return (T) new PlumWebSocketManager(singletonCImpl.providePlumOkHttpClientProvider.get(), singletonCImpl.sessionPreferencesProvider.get(), singletonCImpl.authTokenBridgeProvider.get(), singletonCImpl.provideMoshiProvider.get(), singletonCImpl.libraryCatalogRefreshCoordinatorProvider.get());

          case 3: // plum.tv.core.data.SessionPreferences
          return (T) new SessionPreferences(ApplicationContextModule_ProvideContextFactory.provideContext(singletonCImpl.applicationContextModule), singletonCImpl.provideApplicationScopeProvider.get());

          case 4: // @plum.tv.core.data.di.ApplicationScope kotlinx.coroutines.CoroutineScope
          return (T) MediaModule_ProvideApplicationScopeFactory.provideApplicationScope();

          case 5: // com.squareup.moshi.Moshi
          return (T) DataModule_ProvideMoshiFactory.provideMoshi();

          case 6: // plum.tv.core.data.LibraryCatalogRefreshCoordinator
          return (T) new LibraryCatalogRefreshCoordinator(singletonCImpl.browseRepositoryProvider.get());

          case 7: // plum.tv.core.data.BrowseRepository
          return (T) new BrowseRepository(singletonCImpl.sessionRepositoryProvider.get(), singletonCImpl.homeDashboardDiskCacheProvider.get());

          case 8: // plum.tv.core.data.SessionRepository
          return (T) new SessionRepository(singletonCImpl.sessionPreferencesProvider.get(), singletonCImpl.provideMoshiProvider.get(), singletonCImpl.authTokenBridgeProvider.get(), singletonCImpl.providePlumOkHttpClientProvider.get());

          case 9: // plum.tv.core.data.HomeDashboardDiskCache
          return (T) new HomeDashboardDiskCache(ApplicationContextModule_ProvideContextFactory.provideContext(singletonCImpl.applicationContextModule), singletonCImpl.provideMoshiProvider.get());

          case 10: // plum.tv.core.data.LibraryScanStatusPoller
          return (T) new LibraryScanStatusPoller(singletonCImpl.sessionRepositoryProvider.get(), singletonCImpl.libraryCatalogRefreshCoordinatorProvider.get());

          case 11: // plum.tv.core.data.DiscoverRepository
          return (T) new DiscoverRepository(singletonCImpl.sessionRepositoryProvider.get());

          case 12: // androidx.media3.datasource.DataSource.Factory
          return (T) MediaModule_ProvidePlumMediaDataSourceFactoryFactory.providePlumMediaDataSourceFactory(singletonCImpl.providePlumOkHttpClientProvider.get());

          case 13: // plum.tv.core.data.PlaybackRepository
          return (T) new PlaybackRepository(singletonCImpl.sessionRepositoryProvider.get(), singletonCImpl.providePlumOkHttpClientProvider.get());

          case 14: // plum.tv.core.data.PlayerSubtitlePreferences
          return (T) new PlayerSubtitlePreferences(ApplicationContextModule_ProvideContextFactory.provideContext(singletonCImpl.applicationContextModule), singletonCImpl.provideApplicationScopeProvider.get());

          case 15: // plum.tv.core.data.SearchRepository
          return (T) new SearchRepository(singletonCImpl.sessionRepositoryProvider.get());

          default: throw new AssertionError(id);
        }
      }
    }
  }
}
