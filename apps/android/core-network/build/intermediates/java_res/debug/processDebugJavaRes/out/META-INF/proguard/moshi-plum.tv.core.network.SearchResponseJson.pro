-keepnames class plum.tv.core.network.SearchResponseJson
-if class plum.tv.core.network.SearchResponseJson
-keep class plum.tv.core.network.SearchResponseJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.SearchResponseJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.SearchResponseJson {
    public synthetic <init>(java.lang.String,java.util.List,int,plum.tv.core.network.SearchFacetsJson,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
