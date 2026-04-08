-keepnames class plum.tv.core.network.ShowSeasonEpisodesJson
-if class plum.tv.core.network.ShowSeasonEpisodesJson
-keep class plum.tv.core.network.ShowSeasonEpisodesJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.ShowSeasonEpisodesJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.ShowSeasonEpisodesJson {
    public synthetic <init>(int,java.lang.String,java.util.List,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
