package plum.tv.app

import android.net.Uri
import androidx.activity.ComponentActivity
import androidx.activity.compose.BackHandler
import androidx.activity.compose.LocalActivity
import androidx.compose.foundation.focusGroup
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.Alignment
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusProperties
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.unit.dp
import androidx.compose.ui.zIndex
import androidx.hilt.lifecycle.viewmodel.compose.hiltViewModel
import androidx.media3.common.util.UnstableApi
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Animation
import androidx.compose.material.icons.filled.Download
import androidx.compose.material.icons.filled.Explore
import androidx.compose.material.icons.filled.Home
import androidx.compose.material.icons.automirrored.filled.Logout
import androidx.compose.material.icons.filled.Movie
import androidx.compose.material.icons.filled.MusicNote
import androidx.compose.material.icons.filled.Search
import androidx.compose.material.icons.filled.Tv
import androidx.navigation.NavGraph.Companion.findStartDestination
import androidx.navigation.NavHostController
import androidx.navigation.NavType
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import androidx.navigation.navArgument
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import plum.tv.core.data.LibraryScanStatusPoller
import plum.tv.core.data.PlumWebSocketManager
import plum.tv.core.ui.PlumActionButton
import plum.tv.core.ui.PlumButtonVariant
import plum.tv.core.ui.PlumRailItem
import plum.tv.core.ui.PlumSideRail
import plum.tv.core.ui.PlumTvScaffold
import plum.tv.feature.discover.DiscoverBrowseRoute
import plum.tv.feature.discover.DiscoverDetailRoute
import plum.tv.feature.discover.DiscoverRoute
import plum.tv.feature.discover.DownloadsRoute
import plum.tv.feature.details.MovieDetailRoute
import plum.tv.feature.details.ShowDetailRoute
import plum.tv.feature.details.ShowDetailViewModel
import plum.tv.feature.home.HomeRoute
import plum.tv.feature.auth.AuthViewModel
import plum.tv.feature.library.LibraryBrowseRoute
import plum.tv.feature.library.LibraryHubRoute
import plum.tv.feature.library.LibraryListRoute
import plum.tv.feature.search.SearchRoute
import plum.tv.feature.settings.SettingsRoute

private object Routes {
    const val HOME = "home"
    const val SEARCH = "search"
    /** Not `discover` alone — must not collide with `discover/{mediaType}/{tmdbId}` in NavHost matching. */
    const val DISCOVER = "discover/main"
    const val DISCOVER_BROWSE = "discover/browse?category={category}&mediaType={mediaType}&genre={genre}"
    const val DISCOVER_DETAIL = "discover/{mediaType}/{tmdbId}"
    const val DOWNLOADS = "downloads"
    const val LIBRARIES = "libraries"
    const val LIBRARY_TYPE = "libraries/type/{libraryType}"
    const val HUB = "hub/{libraryType}"
    const val LIBRARY_BROWSE = "library/{libraryId}/browse"
    const val MOVIE = "movie/{libraryId}/{mediaId}"
    const val SHOW = "show/{libraryId}/{showKey}"
    const val PLAY =
        "play/{mediaId}?resume={resume}&libraryId={libraryId}&showKey={showKey}" +
            "&displayTitle={displayTitle}&displaySubtitle={displaySubtitle}"
}

