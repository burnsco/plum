-keepnames class plum.tv.core.network.DiscoverGenreJson
-if class plum.tv.core.network.DiscoverGenreJson
-keep class plum.tv.core.network.DiscoverGenreJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
