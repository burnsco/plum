package plum.tv.app

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import dagger.hilt.android.AndroidEntryPoint
import javax.inject.Inject
import plum.tv.core.data.PlumWebSocketManager
import plum.tv.feature.auth.AuthNavHost

@AndroidEntryPoint
class MainActivity : ComponentActivity() {
    @Inject
    lateinit var webSocketManager: PlumWebSocketManager

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            PlumTvTheme {
                PlumTvRoot(webSocketManager = webSocketManager)
            }
        }
    }
}

@Composable
private fun PlumTvRoot(webSocketManager: PlumWebSocketManager) {
    var authed by remember { mutableStateOf(false) }
    if (!authed) {
        AuthNavHost(onAuthenticated = { authed = true })
    } else {
        MainNavHost(
            webSocketManager = webSocketManager,
            onLogout = {
                webSocketManager.stop()
                authed = false
            },
        )
    }
}
