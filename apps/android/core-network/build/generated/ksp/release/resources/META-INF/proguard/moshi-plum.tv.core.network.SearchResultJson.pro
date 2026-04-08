-keepnames class plum.tv.core.network.SearchResultJson
-if class plum.tv.core.network.SearchResultJson
-keep class plum.tv.core.network.SearchResultJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.SearchResultJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.SearchResultJson {
    public synthetic <init>(java.lang.String,int,java.lang.String,java.lang.String,java.lang.String,java.lang.String,java.lang.String,java.lang.String,java.lang.Double,java.lang.String,java.lang.String,java.lang.String,java.util.List,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
