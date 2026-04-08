-keepnames class plum.tv.core.network.LibraryJson
-if class plum.tv.core.network.LibraryJson
-keep class plum.tv.core.network.LibraryJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
