-keepnames class plum.tv.core.network.LibraryMediaPageJson
-if class plum.tv.core.network.LibraryMediaPageJson
-keep class plum.tv.core.network.LibraryMediaPageJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.LibraryMediaPageJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.LibraryMediaPageJson {
    public synthetic <init>(java.util.List,java.lang.Integer,boolean,java.lang.Integer,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
