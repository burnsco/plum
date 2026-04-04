package plum.tv.app

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.annotation.OptIn
import androidx.compose.runtime.Composable
import androidx.media3.common.util.UnstableApi
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import dagger.hilt.android.AndroidEntryPoint
import javax.inject.Inject
import plum.tv.core.data.LibraryScanStatusPoller
import plum.tv.core.data.PlumWebSocketManager
import plum.tv.core.data.SessionPreferences
import plum.tv.core.ui.PlumTvTheme
import plum.tv.feature.auth.AuthNavHost

@AndroidEntryPoint
class MainActivity : ComponentActivity() {
    @Inject
    lateinit var webSocketManager: PlumWebSocketManager

    @Inject
    lateinit var libraryScanStatusPoller: LibraryScanStatusPoller

    @Inject
    lateinit var sessionPreferences: SessionPreferences

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            val serverUrl by sessionPreferences.serverUrl.collectAsState(initial = "")
            PlumTvTheme(serverBaseUrl = serverUrl ?: "") {
                PlumTvRoot(
                    webSocketManager = webSocketManager,
                    libraryScanStatusPoller = libraryScanStatusPoller,
                    defaultServerUrl = BuildConfig.DEFAULT_SERVER_URL,
                    defaultAdminEmail = BuildConfig.DEFAULT_ADMIN_EMAIL,
                    defaultAdminPassword = BuildConfig.DEFAULT_ADMIN_PASSWORD,
                )
            }
        }
    }
}

@OptIn(UnstableApi::class)
@Composable
private fun PlumTvRoot(
    webSocketManager: PlumWebSocketManager,
    libraryScanStatusPoller: LibraryScanStatusPoller,
    defaultServerUrl: String,
    defaultAdminEmail: String,
    defaultAdminPassword: String,
) {
    var authed by remember { mutableStateOf(false) }
    if (!authed) {
        AuthNavHost(
            onAuthenticated = { authed = true },
            defaultServerUrl = defaultServerUrl,
            defaultAdminEmail = defaultAdminEmail,
            defaultAdminPassword = defaultAdminPassword,
        )
    } else {
        MainNavHost(
            webSocketManager = webSocketManager,
            libraryScanStatusPoller = libraryScanStatusPoller,
            defaultServerUrl = defaultServerUrl,
            onLogout = {
                webSocketManager.stop()
                authed = false
            },
        )
    }
}
