package plum.tv.core.data

enum class SubtitleSize(val storageValue: String) {
    SMALL("small"),
    MEDIUM("medium"),
    LARGE("large"),
    ;

    companion object {
        fun fromStorage(raw: String?): SubtitleSize =
            entries.find { it.storageValue == raw } ?: MEDIUM
    }
}

enum class SubtitlePosition(val storageValue: String) {
    BOTTOM("bottom"),
    TOP("top"),
    ;

    companion object {
        fun fromStorage(raw: String?): SubtitlePosition =
            entries.find { it.storageValue == raw } ?: BOTTOM
    }
}

data class SubtitleAppearance(
    val size: SubtitleSize,
    val position: SubtitlePosition,
    val colorHex: String,
) {
    companion object {
        val DEFAULT = SubtitleAppearance(SubtitleSize.MEDIUM, SubtitlePosition.BOTTOM, "#ffffff")
    }
}
