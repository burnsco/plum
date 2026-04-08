-keepnames class plum.tv.core.network.RecentlyAddedEntryJson
-if class plum.tv.core.network.RecentlyAddedEntryJson
-keep class plum.tv.core.network.RecentlyAddedEntryJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.RecentlyAddedEntryJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.RecentlyAddedEntryJson {
    public synthetic <init>(java.lang.String,plum.tv.core.network.MediaItemJson,java.lang.String,java.lang.String,java.lang.String,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
