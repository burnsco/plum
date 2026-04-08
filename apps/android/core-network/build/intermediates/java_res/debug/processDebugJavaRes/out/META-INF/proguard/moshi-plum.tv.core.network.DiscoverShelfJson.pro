-keepnames class plum.tv.core.network.DiscoverShelfJson
-if class plum.tv.core.network.DiscoverShelfJson
-keep class plum.tv.core.network.DiscoverShelfJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.DiscoverShelfJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.DiscoverShelfJson {
    public synthetic <init>(java.lang.String,java.lang.String,java.util.List,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
