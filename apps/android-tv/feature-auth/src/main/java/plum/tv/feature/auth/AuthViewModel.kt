package plum.tv.feature.auth

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch
import kotlinx.coroutines.withTimeoutOrNull
import plum.tv.core.data.BrowseRepository
import plum.tv.core.data.LibraryCatalogRefreshCoordinator
import plum.tv.core.data.SessionRepository
import java.util.Locale
import javax.inject.Inject

enum class StartupState {
    NeedServer,
    NeedLogin,
    Authenticated,
}

@HiltViewModel
class AuthViewModel @Inject constructor(
    private val sessionRepository: SessionRepository,
    private val browseRepository: BrowseRepository,
    private val catalogRefreshCoordinator: LibraryCatalogRefreshCoordinator,
) : ViewModel() {

    private fun invalidateAllLocalCatalogCaches() {
        catalogRefreshCoordinator.resetScanState()
        browseRepository.invalidateLibrariesCache()
    }

    val serverUrl = sessionRepository.serverUrl.stateIn(
        viewModelScope,
        SharingStarted.WhileSubscribed(5_000),
        null,
    )

    val sessionToken = sessionRepository.sessionToken.stateIn(
        viewModelScope,
        SharingStarted.WhileSubscribed(5_000),
        null,
    )

    fun saveServerUrl(
        url: String,
        onUrlChangedInvalidate: (() -> Unit)? = null,
        /** Called on the main thread after the URL is persisted (or if the save coroutine fails). */
        onDone: (() -> Unit)? = null,
    ) {
        viewModelScope.launch {
            try {
                val hadToken = sessionRepository.sessionToken.first().isNullOrBlank().not()
                sessionRepository.setServerUrl(url)
                invalidateAllLocalCatalogCaches()
                val still = sessionRepository.sessionToken.first().isNullOrBlank().not()
                if (hadToken && !still) {
                    onUrlChangedInvalidate?.invoke()
                }
            } finally {
                onDone?.invoke()
            }
        }
    }

    suspend fun bootstrap(
        defaultServerUrl: String,
        defaultAdminEmail: String,
        defaultAdminPassword: String,
    ): StartupState {
        sessionRepository.hydrateTokenFromStore()
        val currentServerUrl = sessionRepository.serverUrl.first()?.trim()?.trimEnd('/')
        if (currentServerUrl.isNullOrBlank() && defaultServerUrl.isNotBlank()) {
            sessionRepository.setServerUrl(defaultServerUrl)
        }

        suspend fun tryDefaultAdminAutoLogin() {
            val url = sessionRepository.serverUrl.first()?.trim()?.trimEnd('/')
            if (url.isNullOrBlank()) return
            if (sessionRepository.sessionToken.first().isNullOrBlank().not()) return
            if (defaultAdminEmail.isBlank() || defaultAdminPassword.isBlank()) return
            sessionRepository.login(defaultAdminEmail, defaultAdminPassword)
                .onSuccess { invalidateAllLocalCatalogCaches() }
        }

        tryDefaultAdminAutoLogin()

        var state = readStartupState()
        if (state == StartupState.Authenticated && sessionRepository.serverRejectsStoredSession()) {
            sessionRepository.clearLocalSession()
            invalidateAllLocalCatalogCaches()
            tryDefaultAdminAutoLogin()
            state = readStartupState()
        }
        return state
    }

    fun login(email: String, password: String, onResult: (Result<Unit>) -> Unit) {
        viewModelScope.launch {
            val result =
                withTimeoutOrNull(35_000) {
                    sessionRepository.login(email, password)
                        .onSuccess { invalidateAllLocalCatalogCaches() }
                        .map { }
                }
                    ?: Result.failure(
                        Exception("Could not reach the server in time. Check the URL, network, and that Plum is running."),
                    )
            onResult(result)
        }
    }

    /** Redeem a 6-character code created in the web app (Settings → Quick connect). */
    fun redeemQuickConnect(code: String, onResult: (Result<Unit>) -> Unit) {
        viewModelScope.launch {
            val normalized =
                code.uppercase(Locale.US).filter { it.isDigit() || it in 'A'..'Z' }.take(6)
            val result =
                if (normalized.length != 6) {
                    Result.failure(Exception("Enter the 6-character code from the web app."))
                } else {
                    withTimeoutOrNull(35_000) {
                        sessionRepository.redeemQuickConnect(normalized)
                            .onSuccess { invalidateAllLocalCatalogCaches() }
                            .map { }
                    }
                        ?: Result.failure(
                            Exception("Could not reach the server in time. Check the URL, network, and that Plum is running."),
                        )
                }
            onResult(result)
        }
    }

    fun logout(onDone: () -> Unit) {
        viewModelScope.launch {
            sessionRepository.logout()
            invalidateAllLocalCatalogCaches()
            onDone()
        }
    }

    suspend fun readStartupState(): StartupState {
        sessionRepository.hydrateTokenFromStore()
        val url = sessionRepository.serverUrl.first()
        val token = sessionRepository.sessionToken.first()
        return when {
            url.isNullOrBlank() -> StartupState.NeedServer
            token.isNullOrBlank() -> StartupState.NeedLogin
            else -> StartupState.Authenticated
        }
    }
}
