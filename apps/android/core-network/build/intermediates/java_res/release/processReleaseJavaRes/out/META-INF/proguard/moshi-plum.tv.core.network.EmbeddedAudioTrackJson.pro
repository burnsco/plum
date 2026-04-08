-keepnames class plum.tv.core.network.EmbeddedAudioTrackJson
-if class plum.tv.core.network.EmbeddedAudioTrackJson
-keep class plum.tv.core.network.EmbeddedAudioTrackJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.EmbeddedAudioTrackJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.EmbeddedAudioTrackJson {
    public synthetic <init>(int,java.lang.String,java.lang.String,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
