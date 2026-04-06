# App release shrinker: rules beyond library consumer-rules (Hilt, OkHttp, Kotlin).

# OkHttp / Okio (Retrofit; OkHttp platform bits)
-dontwarn okhttp3.internal.platform.**
-dontwarn org.conscrypt.**
-dontwarn org.bouncycastle.**
-dontwarn org.openjsse.**

# Kotlin metadata (Moshi / reflection)
-keepclassmembers class kotlin.Metadata { *; }

# Explicit Retrofit entry: Plum API (belt-and-suspenders with interface * rule in core-network)
-keep,allowobfuscation interface plum.tv.core.network.PlumApi { *; }
