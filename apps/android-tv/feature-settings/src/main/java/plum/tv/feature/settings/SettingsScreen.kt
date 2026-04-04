package plum.tv.feature.settings

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.OutlinedTextField
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.tv.material3.Text
import plum.tv.core.ui.LaunchedTvFocusTo
import plum.tv.core.ui.PlumActionButton
import plum.tv.core.ui.PlumButtonVariant
import plum.tv.core.ui.PlumScreenPadding
import plum.tv.core.ui.PlumScreenTitle
import plum.tv.core.ui.PlumTheme
import plum.tv.core.ui.plumOutlinedFieldColors
import plum.tv.feature.auth.AuthViewModel

@Composable
fun SettingsRoute(
    onLogoutComplete: () -> Unit,
    defaultServerUrl: String,
    viewModel: AuthViewModel = hiltViewModel(),
) {
    val serverUrl by viewModel.serverUrl.collectAsState(initial = null)
    var url by remember(serverUrl, defaultServerUrl) {
        mutableStateOf(serverUrl ?: defaultServerUrl.ifBlank { "http://10.0.2.2:8080" })
    }
    val urlFieldFocus = remember { FocusRequester() }
    LaunchedTvFocusTo(focusRequester = urlFieldFocus)

    Column(
        modifier = Modifier.fillMaxSize().padding(PlumScreenPadding()),
        verticalArrangement = Arrangement.spacedBy(20.dp),
    ) {
        PlumScreenTitle("Settings", "Manage the connected server and your current session.")
        Text("Server URL", color = PlumTheme.palette.textSecondary)
        OutlinedTextField(
            value = url,
            onValueChange = { url = it },
            singleLine = true,
            modifier = Modifier.fillMaxWidth().focusRequester(urlFieldFocus),
            colors = plumOutlinedFieldColors(),
        )
        PlumActionButton(
            label = "Save server URL",
            onClick = { viewModel.saveServerUrl(url.trim(), onUrlChangedInvalidate = onLogoutComplete) },
            variant = PlumButtonVariant.Primary,
            leadingBadge = "SV",
        )
        Text(
            text = "Changing the server clears your session; sign in again after switching.",
            color = PlumTheme.palette.muted,
        )
        PlumActionButton(
            label = "Log out",
            onClick = { viewModel.logout(onLogoutComplete) },
            variant = PlumButtonVariant.Secondary,
            leadingBadge = "LO",
        )
    }
}
