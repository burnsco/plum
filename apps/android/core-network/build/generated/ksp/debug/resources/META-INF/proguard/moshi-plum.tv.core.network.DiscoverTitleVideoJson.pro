-keepnames class plum.tv.core.network.DiscoverTitleVideoJson
-if class plum.tv.core.network.DiscoverTitleVideoJson
-keep class plum.tv.core.network.DiscoverTitleVideoJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.DiscoverTitleVideoJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.DiscoverTitleVideoJson {
    public synthetic <init>(java.lang.String,java.lang.String,java.lang.String,java.lang.String,java.lang.Boolean,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
