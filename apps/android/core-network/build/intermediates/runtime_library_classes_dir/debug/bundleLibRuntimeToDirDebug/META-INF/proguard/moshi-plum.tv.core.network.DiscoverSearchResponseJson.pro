-keepnames class plum.tv.core.network.DiscoverSearchResponseJson
-if class plum.tv.core.network.DiscoverSearchResponseJson
-keep class plum.tv.core.network.DiscoverSearchResponseJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.DiscoverSearchResponseJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.DiscoverSearchResponseJson {
    public synthetic <init>(java.util.List,java.util.List,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
