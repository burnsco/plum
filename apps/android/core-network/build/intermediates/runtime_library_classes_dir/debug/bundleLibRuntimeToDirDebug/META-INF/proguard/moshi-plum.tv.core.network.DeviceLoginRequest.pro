-keepnames class plum.tv.core.network.DeviceLoginRequest
-if class plum.tv.core.network.DeviceLoginRequest
-keep class plum.tv.core.network.DeviceLoginRequestJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
