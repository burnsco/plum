-keepnames class plum.tv.core.network.DiscoverResponseJson
-if class plum.tv.core.network.DiscoverResponseJson
-keep class plum.tv.core.network.DiscoverResponseJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.DiscoverResponseJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.DiscoverResponseJson {
    public synthetic <init>(java.util.List,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
