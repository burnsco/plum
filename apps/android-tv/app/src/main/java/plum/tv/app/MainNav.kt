package plum.tv.app

import android.net.Uri
import androidx.activity.ComponentActivity
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
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
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.media3.common.util.UnstableApi
import androidx.navigation.NavGraph.Companion.findStartDestination
import androidx.navigation.NavHostController
import androidx.navigation.NavType
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import androidx.navigation.navArgument
import kotlinx.coroutines.launch
import plum.tv.core.data.PlumWebSocketManager
import plum.tv.core.ui.PlumActionButton
import plum.tv.core.ui.PlumButtonVariant
import plum.tv.core.ui.PlumRailItem
import plum.tv.core.ui.PlumScreenTitle
import plum.tv.core.ui.PlumSideRail
import plum.tv.core.ui.PlumTvScaffold
import plum.tv.feature.discover.DiscoverBrowseRoute
import plum.tv.feature.discover.DiscoverDetailRoute
import plum.tv.feature.discover.DiscoverRoute
import plum.tv.feature.discover.DownloadsRoute
import plum.tv.feature.details.MovieDetailRoute
import plum.tv.feature.details.ShowDetailRoute
import plum.tv.feature.home.HomeRoute
import plum.tv.feature.library.LibraryBrowseRoute
import plum.tv.feature.library.LibraryListRoute
import plum.tv.feature.settings.SettingsRoute

private object Routes {
    const val HOME = "home"
    const val DISCOVER = "discover"
    const val DISCOVER_BROWSE = "discover/browse?category={category}&mediaType={mediaType}&genre={genre}"
    const val DISCOVER_DETAIL = "discover/{mediaType}/{tmdbId}"
    const val DOWNLOADS = "downloads"
    const val LIBRARIES = "libraries"
    const val LIBRARY_TYPE = "libraries/type/{libraryType}"
    const val LIBRARY_BROWSE = "library/{libraryId}/browse"
    const val MOVIE = "movie/{libraryId}/{mediaId}"
    const val SHOW = "show/{libraryId}/{showKey}"
    const val PLAY = "play/{mediaId}?resume={resume}"
}

