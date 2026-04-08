-keepnames class plum.tv.core.network.EmbeddedSubtitleJson
-if class plum.tv.core.network.EmbeddedSubtitleJson
-keep class plum.tv.core.network.EmbeddedSubtitleJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.EmbeddedSubtitleJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.EmbeddedSubtitleJson {
    public synthetic <init>(int,java.lang.String,java.lang.String,java.lang.String,java.lang.Boolean,boolean,boolean,java.lang.Boolean,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
