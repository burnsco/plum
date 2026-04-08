-keepnames class plum.tv.core.network.PlaybackSessionUpdateEventJson
-if class plum.tv.core.network.PlaybackSessionUpdateEventJson
-keep class plum.tv.core.network.PlaybackSessionUpdateEventJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.PlaybackSessionUpdateEventJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.PlaybackSessionUpdateEventJson {
    public synthetic <init>(java.lang.String,java.lang.String,java.lang.String,int,java.lang.Integer,int,java.lang.String,java.lang.String,double,java.lang.String,java.lang.Integer,java.lang.String,java.lang.Double,java.lang.Double,java.lang.Double,java.lang.Double,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
