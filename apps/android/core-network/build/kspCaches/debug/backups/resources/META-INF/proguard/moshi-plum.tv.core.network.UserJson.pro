-keepnames class plum.tv.core.network.UserJson
-if class plum.tv.core.network.UserJson
-keep class plum.tv.core.network.UserJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
