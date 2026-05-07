package plum.tv.feature.auth

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
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
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.unit.dp
import androidx.hilt.lifecycle.viewmodel.compose.hiltViewModel
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import androidx.tv.material3.Text as TvText
import kotlinx.coroutines.delay
import plum.tv.core.ui.PlumActionButton
import plum.tv.core.ui.PlumButtonVariant
import plum.tv.core.ui.PlumTheme

@Composable
fun AuthNavHost(
    onAuthenticated: () -> Unit,
    defaultServerUrl: String,
    defaultAdminEmail: String,
    defaultAdminPassword: String,
    viewModel: AuthViewModel = hiltViewModel()
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
                onReady = onAuthenticated
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
                }
            )
        }
        composable("login") {
            LoginRoute(
                viewModel = viewModel,
                defaultAdminEmail = defaultAdminEmail,
                defaultAdminPassword = defaultAdminPassword,
                onSuccess = onAuthenticated,
                onOpenQuickConnect = { navController.navigate("quick_connect") }
            )
        }
        composable("quick_connect") {
            QuickConnectRoute(
                viewModel = viewModel,
                onSuccess = onAuthenticated,
                onBack = { navController.popBackStack() }
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
    onReady: () -> Unit
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
    onSaved: () -> Unit
) {
    var url by remember {
        mutableStateOf(
            viewModel.serverUrl.value ?: defaultServerUrl.ifBlank { "http://10.0.2.2:8080" }
        )
    }
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(48.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp)
    ) {
        Text("Plum server URL")
        OutlinedTextField(
            value = url,
            onValueChange = { url = it },
            modifier = Modifier,
            singleLine = true
        )
        Button(onClick = {
            viewModel.saveServerUrl(url.trim(), onDone = { onSaved() })
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
    onOpenQuickConnect: () -> Unit
) {
    var email by remember { mutableStateOf(defaultAdminEmail) }
    var password by remember { mutableStateOf(defaultAdminPassword) }
    var error by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    val quickStartAvailable =
        remember(defaultAdminEmail, defaultAdminPassword) {
            defaultAdminEmail.isNotBlank() && defaultAdminPassword.isNotBlank()
        }
    val quickStartFocus = remember { FocusRequester() }

    LaunchedEffect(quickStartAvailable) {
        if (quickStartAvailable) {
            delay(16)
            quickStartFocus.requestFocus()
        }
    }

    fun runLogin(e: String, p: String) {
        if (busy) return
        busy = true
        error = null
        viewModel.login(e, p) { result ->
            busy = false
            result.onSuccess { onSuccess() }
            result.onFailure { error = it.message ?: "Login failed" }
        }
    }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(48.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp)
    ) {
        Text("Sign in")
        if (quickStartAvailable) {
            // Keep enabled while busy so TV focus is not dropped from this control (disabled surfaces
            // often stop receiving focus, which feels like a hang).
            PlumActionButton(
                label = if (busy) "Signing in…" else "Quick start with default admin",
                onClick = { runLogin(defaultAdminEmail, defaultAdminPassword) },
                modifier = Modifier.focusRequester(quickStartFocus)
            )
            TvText(
                text = "Same credentials as Plum web onboarding (dev). One click — no typing.",
                style = PlumTheme.typography.bodySmall,
                color = PlumTheme.palette.muted
            )
            TvText(
                text = "Or enter email and password below.",
                style = PlumTheme.typography.labelLarge,
                color = PlumTheme.palette.textSecondary
            )
        }
        PlumActionButton(label = "Quick connect", onClick = onOpenQuickConnect)
        TvText(
            text = "Generate a code on this TV, then enter it from the Plum web profile menu.",
            style = PlumTheme.typography.bodySmall,
            color = PlumTheme.palette.muted
        )
        OutlinedTextField(value = email, onValueChange = { email = it }, singleLine = true, label = { Text("Email") })
        OutlinedTextField(
            value = password,
            onValueChange = { password = it },
            singleLine = true,
            label = { Text("Password") }
        )
        error?.let { Text(it) }
        Button(
            onClick = { runLogin(email, password) },
            enabled = !busy
        ) {
            Text(if (busy) "Signing in…" else "Login")
        }
    }
}

@Composable
private fun QuickConnectRoute(
    viewModel: AuthViewModel,
    onSuccess: () -> Unit,
    onBack: () -> Unit
) {
    var code by remember { mutableStateOf<String?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    val connectFocus = remember { FocusRequester() }

    LaunchedEffect(Unit) {
        delay(16)
        connectFocus.requestFocus()
    }

    fun start() {
        if (busy) return
        busy = true
        error = null
        code = null
        viewModel.startDeviceQuickConnect(
            onCode = { code = it }
        ) { result ->
            busy = false
            result.onSuccess { onSuccess() }
            result.onFailure { error = it.message ?: "Quick connect failed" }
        }
    }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(48.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp)
    ) {
        Text("Quick connect")
        TvText(
            text = "Generate a code on this TV, then open Plum on your computer or phone. Use the profile menu → Quick connect and enter the code.",
            style = PlumTheme.typography.bodySmall,
            color = PlumTheme.palette.muted
        )
        code?.let {
            TvText(
                text = it,
                style = PlumTheme.typography.displaySmall,
                color = PlumTheme.palette.text
            )
            TvText(
                text = "Waiting for web approval...",
                style = PlumTheme.typography.bodySmall,
                color = PlumTheme.palette.muted
            )
        }
        error?.let { Text(it) }
        Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            PlumActionButton(label = "Back", onClick = onBack, variant = PlumButtonVariant.Ghost)
            PlumActionButton(
                label = if (busy) {
                    "Waiting..."
                } else if (code == null) {
                    "Generate code"
                } else {
                    "New code"
                },
                onClick = { start() },
                modifier = Modifier.focusRequester(connectFocus),
                enabled = !busy
            )
        }
    }
}
