package plum.tv.feature.auth

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch
import plum.tv.core.data.BrowseRepository
import plum.tv.core.data.SessionRepository
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
) : ViewModel() {

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

    fun saveServerUrl(url: String, onUrlChangedInvalidate: (() -> Unit)? = null) {
        viewModelScope.launch {
            val hadToken = sessionRepository.sessionToken.first().isNullOrBlank().not()
            sessionRepository.setServerUrl(url)
            browseRepository.invalidateLibrariesCache()
            val still = sessionRepository.sessionToken.first().isNullOrBlank().not()
            if (hadToken && !still) {
                onUrlChangedInvalidate?.invoke()
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

        val serverUrl = sessionRepository.serverUrl.first()?.trim()?.trimEnd('/')
        val sessionToken = sessionRepository.sessionToken.first()
        if (!serverUrl.isNullOrBlank() && sessionToken.isNullOrBlank() && defaultAdminEmail.isNotBlank() && defaultAdminPassword.isNotBlank()) {
            sessionRepository.login(defaultAdminEmail, defaultAdminPassword).getOrNull()
        }

        return readStartupState()
    }

    fun login(email: String, password: String, onResult: (Result<Unit>) -> Unit) {
        viewModelScope.launch {
            onResult(
                sessionRepository.login(email, password)
                    .onSuccess { browseRepository.invalidateLibrariesCache() }
                    .map { },
            )
        }
    }

    fun logout(onDone: () -> Unit) {
        viewModelScope.launch {
            sessionRepository.logout()
            browseRepository.invalidateLibrariesCache()
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
