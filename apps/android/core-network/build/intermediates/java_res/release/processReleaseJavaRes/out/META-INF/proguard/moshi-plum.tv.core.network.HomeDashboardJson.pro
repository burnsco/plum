-keepnames class plum.tv.core.network.HomeDashboardJson
-if class plum.tv.core.network.HomeDashboardJson
-keep class plum.tv.core.network.HomeDashboardJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.HomeDashboardJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.HomeDashboardJson {
    public synthetic <init>(java.util.List,java.util.List,java.util.List,java.util.List,java.util.List,java.util.List,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
