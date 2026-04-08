-keepnames class plum.tv.core.network.LibraryJson
-if class plum.tv.core.network.LibraryJson
-keep class plum.tv.core.network.LibraryJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.LibraryJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.LibraryJson {
    public synthetic <init>(int,java.lang.String,java.lang.String,java.lang.String,int,java.lang.String,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
