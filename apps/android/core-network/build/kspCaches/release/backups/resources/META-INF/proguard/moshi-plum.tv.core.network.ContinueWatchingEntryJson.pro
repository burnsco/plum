-keepnames class plum.tv.core.network.ContinueWatchingEntryJson
-if class plum.tv.core.network.ContinueWatchingEntryJson
-keep class plum.tv.core.network.ContinueWatchingEntryJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.ContinueWatchingEntryJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.ContinueWatchingEntryJson {
    public synthetic <init>(java.lang.String,plum.tv.core.network.MediaItemJson,java.lang.String,java.lang.String,java.lang.String,double,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
