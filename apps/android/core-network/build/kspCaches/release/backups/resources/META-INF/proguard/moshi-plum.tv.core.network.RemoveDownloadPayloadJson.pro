-keepnames class plum.tv.core.network.RemoveDownloadPayloadJson
-if class plum.tv.core.network.RemoveDownloadPayloadJson
-keep class plum.tv.core.network.RemoveDownloadPayloadJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
