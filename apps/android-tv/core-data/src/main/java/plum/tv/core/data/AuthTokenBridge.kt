package plum.tv.core.data

import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class AuthTokenBridge @Inject constructor() {
    @Volatile
    private var token: String? = null

    fun setToken(value: String?) {
        token = value
    }

    fun bearerToken(): String? = token
}
