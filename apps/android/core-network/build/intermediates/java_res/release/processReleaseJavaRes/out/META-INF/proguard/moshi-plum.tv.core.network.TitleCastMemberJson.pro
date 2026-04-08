-keepnames class plum.tv.core.network.TitleCastMemberJson
-if class plum.tv.core.network.TitleCastMemberJson
-keep class plum.tv.core.network.TitleCastMemberJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.TitleCastMemberJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.TitleCastMemberJson {
    public synthetic <init>(java.lang.String,java.lang.String,java.lang.Integer,java.lang.String,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
