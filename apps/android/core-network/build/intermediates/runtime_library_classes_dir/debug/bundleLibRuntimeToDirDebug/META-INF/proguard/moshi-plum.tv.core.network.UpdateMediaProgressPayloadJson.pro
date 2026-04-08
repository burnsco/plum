-keepnames class plum.tv.core.network.UpdateMediaProgressPayloadJson
-if class plum.tv.core.network.UpdateMediaProgressPayloadJson
-keep class plum.tv.core.network.UpdateMediaProgressPayloadJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.UpdateMediaProgressPayloadJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.UpdateMediaProgressPayloadJson {
    public synthetic <init>(double,double,java.lang.Boolean,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
