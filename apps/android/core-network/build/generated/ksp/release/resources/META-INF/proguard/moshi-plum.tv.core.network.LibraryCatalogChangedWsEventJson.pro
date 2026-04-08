-keepnames class plum.tv.core.network.LibraryCatalogChangedWsEventJson
-if class plum.tv.core.network.LibraryCatalogChangedWsEventJson
-keep class plum.tv.core.network.LibraryCatalogChangedWsEventJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
