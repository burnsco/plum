-keepnames class plum.tv.core.network.DiscoverBrowseResponseJson
-if class plum.tv.core.network.DiscoverBrowseResponseJson
-keep class plum.tv.core.network.DiscoverBrowseResponseJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.DiscoverBrowseResponseJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.DiscoverBrowseResponseJson {
    public synthetic <init>(java.util.List,int,int,int,java.lang.String,plum.tv.core.network.DiscoverGenreJson,java.lang.String,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
