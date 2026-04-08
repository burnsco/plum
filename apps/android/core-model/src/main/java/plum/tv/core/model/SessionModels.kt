package plum.tv.core.model

data class DeviceLoginResult(
    val userId: Int,
    val email: String,
    val sessionToken: String,
    val expiresAtIso: String,
)
