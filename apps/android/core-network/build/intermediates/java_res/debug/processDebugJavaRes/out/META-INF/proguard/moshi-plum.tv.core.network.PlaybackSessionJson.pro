-keepnames class plum.tv.core.network.PlaybackSessionJson
-if class plum.tv.core.network.PlaybackSessionJson
-keep class plum.tv.core.network.PlaybackSessionJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.PlaybackSessionJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.PlaybackSessionJson {
    public synthetic <init>(java.lang.String,int,java.lang.String,java.lang.Integer,java.lang.Integer,java.lang.String,java.lang.String,double,java.lang.String,java.util.List,java.util.List,java.util.List,java.lang.Double,java.lang.Double,java.lang.Double,java.lang.Double,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
