-keepnames class plum.tv.core.network.ShowEpisodesResponseJson
-if class plum.tv.core.network.ShowEpisodesResponseJson
-keep class plum.tv.core.network.ShowEpisodesResponseJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.ShowEpisodesResponseJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.ShowEpisodesResponseJson {
    public synthetic <init>(java.util.List,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