@UnstableApi
@Composable
fun MainNavHost(
    webSocketManager: PlumWebSocketManager,
    libraryScanStatusPoller: LibraryScanStatusPoller,
    defaultServerUrl: String,
    onLogout: () -> Unit,
) {
    val navController = rememberNavController()
    val navBackStackEntry by navController.currentBackStackEntryAsState()
    val currentRoute = navBackStackEntry?.destination?.route.orEmpty()
    val currentLibraryType = navBackStackEntry?.arguments?.getString("libraryType")
    val hideSideRail = currentRoute.startsWith("play/")
    val scope = rememberCoroutineScope()
    val activity = LocalActivity.current as ComponentActivity
    val mainNavVm: MainNavViewModel = hiltViewModel(viewModelStoreOwner = activity)
    val authVm: AuthViewModel = hiltViewModel(viewModelStoreOwner = activity)
    val mainContentFocusRequester = remember { FocusRequester() }
    val homeRailFocusRequester = remember { FocusRequester() }
    var browseRailType by remember { mutableStateOf<String?>(null) }
    LaunchedEffect(navBackStackEntry) {
        val entry = navBackStackEntry
        when {
            entry?.destination?.route?.startsWith("hub/") == true -> {
                browseRailType = entry.arguments?.getString("libraryType")
            }
            entry?.destination?.route == Routes.LIBRARY_TYPE -> {
                browseRailType = entry.arguments?.getString("libraryType")
            }
            entry?.destination?.route == Routes.LIBRARY_BROWSE -> {
                val id = entry.arguments?.getInt("libraryId")
                browseRailType = if (id != null) mainNavVm.railTypeForBrowseLibraryId(id) else null
            }
            else -> browseRailType = null
        }
    }

    LaunchedEffect(navBackStackEntry?.id, hideSideRail, currentRoute) {
        if (hideSideRail) return@LaunchedEffect
        delay(16)
        if (currentRoute == Routes.HOME) {
            homeRailFocusRequester.requestFocus()
        } else {
            mainContentFocusRequester.requestFocus()
        }
    }

    DisposableEffect(webSocketManager) {
        webSocketManager.start()
        onDispose { webSocketManager.stop() }
    }

    DisposableEffect(libraryScanStatusPoller) {
        libraryScanStatusPoller.start(scope)
        onDispose { libraryScanStatusPoller.stop() }
    }

    LaunchedEffect(Unit) {
        mainNavVm.scheduleLibraryMediaPrefetch()
    }

    fun navigatePlay(
        mediaId: Int,
        resumeSec: Float = 0f,
        libraryId: Int? = null,
        showKey: String? = null,
        displayTitle: String? = null,
        displaySubtitle: String? = null,
    ) {
        val params = buildList {
            add("resume=$resumeSec")
            add("libraryId=${libraryId ?: -1}")
            add("showKey=${Uri.encode(showKey.orEmpty())}")
            add("displayTitle=${Uri.encode(displayTitle.orEmpty())}")
            add("displaySubtitle=${Uri.encode(displaySubtitle.orEmpty())}")
        }
        navController.navigate("play/$mediaId?${params.joinToString("&")}")
    }

    val railItems =
        listOf(
            PlumRailItem(
                key = Routes.HOME,
                label = "Home",
                icon = Icons.Filled.Home,
                selected = currentRoute == Routes.HOME,
                onClick = { goToRoot(navController, Routes.HOME) },
            ),
            PlumRailItem(
                key = Routes.SEARCH,
                label = "Search",
                icon = Icons.Filled.Search,
                selected = currentRoute == Routes.SEARCH,
                onClick = { goToRoot(navController, Routes.SEARCH) },
            ),
            PlumRailItem(
                key = Routes.DISCOVER,
                label = "Discover",
                icon = Icons.Filled.Explore,
                selected =
                    currentRoute == Routes.DISCOVER ||
                        currentRoute.startsWith("discover/browse") ||
                        currentRoute == Routes.DISCOVER_DETAIL,
                onClick = { goToRoot(navController, Routes.DISCOVER) },
            ),
            PlumRailItem(
                key = Routes.DOWNLOADS,
                label = "Downloads",
                icon = Icons.Filled.Download,
                selected = currentRoute == Routes.DOWNLOADS || currentRoute.startsWith("downloads"),
                onClick = { goToRoot(navController, Routes.DOWNLOADS) },
                dividerAfter = true,
            ),
            PlumRailItem(
                key = "library-tv",
                label = "TV",
                icon = Icons.Filled.Tv,
                selected =
                    (currentRoute.startsWith("libraries/type/") && currentLibraryType == "tv") ||
                        browseRailType == "tv",
                onClick = {
                    openLibraryTypeFromRail(scope, navController, mainNavVm, "tv")
                },
            ),
            PlumRailItem(
                key = "library-movies",
                label = "Movies",
                icon = Icons.Filled.Movie,
                selected =
                    (currentRoute.startsWith("libraries/type/") && currentLibraryType == "movie") ||
                        browseRailType == "movie",
                onClick = {
                    openLibraryTypeFromRail(scope, navController, mainNavVm, "movie")
                },
            ),
            PlumRailItem(
                key = "library-anime",
                label = "Anime",
                icon = Icons.Filled.Animation,
                selected =
                    (currentRoute.startsWith("libraries/type/") && currentLibraryType == "anime") ||
                        browseRailType == "anime",
                onClick = {
                    openLibraryTypeFromRail(scope, navController, mainNavVm, "anime")
                },
            ),
            PlumRailItem(
                key = "library-music",
                label = "Music",
                icon = Icons.Filled.MusicNote,
                selected =
                    (currentRoute.startsWith("libraries/type/") && currentLibraryType == "music") ||
                        browseRailType == "music",
                onClick = {
                    openLibraryTypeFromRail(scope, navController, mainNavVm, "music")
                },
            ),
        )

    PlumTvScaffold {
        Box(Modifier.fillMaxSize()) {
            // Lower priority than child screens (e.g. player): pop nav first, then finish the task.
            BackHandler {
                if (!navController.popBackStack()) {
                    activity.finish()
                }
            }
            Row(Modifier.fillMaxSize()) {
                if (!hideSideRail) {
                    PlumSideRail(
                        items = railItems,
                        contentFocusRequester = mainContentFocusRequester,
                        firstItemFocusRequester = homeRailFocusRequester,
                        footer = {
                            PlumActionButton(
                                modifier =
                                    Modifier.focusProperties { right = mainContentFocusRequester },
                                label = "Log Out",
                                onClick = {
                                    authVm.logout {
                                        webSocketManager.stop()
                                        onLogout()
                                    }
                                },
                                variant = PlumButtonVariant.Ghost,
                                leadingIcon = Icons.AutoMirrored.Filled.Logout,
                            )
                        },
                    )
                }

                // Keep weight + padding stable for every route. Toggling them for `play/...` made the
                // NavHost bounds jump (felt like zoom in/out around enter/exit playback).
                // No outer padding here — each screen owns its own padding via PlumScreenPadding().
                Box(
                    modifier =
                        Modifier
                            .weight(1f)
                            .fillMaxSize()
                            .focusRequester(mainContentFocusRequester)
                            .focusGroup(),
                ) {
                    NavHost(
                    navController = navController,
                    startDestination = Routes.HOME,
                    modifier = Modifier.fillMaxSize(),
                ) {
                    composable(Routes.HOME) {
                        HomeRoute(
                            onPlayMedia = { mediaId, resumeSec, libraryId, showKey, displayTitle, displaySubtitle ->
                                navigatePlay(
                                    mediaId,
                                    resumeSec,
                                    libraryId,
                                    showKey,
                                    displayTitle = displayTitle,
                                    displaySubtitle = displaySubtitle,
                                )
                            },
                            onOpenShow = { libraryId, showKey ->
                                val enc = Uri.encode(showKey)
                                navController.navigate("show/$libraryId/$enc")
                            },
                        )
                    }
                    composable(Routes.SEARCH) {
                        SearchRoute(
                            onOpenMovie = { libraryId, mediaId ->
                                navController.navigate("movie/$libraryId/$mediaId")
                            },
                            onOpenShow = { libraryId, showKey ->
                                navController.navigate("show/$libraryId/${Uri.encode(showKey)}")
                            },
                        )
                    }
                    composable(
                        route = Routes.DISCOVER_BROWSE,
                        arguments = listOf(
                            navArgument("category") {
                                type = NavType.StringType
                                defaultValue = ""
                            },
                            navArgument("mediaType") {
                                type = NavType.StringType
                                defaultValue = ""
                            },
                            navArgument("genre") {
                                type = NavType.IntType
                                defaultValue = 0
                            },
                        ),
                    ) { entry ->
                        DiscoverBrowseRoute(
                            category = entry.arguments?.getString("category")?.takeIf { it.isNotBlank() },
                            mediaType = entry.arguments?.getString("mediaType")?.takeIf { it.isNotBlank() },
                            genreId = entry.arguments?.getInt("genre")?.takeIf { it > 0 },
                            onOpenTitle = { mediaType, tmdbId ->
                                navController.navigate("discover/$mediaType/$tmdbId")
                            },
                            onBack = { navController.popBackStack() },
                        )
                    }
                    composable(
                        route = Routes.DISCOVER_DETAIL,
                        arguments = listOf(
                            navArgument("mediaType") { type = NavType.StringType },
                            navArgument("tmdbId") { type = NavType.IntType },
                        ),
                    ) { entry ->
                        val mediaType = entry.arguments?.getString("mediaType") ?: "movie"
                        val tmdbId = entry.arguments?.getInt("tmdbId") ?: 0
                        DiscoverDetailRoute(
                            mediaType = mediaType,
                            tmdbId = tmdbId,
                            onOpenLibrary = { libraryId, showKey ->
                                when {
                                    showKey != null -> navController.navigate("show/$libraryId/${Uri.encode(showKey)}")
                                    else -> navController.navigate("library/$libraryId/browse")
                                }
                            },
                            onBack = { navController.popBackStack() },
                            onOpenSettings = { navController.navigate("settings") },
                        )
                    }
                    composable(Routes.DISCOVER) {
                        DiscoverRoute(
                            onOpenBrowse = { category, mediaType, genreId ->
                                navController.navigate(buildDiscoverBrowseRoute(category, mediaType, genreId))
                            },
                            onOpenTitle = { mediaType, tmdbId ->
                                navController.navigate("discover/$mediaType/$tmdbId")
                            },
                        )
                    }
                    composable(Routes.DOWNLOADS) {
                        DownloadsRoute(onOpenSettings = { navController.navigate("settings") })
                    }
                    composable(
                        route = Routes.HUB,
                        arguments = listOf(navArgument("libraryType") { type = NavType.StringType }),
                    ) { entry ->
                        LibraryHubRoute(
                            libraryType = entry.arguments?.getString("libraryType"),
                            onPlayMedia = { mediaId, resumeSec, libraryId, showKey, displayTitle, displaySubtitle ->
                                navigatePlay(
                                    mediaId,
                                    resumeSec,
                                    libraryId,
                                    showKey,
                                    displayTitle = displayTitle,
                                    displaySubtitle = displaySubtitle,
                                )
                            },
                            onOpenShow = { libraryId, showKey ->
                                val enc = Uri.encode(showKey)
                                navController.navigate("show/$libraryId/$enc")
                            },
                            onOpenLibrary = { id ->
                                navController.navigate("library/$id/browse") {
                                    popUpTo(navController.graph.findStartDestination().id) {
                                        saveState = true
                                    }
                                    launchSingleTop = false
                                    restoreState = false
                                }
                            },
                        )
                    }
                    composable(Routes.LIBRARIES) {
                        LibraryListRoute(
                            onOpenLibrary = { id ->
                                navController.navigate("library/$id/browse")
                            },
                        )
                    }
                    composable(
                        route = Routes.LIBRARY_TYPE,
                        arguments = listOf(navArgument("libraryType") { type = NavType.StringType }),
                    ) { entry ->
                        LibraryListRoute(
                            onOpenLibrary = { id ->
                                navController.navigate("library/$id/browse")
                            },
                            libraryType = entry.arguments?.getString("libraryType"),
                        )
                    }
                    composable(
                        route = Routes.LIBRARY_BROWSE,
                        arguments = listOf(navArgument("libraryId") { type = NavType.IntType }),
                    ) {
                        LibraryBrowseRoute(
                            onOpenMovie = { libraryId, mediaId ->
                                navController.navigate("movie/$libraryId/$mediaId")
                            },
                            onOpenShow = { libraryId, showKey ->
                                val enc = Uri.encode(showKey)
                                navController.navigate("show/$libraryId/$enc")
                            },
                        )
                    }
                    composable(
                        route = Routes.MOVIE,
                        arguments = listOf(
                            navArgument("libraryId") { type = NavType.IntType },
                            navArgument("mediaId") { type = NavType.IntType },
                        ),
                    ) {
                        MovieDetailRoute(
                            onBack = { navController.popBackStack() },
                            onPlay = { mediaId, libraryId, title, subtitle ->
                                navigatePlay(
                                    mediaId,
                                    0f,
                                    libraryId = libraryId,
                                    displayTitle = title,
                                    displaySubtitle = subtitle,
                                )
                            },
                        )
                    }
                    composable(
                        route = Routes.SHOW,
                        arguments = listOf(
                            navArgument("libraryId") { type = NavType.IntType },
                            navArgument("showKey") { type = NavType.StringType },
                        ),
                    ) {
                        ShowDetailRoute(
                            onBack = { navController.popBackStack() },
                            onPlayEpisode = { mediaId, resumeSec, showLibraryId, showKey ->
                                navigatePlay(
                                    mediaId,
                                    resumeSec,
                                    libraryId = showLibraryId,
                                    showKey = showKey,
                                )
                            },
                        )
                    }
                    composable(
                        route = Routes.PLAY,
                        arguments = listOf(
                            navArgument("mediaId") { type = NavType.IntType },
                            navArgument("resume") {
                                type = NavType.FloatType
                                defaultValue = 0f
                            },
                            navArgument("libraryId") {
                                type = NavType.IntType
                                defaultValue = -1
                            },
                            navArgument("showKey") {
                                type = NavType.StringType
                                defaultValue = ""
                            },
                            navArgument("displayTitle") {
                                type = NavType.StringType
                                defaultValue = ""
                            },
                            navArgument("displaySubtitle") {
                                type = NavType.StringType
                                defaultValue = ""
                            },
                        ),
                    ) {
                        // Real UI is the fullscreen overlay below so this destination does not resize
                        // when the side rail is hidden for playback.
                        Spacer(Modifier.fillMaxSize())
                    }
                    composable("settings") {
                        SettingsRoute(
                            onLogoutComplete = onLogout,
                            defaultServerUrl = defaultServerUrl,
                        )
                    }
                }
                }
            }

            navBackStackEntry?.takeIf { hideSideRail }?.let { playEntry ->
                val playerVm = hiltViewModel<PlayerViewModel>(playEntry)
                Box(
                    Modifier
                        .fillMaxSize()
                        .zIndex(1f),
                ) {
                    PlayerRoute(
                        onClose = {
                            val prev = navController.previousBackStackEntry
                            if (prev?.destination?.route == Routes.SHOW) {
                                val playShowKey =
                                    playEntry.arguments?.getString("showKey")?.trim().orEmpty().let { k ->
                                        if (k.isEmpty()) "" else Uri.decode(k)
                                    }
                                val prevShowKey =
                                    prev.arguments?.getString("showKey")?.trim().orEmpty().let { k ->
                                        if (k.isEmpty()) "" else Uri.decode(k)
                                    }
                                if (playShowKey.isNotEmpty() && playShowKey == prevShowKey) {
                                    prev.savedStateHandle[ShowDetailViewModel.RETURN_FOCUS_EPISODE_MEDIA_ID] =
                                        playerVm.currentPlaybackMediaId()
                                }
                            }
                            navController.popBackStack()
                        },
                        viewModel = playerVm,
                    )
                }
            }
        }
    }
}

