-keepnames class plum.tv.core.network.DownloadItemJson
-if class plum.tv.core.network.DownloadItemJson
-keep class plum.tv.core.network.DownloadItemJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.DownloadItemJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.DownloadItemJson {
    public synthetic <init>(java.lang.String,java.lang.String,java.lang.String,java.lang.String,java.lang.String,java.lang.Double,java.lang.Long,java.lang.Double,java.lang.String,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
