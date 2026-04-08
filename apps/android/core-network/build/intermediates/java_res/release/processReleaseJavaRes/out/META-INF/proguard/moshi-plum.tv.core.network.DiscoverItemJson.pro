-keepnames class plum.tv.core.network.DiscoverItemJson
-if class plum.tv.core.network.DiscoverItemJson
-keep class plum.tv.core.network.DiscoverItemJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.DiscoverItemJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.DiscoverItemJson {
    public synthetic <init>(java.lang.String,int,java.lang.String,java.lang.String,java.lang.String,java.lang.String,java.lang.String,java.lang.String,java.lang.Double,java.util.List,plum.tv.core.network.DiscoverAcquisitionJson,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
