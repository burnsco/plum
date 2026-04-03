package plum.tv.core.data

import android.util.Log
import com.squareup.moshi.Moshi
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import okhttp3.OkHttpClient
import plum.tv.core.model.DeviceLoginResult
import plum.tv.core.network.DeviceLoginRequest
import plum.tv.core.network.PlumApi
import plum.tv.core.network.PlumRetrofit

@Singleton
class SessionRepository @Inject constructor(
    private val prefs: SessionPreferences,
    private val moshi: Moshi,
    private val tokenBridge: AuthTokenBridge,
    private val okHttpClient: OkHttpClient,
) {
    private companion object {
        const val TAG = "PlumTV"
    }

    val serverUrl: Flow<String?> get() = prefs.serverUrl
    val sessionToken: Flow<String?> get() = prefs.sessionToken

    @Volatile
    private var cachedBaseUrl: String? = null

    @Volatile
    private var cachedApi: PlumApi? = null

    private val apiMutex = Mutex()

    suspend fun hydrateTokenFromStore() {
        tokenBridge.setToken(prefs.sessionToken.first())
    }

    suspend fun setServerUrl(url: String) {
        val prev = prefs.serverUrl.first()?.trim()?.trimEnd('/')
        val next = url.trim().trimEnd('/')
        if (prev != null && prev.isNotEmpty() && prev != next) {
            Log.i(TAG, "server url changed old=$prev new=$next")
            prefs.clearSession()
            tokenBridge.setToken(null)
        } else {
            Log.i(TAG, "server url set url=$next")
        }
        prefs.setServerUrl(url)
        apiMutex.withLock {
            cachedBaseUrl = null
            cachedApi = null
        }
    }

    suspend fun getPlumApi(): PlumApi {
        val base = prefs.serverUrl.first()?.trim()?.trimEnd('/')
            ?: throw IllegalStateException("Server URL is not set")
        return apiMutex.withLock {
            if (cachedBaseUrl == base && cachedApi != null) {
                return@withLock cachedApi!!
            }
            val api = PlumRetrofit.createApi(base, okHttpClient, moshi)
            cachedBaseUrl = base
            cachedApi = api
            api
        }
    }

    suspend fun login(email: String, password: String): Result<DeviceLoginResult> =
        runCatching {
            Log.i(TAG, "login start")
            val api = getPlumApi()
            val res = api.deviceLogin(DeviceLoginRequest(email = email, password = password))
            if (!res.isSuccessful) {
                error(res.errorBody()?.string() ?: "Login failed (${res.code()})")
            }
            val body = res.body() ?: error("Empty login response")
            prefs.setSessionToken(body.sessionToken)
            tokenBridge.setToken(body.sessionToken)
            Log.i(TAG, "login success userId=${body.user.id}")
            DeviceLoginResult(
                userId = body.user.id,
                email = body.user.email,
                sessionToken = body.sessionToken,
                expiresAtIso = body.expiresAt,
            )
        }.onFailure {
            Log.w(TAG, "login failure error=${it.message}", it)
        }

    suspend fun logout() {
        Log.i(TAG, "logout start")
        runCatching {
            val api = getPlumApi()
            api.logout()
        }.onFailure {
            Log.w(TAG, "logout failure error=${it.message}", it)
        }
        prefs.clearSession()
        tokenBridge.setToken(null)
        apiMutex.withLock {
            cachedApi = null
            cachedBaseUrl = null
        }
        Log.i(TAG, "logout complete")
    }
}
