package plum.tv.core.player

data class TrackPickerOption(
    val id: String,
    val label: String,
    val selected: Boolean,
)

sealed class TrackPicker {
    abstract val title: String
    abstract val options: List<TrackPickerOption>

    data class Subtitles(
        override val title: String = "Subtitles",
        override val options: List<TrackPickerOption>,
    ) : TrackPicker()

    data class Audio(
        override val title: String = "Audio",
        override val options: List<TrackPickerOption>,
    ) : TrackPicker()
}
