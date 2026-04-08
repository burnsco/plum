-keepnames class plum.tv.core.network.UpdatePlaybackSessionAudioPayloadJson
-if class plum.tv.core.network.UpdatePlaybackSessionAudioPayloadJson
-keep class plum.tv.core.network.UpdatePlaybackSessionAudioPayloadJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
