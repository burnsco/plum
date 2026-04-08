-keepnames class plum.tv.core.network.DiscoverTitleDetailsJson
-if class plum.tv.core.network.DiscoverTitleDetailsJson
-keep class plum.tv.core.network.DiscoverTitleDetailsJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.DiscoverTitleDetailsJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.DiscoverTitleDetailsJson {
    public synthetic <init>(java.lang.String,int,java.lang.String,java.lang.String,java.lang.String,java.lang.String,java.lang.String,java.lang.String,java.lang.Double,java.lang.String,java.lang.Double,java.lang.String,java.util.List,java.lang.Integer,java.lang.Integer,java.lang.Integer,java.util.List,java.util.List,plum.tv.core.network.DiscoverAcquisitionJson,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