@UnstableApi
@Composable
fun MainNavHost(
    webSocketManager: PlumWebSocketManager,
    defaultServerUrl: String,
    onLogout: () -> Unit,
) {
    val navController = rememberNavController()
    val navBackStackEntry by navController.currentBackStackEntryAsState()
    val currentRoute = navBackStackEntry?.destination?.route.orEmpty()
    val currentLibraryType = navBackStackEntry?.arguments?.getString("libraryType")
    val hideSideRail = currentRoute == Routes.PLAY
    val scope = rememberCoroutineScope()
    val activity = LocalContext.current as ComponentActivity
    val mainNavVm: MainNavViewModel = hiltViewModel(viewModelStoreOwner = activity)
    var browseRailType by remember { mutableStateOf<String?>(null) }
    LaunchedEffect(navBackStackEntry) {
        val entry = navBackStackEntry
        if (entry?.destination?.route == Routes.LIBRARY_BROWSE) {
            val id = entry.arguments?.getInt("libraryId")
            browseRailType = if (id != null) mainNavVm.railTypeForBrowseLibraryId(id) else null
        } else {
            browseRailType = null
        }
    }

    DisposableEffect(webSocketManager) {
        webSocketManager.start(scope)
        onDispose { webSocketManager.stop() }
    }

    fun navigatePlay(mediaId: Int, resumeSec: Float = 0f) {
        navController.navigate("play/$mediaId?resume=$resumeSec")
    }

    val railItems =
        listOf(
            PlumRailItem(
                key = Routes.HOME,
                label = "Home",
                badge = "H",
                selected = currentRoute == Routes.HOME,
                onClick = { goToRoot(navController, Routes.HOME) },
            ),
            PlumRailItem(
                key = Routes.DISCOVER,
                label = "Discover",
                badge = "D",
                selected = currentRoute == Routes.DISCOVER || currentRoute.startsWith("discover/"),
                onClick = { goToRoot(navController, Routes.DISCOVER) },
            ),
            PlumRailItem(
                key = Routes.DOWNLOADS,
                label = "Downloads",
                badge = "DL",
                selected = currentRoute == Routes.DOWNLOADS || currentRoute.startsWith("downloads"),
                onClick = { goToRoot(navController, Routes.DOWNLOADS) },
                dividerAfter = true,
            ),
            PlumRailItem(
                key = "library-tv",
                label = "TV",
                badge = "TV",
                selected =
                    (currentRoute.startsWith("libraries/type/") && currentLibraryType == "tv") ||
                        browseRailType == "tv",
                onClick = {
                    scope.launch {
                        val id = mainNavVm.firstLibraryIdForType("tv")
                        if (id != null) {
                            goToLibraryBrowse(navController, id)
                        } else {
                            goToRoot(navController, "libraries/type/tv")
                        }
                    }
                },
            ),
            PlumRailItem(
                key = "library-movies",
                label = "Movies",
                badge = "MV",
                selected =
                    (currentRoute.startsWith("libraries/type/") && currentLibraryType == "movie") ||
                        browseRailType == "movie",
                onClick = {
                    scope.launch {
                        val id = mainNavVm.firstLibraryIdForType("movie")
                        if (id != null) {
                            goToLibraryBrowse(navController, id)
                        } else {
                            goToRoot(navController, "libraries/type/movie")
                        }
                    }
                },
            ),
            PlumRailItem(
                key = "library-anime",
                label = "Anime",
                badge = "AN",
                selected =
                    (currentRoute.startsWith("libraries/type/") && currentLibraryType == "anime") ||
                        browseRailType == "anime",
                onClick = {
                    scope.launch {
                        val id = mainNavVm.firstLibraryIdForType("anime")
                        if (id != null) {
                            goToLibraryBrowse(navController, id)
                        } else {
                            goToRoot(navController, "libraries/type/anime")
                        }
                    }
                },
            ),
            PlumRailItem(
                key = "library-music",
                label = "Music",
                badge = "MU",
                selected =
                    (currentRoute.startsWith("libraries/type/") && currentLibraryType == "music") ||
                        browseRailType == "music",
                onClick = {
                    scope.launch {
                        val id = mainNavVm.firstLibraryIdForType("music")
                        if (id != null) {
                            goToLibraryBrowse(navController, id)
                        } else {
                            goToRoot(navController, "libraries/type/music")
                        }
                    }
                },
            ),
        )

    PlumTvScaffold {
        Row(Modifier.fillMaxSize()) {
            if (!hideSideRail) {
                PlumSideRail(
                    items = railItems,
                    footer = {
                        PlumActionButton(
                            label = "OUT",
                            onClick = onLogout,
                            variant = PlumButtonVariant.Ghost,
                        )
                    },
                )
            }

            Box(
                modifier =
                    if (hideSideRail) {
                        Modifier.fillMaxSize()
                    } else {
                        Modifier
                            .weight(1f)
                            .fillMaxSize()
                            .padding(end = 20.dp, top = 24.dp, bottom = 24.dp)
                    },
            ) {
                NavHost(
                    navController = navController,
                    startDestination = Routes.HOME,
                    modifier = Modifier.fillMaxSize(),
                ) {
                    composable(Routes.HOME) {
                        HomeRoute(
                            onPlayMovie = { mediaId, resumeSec ->
                                navigatePlay(mediaId, resumeSec)
                            },
                            onOpenShow = { libraryId, showKey ->
                                val enc = Uri.encode(showKey)
                                navController.navigate("show/$libraryId/$enc")
                            },
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
                    composable(Routes.DOWNLOADS) {
                        DownloadsRoute(onOpenSettings = { navController.navigate("settings") })
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
                            onPlay = { mediaId -> navigatePlay(mediaId, 0f) },
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
                            onPlayEpisode = { mediaId, resumeSec -> navigatePlay(mediaId, resumeSec) },
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
                        ),
                    ) {
                        PlayerRoute(onClose = { navController.popBackStack() })
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
    }
}

@Composable
private fun PlaceholderRoute(
    title: String,
    subtitle: String,
) {
    Box(
        modifier = Modifier.fillMaxSize(),
        contentAlignment = Alignment.Center,
    ) {
        Column(
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(12.dp),
            modifier = Modifier.padding(24.dp),
        ) {
            PlumScreenTitle(title = title, subtitle = subtitle)
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
        launchSingleTop = true
        restoreState = true
    }
}
