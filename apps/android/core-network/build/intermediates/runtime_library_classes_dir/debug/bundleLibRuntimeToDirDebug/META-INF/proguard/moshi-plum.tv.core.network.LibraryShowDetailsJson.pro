-keepnames class plum.tv.core.network.LibraryShowDetailsJson
-if class plum.tv.core.network.LibraryShowDetailsJson
-keep class plum.tv.core.network.LibraryShowDetailsJsonJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
-if class plum.tv.core.network.LibraryShowDetailsJson
-keepnames class kotlin.jvm.internal.DefaultConstructorMarker
-keepclassmembers class plum.tv.core.network.LibraryShowDetailsJson {
    public synthetic <init>(int,java.lang.String,java.lang.String,java.lang.String,java.lang.String,java.lang.String,java.lang.String,java.lang.String,java.lang.String,java.lang.Double,java.lang.String,java.lang.Double,java.lang.Integer,int,int,java.util.List,java.util.List,int,kotlin.jvm.internal.DefaultConstructorMarker);
}
