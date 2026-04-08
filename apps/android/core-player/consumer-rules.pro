# Media3 / ExoPlayer: default extractors, renderers, and codec paths are loaded via reflection.
-keep class androidx.media3.common.** { *; }
-keep class androidx.media3.exoplayer.** { *; }
-keep class androidx.media3.decoder.** { *; }
-keep class androidx.media3.extractor.** { *; }
-keep class androidx.media3.container.** { *; }
-dontwarn androidx.media3.**
