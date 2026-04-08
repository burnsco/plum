-keepnames class plum.tv.core.network.LibraryScanStatusJson
-if class plum.tv.core.network.LibraryScanStatusJson
-keep class plum.tv.core.network.LibraryScanStatusJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.LibraryScanStatusJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.LibraryScanStatusJson {
    public synthetic <init>(int,java.lang.String,java.lang.String,boolean,java.lang.String,int,int,int,int,int,int,int,int,boolean,java.lang.String,java.lang.String,java.lang.String,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
