-keepnames class plum.tv.core.network.DetachPlaybackSessionCommandJson
-if class plum.tv.core.network.DetachPlaybackSessionCommandJson
-keep class plum.tv.core.network.DetachPlaybackSessionCommandJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.DetachPlaybackSessionCommandJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.DetachPlaybackSessionCommandJson {
    public synthetic <init>(java.lang.String,java.lang.String,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
