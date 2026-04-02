package plum.tv.feature.auth

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch
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
            val still = sessionRepository.sessionToken.first().isNullOrBlank().not()
            if (hadToken && !still) {
                onUrlChangedInvalidate?.invoke()
            }
        }
    }

    fun login(email: String, password: String, onResult: (Result<Unit>) -> Unit) {
        viewModelScope.launch {
            onResult(sessionRepository.login(email, password).map { })
        }
    }

    fun logout(onDone: () -> Unit) {
        viewModelScope.launch {
            sessionRepository.logout()
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
