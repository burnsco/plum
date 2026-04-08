-keepnames class plum.tv.core.network.SearchFacetsJson
-if class plum.tv.core.network.SearchFacetsJson
-keep class plum.tv.core.network.SearchFacetsJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.SearchFacetsJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.SearchFacetsJson {
    public synthetic <init>(java.util.List,java.util.List,java.util.List,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
