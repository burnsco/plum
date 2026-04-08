-keepnames class plum.tv.core.network.LibraryScanUpdateWsEventJson
-if class plum.tv.core.network.LibraryScanUpdateWsEventJson
-keep class plum.tv.core.network.LibraryScanUpdateWsEventJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
