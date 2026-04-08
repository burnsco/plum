-keepnames class plum.tv.core.network.CreatePlaybackSessionPayloadJson
-if class plum.tv.core.network.CreatePlaybackSessionPayloadJson
-keep class plum.tv.core.network.CreatePlaybackSessionPayloadJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.CreatePlaybackSessionPayloadJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.CreatePlaybackSessionPayloadJson {
    public synthetic <init>(java.lang.Integer,plum.tv.core.network.ClientPlaybackCapabilitiesJson,java.lang.Integer,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
