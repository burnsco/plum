package plum.tv.feature.settings

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.compose.material3.OutlinedTextField
import androidx.tv.material3.Button
import androidx.tv.material3.ExperimentalTvMaterial3Api
import androidx.tv.material3.Text
import kotlinx.coroutines.launch
import androidx.compose.runtime.rememberCoroutineScope
import plum.tv.feature.auth.AuthViewModel

@OptIn(ExperimentalTvMaterial3Api::class)
@Composable
fun SettingsRoute(
    onLogoutComplete: () -> Unit,
    viewModel: AuthViewModel = hiltViewModel(),
) {
    val serverUrl by viewModel.serverUrl.collectAsState(initial = null)
    var url by remember(serverUrl) { mutableStateOf(serverUrl ?: "http://10.0.2.2:8080") }
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(48.dp),
        verticalArrangement = Arrangement.spacedBy(20.dp),
    ) {
        Text("Settings")
        Text("Server URL")
        OutlinedTextField(
            value = url,
            onValueChange = { url = it },
            singleLine = true,
        )
        Button(
            onClick = {
                viewModel.saveServerUrl(url.trim(), onUrlChangedInvalidate = onLogoutComplete)
            },
        ) {
            Text("Save server URL")
        }
        Text("Changing the server clears your session; sign in again after switching.")
        Button(
            onClick = {
                viewModel.logout(onLogoutComplete)
            },
        ) {
            Text("Log out")
        }
    }
}