private fun buildDiscoverBrowseRoute(
    category: String?,
    mediaType: String?,
    genreId: Int?,
): String {
    val params = buildList {
        if (!category.isNullOrBlank()) add("category=${Uri.encode(category)}")
        if (!mediaType.isNullOrBlank()) add("mediaType=${Uri.encode(mediaType)}")
        if (genreId != null && genreId > 0) add("genre=$genreId")
    }
    return if (params.isEmpty()) "discover/browse" else "discover/browse?${params.joinToString("&")}"
}

private fun goToRoot(navController: NavHostController, route: String) {
    navController.navigate(route) {
        popUpTo(navController.graph.findStartDestination().id) {
            saveState = true
        }
        launchSingleTop = true
        restoreState = true
    }
}

private fun goToLibraryBrowse(navController: NavHostController, libraryId: Int) {
    navController.navigate("library/$libraryId/browse") {
        popUpTo(navController.graph.findStartDestination().id) {
            saveState = true
        }
        // Same graph destination for every libraryId; singleTop + restoreState can block a switch
        // or re-apply the wrong SavedState for another library's browse screen.
        launchSingleTop = false
        restoreState = false
    }
}

private fun openLibraryTypeFromRail(
    scope: CoroutineScope,
    navController: NavHostController,
    mainNavVm: MainNavViewModel,
    libraryType: String,
) {
    scope.launch {
        val soleId = mainNavVm.soleLibraryIdForType(libraryType)
        if (soleId != null) {
            goToLibraryBrowse(navController, soleId)
        } else {
            goToRoot(navController, "hub/$libraryType")
        }
    }
}
