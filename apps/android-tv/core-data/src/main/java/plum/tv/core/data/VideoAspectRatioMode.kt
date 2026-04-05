package plum.tv.core.data

/**
 * How video is scaled inside the player surface. [AUTO] follows the decoded stream dimensions.
 */
enum class VideoAspectRatioMode(val storageValue: String) {
    AUTO("auto"),
    /** Crop to fill the screen (no empty bars). */
    ZOOM("zoom"),
    /** Distort to fill the screen. */
    STRETCH("stretch"),
    /** Letterbox/pillar inside a 16:9 frame. */
    RATIO_16_9("ratio-16-9"),
    RATIO_4_3("ratio-4-3"),
    RATIO_21_9("ratio-21-9"),
    ;

    companion object {
        fun fromStorage(raw: String?): VideoAspectRatioMode {
            if (raw.isNullOrBlank()) return AUTO
            return entries.find { it.storageValue == raw.trim() } ?: AUTO
        }
    }
}
