-keepnames class plum.tv.core.network.SubtitleJson
-if class plum.tv.core.network.SubtitleJson
-keep class plum.tv.core.network.SubtitleJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.SubtitleJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.SubtitleJson {
    public synthetic <init>(int,java.lang.String,java.lang.String,java.lang.String,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
