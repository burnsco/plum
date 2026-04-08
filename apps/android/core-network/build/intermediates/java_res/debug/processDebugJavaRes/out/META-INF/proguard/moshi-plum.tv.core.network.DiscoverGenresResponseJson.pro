-keepnames class plum.tv.core.network.DiscoverGenresResponseJson
-if class plum.tv.core.network.DiscoverGenresResponseJson
-keep class plum.tv.core.network.DiscoverGenresResponseJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.DiscoverGenresResponseJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.DiscoverGenresResponseJson {
    public synthetic <init>(java.util.List,java.util.List,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
