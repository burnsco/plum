-keepnames class plum.tv.core.network.DownloadsResponseJson
-if class plum.tv.core.network.DownloadsResponseJson
-keep class plum.tv.core.network.DownloadsResponseJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.DownloadsResponseJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.DownloadsResponseJson {
    public synthetic <init>(boolean,java.util.List,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
