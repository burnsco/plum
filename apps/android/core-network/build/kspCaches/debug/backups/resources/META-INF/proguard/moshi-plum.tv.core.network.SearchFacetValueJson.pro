-keepnames class plum.tv.core.network.SearchFacetValueJson
-if class plum.tv.core.network.SearchFacetValueJson
-keep class plum.tv.core.network.SearchFacetValueJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
