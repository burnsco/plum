package plum.tv.core.player

/** Picker / WebSocket track id for subtitle rows (off, Exo text group, or server burn-in stream index). */
sealed class SubtitlePickerTrackId {
    data object Off : SubtitlePickerTrackId()

    data class TextTrack(val groupIndex: Int, val trackIndex: Int) : SubtitlePickerTrackId()

    data class BurnIn(val streamIndex: Int) : SubtitlePickerTrackId()

    fun toWireId(): String =
        when (this) {
            is Off -> "off"
            is TextTrack -> "t:$groupIndex:$trackIndex"
            is BurnIn -> "burn:$streamIndex"
        }

    companion object {
        fun parse(raw: String): SubtitlePickerTrackId? =
            when {
                raw == "off" -> Off
                raw.startsWith("burn:") ->
                    raw.removePrefix("burn:").toIntOrNull()?.let { BurnIn(it) }
                raw.startsWith("t:") -> {
                    val parts = raw.removePrefix("t:").split(":")
                    if (parts.size != 2) {
                        null
                    } else {
                        val gi = parts[0].toIntOrNull()
                        val j = parts[1].toIntOrNull()
                        if (gi == null || j == null) null else TextTrack(gi, j)
                    }
                }
                else -> null
            }
    }
}
