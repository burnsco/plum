-keepnames class plum.tv.core.network.DiscoverAcquisitionJson
-if class plum.tv.core.network.DiscoverAcquisitionJson
-keep class plum.tv.core.network.DiscoverAcquisitionJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.DiscoverAcquisitionJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.DiscoverAcquisitionJson {
    public synthetic <init>(java.lang.String,java.lang.String,java.lang.Boolean,java.lang.Boolean,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
