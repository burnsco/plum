-keepnames class plum.tv.core.network.DeviceLoginResponseJson
-if class plum.tv.core.network.DeviceLoginResponseJson
-keep class plum.tv.core.network.DeviceLoginResponseJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
