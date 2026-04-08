-keepnames class plum.tv.core.network.AttachPlaybackSessionCommandJson
-if class plum.tv.core.network.AttachPlaybackSessionCommandJson
-keep class plum.tv.core.network.AttachPlaybackSessionCommandJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.AttachPlaybackSessionCommandJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.AttachPlaybackSessionCommandJson {
    public synthetic <init>(java.lang.String,java.lang.String,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
