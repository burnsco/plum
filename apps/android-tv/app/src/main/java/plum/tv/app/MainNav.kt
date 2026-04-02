package plum.tv.app

import android.net.Uri
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.rememberCoroutineScope
import androidx.media3.common.util.UnstableApi
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import plum.tv.core.data.PlumWebSocketManager
import androidx.navigation.NavGraph.Companion.findStartDestination
import androidx.navigation.NavType
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import androidx.navigation.navArgument
import androidx.tv.material3.Button
import androidx.tv.material3.ButtonDefaults
import androidx.tv.material3.ExperimentalTvMaterial3Api
import androidx.tv.material3.Text
import plum.tv.feature.details.MovieDetailRoute
import plum.tv.feature.details.ShowDetailRoute
import plum.tv.feature.home.HomeRoute
import plum.tv.feature.library.LibraryBrowseRoute
import plum.tv.feature.library.LibraryListRoute
import plum.tv.feature.search.SearchRoute
import plum.tv.feature.settings.SettingsRoute

private object Routes {
    const val HOME = "home"
    const val LIBRARIES = "libraries"
    const val LIBRARY_BROWSE = "library/{libraryId}/browse"
    const val MOVIE = "movie/{libraryId}/{mediaId}"
    const val SHOW = "show/{libraryId}/{showKey}"
    const val SEARCH = "search"
    const val SETTINGS = "settings"
    const val PLAY = "play/{mediaId}?resume={resume}"
}

@OptIn(ExperimentalTvMaterial3Api::class)
@UnstableApi
@Composable
fun MainNavHost(
    webSocketManager: PlumWebSocketManager,
    onLogout: () -> Unit,
) {
    val navController = rememberNavController()
    val navBackStackEntry by navController.currentBackStackEntryAsState()
    val hideSideRail = navBackStackEntry?.destination?.route == Routes.PLAY
    val scope = rememberCoroutineScope()
    DisposableEffect(webSocketManager) {
        webSocketManager.start(scope)
        onDispose { webSocketManager.stop() }
    }

    fun navigatePlay(mediaId: Int, resumeSec: Float = 0f) {
        navController.navigate("play/$mediaId?resume=$resumeSec")
    }

    Row(Modifier.fillMaxSize()) {
        if (!hideSideRail) {
        Column(
            modifier = Modifier
                .width(260.dp)
                .padding(24.dp),
        ) {
            fun go(route: String) {
                navController.navigate(route) {
                    popUpTo(navController.graph.findStartDestination().id) {
                        saveState = true
                    }
                    launchSingleTop = true
                    restoreState = true
                }
            }
            Button(
                onClick = { go(Routes.HOME) },
                modifier = Modifier
                    .fillMaxWidth()
                    .height(52.dp)
                    .padding(bottom = 8.dp),
                scale = ButtonDefaults.scale(focusedScale = 1.08f),
            ) {
                Text("Home")
            }
            Button(
                onClick = { go(Routes.LIBRARIES) },
                modifier = Modifier
                    .fillMaxWidth()
                    .height(52.dp)
                    .padding(bottom = 8.dp),
                scale = ButtonDefaults.scale(focusedScale = 1.08f),
            ) {
                Text("Libraries")
            }
            Button(
                onClick = { go(Routes.SEARCH) },
                modifier = Modifier
                    .fillMaxWidth()
                    .height(52.dp)
                    .padding(bottom = 8.dp),
                scale = ButtonDefaults.scale(focusedScale = 1.08f),
            ) {
                Text("Search")
            }
            Button(
                onClick = { go(Routes.SETTINGS) },
                modifier = Modifier
                    .fillMaxWidth()
                    .height(52.dp)
                    .padding(bottom = 8.dp),
                scale = ButtonDefaults.scale(focusedScale = 1.08f),
            ) {
                Text("Settings")
            }
        }
        }

        NavHost(
            navController = navController,
            startDestination = Routes.HOME,
            modifier =
                if (hideSideRail) {
                    Modifier.fillMaxSize()
                } else {
                    Modifier
                        .weight(1f)
                        .fillMaxSize()
                },
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
            composable(Routes.LIBRARIES) {
                LibraryListRoute(
                    onOpenLibrary = { id ->
                        navController.navigate("library/$id/browse")
                    },
                )
            }
            composable(
                route = Routes.LIBRARY_BROWSE,
                arguments = listOf(
                    androidx.navigation.navArgument("libraryId") { type = NavType.IntType },
                ),
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
                    androidx.navigation.navArgument("libraryId") { type = NavType.IntType },
                    androidx.navigation.navArgument("mediaId") { type = NavType.IntType },
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
                    androidx.navigation.navArgument("libraryId") { type = NavType.IntType },
                    androidx.navigation.navArgument("showKey") { type = NavType.StringType },
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
            composable(Routes.SEARCH) {
                SearchRoute(
                    onOpenMovie = { libraryId, mediaId ->
                        navController.navigate("movie/$libraryId/$mediaId")
                    },
                    onOpenShow = { libraryId, showKey ->
                        val enc = Uri.encode(showKey)
                        navController.navigate("show/$libraryId/$enc")
                    },
                )
            }
            composable(Routes.SETTINGS) {
                SettingsRoute(onLogoutComplete = onLogout)
            }
        }
    }
}
