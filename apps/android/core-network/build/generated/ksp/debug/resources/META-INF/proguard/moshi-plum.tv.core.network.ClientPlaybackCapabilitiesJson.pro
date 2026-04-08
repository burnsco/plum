-keepnames class plum.tv.core.network.ClientPlaybackCapabilitiesJson
-if class plum.tv.core.network.ClientPlaybackCapabilitiesJson
-keep class plum.tv.core.network.ClientPlaybackCapabilitiesJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
