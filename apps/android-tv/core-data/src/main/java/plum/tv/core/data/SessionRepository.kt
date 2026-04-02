package plum.tv.core.data

import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.first
import okhttp3.OkHttpClient
import plum.tv.core.model.DeviceLoginResult
import plum.tv.core.network.DeviceLoginRequest
import plum.tv.core.network.PlumApi
import plum.tv.core.network.PlumRetrofit
import com.squareup.moshi.Moshi
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class SessionRepository @Inject constructor(
    private val prefs: SessionPreferences,
    private val moshi: Moshi,
    private val tokenBridge: AuthTokenBridge,
    private val okHttpClient: OkHttpClient,
) {
    val serverUrl: Flow<String?> get() = prefs.serverUrl
    val sessionToken: Flow<String?> get() = prefs.sessionToken

    @Volatile
    private var cachedBaseUrl: String? = null

    @Volatile
    private var cachedApi: PlumApi? = null

    suspend fun hydrateTokenFromStore() {
        tokenBridge.setToken(prefs.sessionToken.first())
    }

    suspend fun setServerUrl(url: String) {
        val prev = prefs.serverUrl.first()?.trim()?.trimEnd('/')
        val next = url.trim().trimEnd('/')
        if (prev != null && prev.isNotEmpty() && prev != next) {
            prefs.clearSession()
            tokenBridge.setToken(null)
        }
        prefs.setServerUrl(url)
        synchronized(this) {
            cachedBaseUrl = null
            cachedApi = null
        }
    }

    suspend fun getPlumApi(): PlumApi {
        val base = prefs.serverUrl.first()?.trim()?.trimEnd('/')
            ?: throw IllegalStateException("Server URL is not set")
        synchronized(this) {
            if (cachedBaseUrl == base && cachedApi != null) {
                return cachedApi!!
            }
            val api = PlumRetrofit.createApi(base, okHttpClient, moshi)
            cachedBaseUrl = base
            cachedApi = api
            return api
        }
    }

    suspend fun login(email: String, password: String): Result<DeviceLoginResult> = runCatching {
        val api = getPlumApi()
        val res = api.deviceLogin(DeviceLoginRequest(email = email, password = password))
        if (!res.isSuccessful) {
            error(res.errorBody()?.string() ?: "Login failed (${res.code()})")
        }
        val body = res.body() ?: error("Empty login response")
        prefs.setSessionToken(body.sessionToken)
        tokenBridge.setToken(body.sessionToken)
        DeviceLoginResult(
            userId = body.user.id,
            email = body.user.email,
            sessionToken = body.sessionToken,
            expiresAtIso = body.expiresAt,
        )
    }

    suspend fun logout() {
        runCatching {
            val api = getPlumApi()
            api.logout()
        }
        prefs.clearSession()
        tokenBridge.setToken(null)
        synchronized(this) {
            cachedApi = null
            cachedBaseUrl = null
        }
    }
}
