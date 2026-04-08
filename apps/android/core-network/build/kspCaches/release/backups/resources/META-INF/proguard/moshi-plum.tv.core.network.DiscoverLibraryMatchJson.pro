-keepnames class plum.tv.core.network.DiscoverLibraryMatchJson
-if class plum.tv.core.network.DiscoverLibraryMatchJson
-keep class plum.tv.core.network.DiscoverLibraryMatchJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.DiscoverLibraryMatchJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.DiscoverLibraryMatchJson {
    public synthetic <init>(int,java.lang.String,java.lang.String,java.lang.String,java.lang.String,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
