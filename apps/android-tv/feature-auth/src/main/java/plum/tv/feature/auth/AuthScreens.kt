package plum.tv.feature.auth

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController

@Composable
fun AuthNavHost(
    onAuthenticated: () -> Unit,
    defaultServerUrl: String,
    defaultAdminEmail: String,
    defaultAdminPassword: String,
    viewModel: AuthViewModel = hiltViewModel(),
) {
    val navController = rememberNavController()
    NavHost(navController = navController, startDestination = "splash") {
        composable("splash") {
            SplashRoute(
                viewModel = viewModel,
                defaultServerUrl = defaultServerUrl,
                defaultAdminEmail = defaultAdminEmail,
                defaultAdminPassword = defaultAdminPassword,
                onNeedServer = { navController.navigate("server") { popUpTo("splash") { inclusive = true } } },
                onNeedLogin = { navController.navigate("login") { popUpTo("splash") { inclusive = true } } },
                onReady = onAuthenticated,
            )
        }
        composable("server") {
            ServerRoute(
                viewModel = viewModel,
                defaultServerUrl = defaultServerUrl,
                onSaved = {
                    navController.navigate("login") {
                        popUpTo("server") { inclusive = true }
                    }
                },
            )
        }
        composable("login") {
            LoginRoute(
                viewModel = viewModel,
                defaultAdminEmail = defaultAdminEmail,
                defaultAdminPassword = defaultAdminPassword,
                onSuccess = onAuthenticated,
            )
        }
    }
}

@Composable
private fun SplashRoute(
    viewModel: AuthViewModel,
    defaultServerUrl: String,
    defaultAdminEmail: String,
    defaultAdminPassword: String,
    onNeedServer: () -> Unit,
    onNeedLogin: () -> Unit,
    onReady: () -> Unit,
) {
    LaunchedEffect(defaultServerUrl, defaultAdminEmail, defaultAdminPassword) {
        when (viewModel.bootstrap(defaultServerUrl, defaultAdminEmail, defaultAdminPassword)) {
            StartupState.NeedServer -> onNeedServer()
            StartupState.NeedLogin -> onNeedLogin()
            StartupState.Authenticated -> onReady()
        }
    }
    Text("Starting…", modifier = Modifier.padding(48.dp))
}

@Composable
private fun ServerRoute(
    viewModel: AuthViewModel,
    defaultServerUrl: String,
    onSaved: () -> Unit,
) {
    var url by remember {
        mutableStateOf(
            viewModel.serverUrl.value ?: defaultServerUrl.ifBlank { "http://10.0.2.2:8080" },
        )
    }
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(48.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Text("Plum server URL")
        OutlinedTextField(
            value = url,
            onValueChange = { url = it },
            modifier = Modifier,
            singleLine = true,
        )
        Button(onClick = {
            viewModel.saveServerUrl(url.trim())
            onSaved()
        }) {
            Text("Continue")
        }
    }
}

@Composable
private fun LoginRoute(
    viewModel: AuthViewModel,
    defaultAdminEmail: String,
    defaultAdminPassword: String,
    onSuccess: () -> Unit,
) {
    var email by remember { mutableStateOf(defaultAdminEmail) }
    var password by remember { mutableStateOf(defaultAdminPassword) }
    var error by remember { mutableStateOf<String?>(null) }
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(48.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Text("Sign in")
        OutlinedTextField(value = email, onValueChange = { email = it }, singleLine = true, label = { Text("Email") })
        OutlinedTextField(
            value = password,
            onValueChange = { password = it },
            singleLine = true,
            label = { Text("Password") },
        )
        error?.let { Text(it) }
        Button(
            onClick = {
                viewModel.login(email, password) { result ->
                    result.onSuccess { onSuccess() }
                    result.onFailure { error = it.message ?: "Login failed" }
                }
            },
        ) {
            Text("Login")
        }
    }
}
