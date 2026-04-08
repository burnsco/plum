# Moshi Kotlin codegen: keep generated *JsonAdapter for each @JsonClass DTO.
-if @com.squareup.moshi.JsonClass class *
-keep,allowobfuscation class <1>JsonAdapter { *; }
-keep,allowobfuscation class <1>JsonAdapter$* { *; }

# Retrofit reflects on service interface methods and generic signatures.
-keepattributes Signature, InnerClasses, EnclosingMethod, AnnotationDefault
-keepclassmembers,allowshrinking,allowobfuscation interface * {
    @retrofit2.http.* <methods>;
}
