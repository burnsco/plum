package plum.tv.core.player

import java.util.Locale
import plum.tv.core.network.EmbeddedSubtitleJson

internal enum class AndroidEmbeddedSubtitleDelivery {
    TextVtt,
    PgsBinary,
}

internal fun EmbeddedSubtitleJson.supportsAndroidTextDelivery(): Boolean {
    if (supported == false) return false
    if (deliveryModes?.any { it.mode == "direct_vtt" || it.mode == "hls_vtt" } == true) {
        return true
    }
    return vttEligible
}

internal fun EmbeddedSubtitleJson.supportsAndroidPgsBinaryDelivery(): Boolean {
    if (supported == false) return false
    if (deliveryModes?.any { it.mode == "pgs_binary" } == true) {
        return true
    }
    return pgsBinaryEligible
}

internal fun EmbeddedSubtitleJson.supportsBurnInDelivery(): Boolean {
    if (supported == false) return false
    return deliveryModes?.any { it.mode == "burn_in" } == true || preferredAndroidDeliveryMode == "burn_in"
}

internal fun EmbeddedSubtitleJson.preferredAndroidEmbeddedSubtitleDelivery(): AndroidEmbeddedSubtitleDelivery? {
    val textEligible = supportsAndroidTextDelivery()
    val pgsEligible = supportsAndroidPgsBinaryDelivery()
    return when {
        textEligible -> AndroidEmbeddedSubtitleDelivery.TextVtt
        pgsEligible -> AndroidEmbeddedSubtitleDelivery.PgsBinary
        else -> null
    }
}

internal fun embeddedSubtitleCodecMatchesTextFormat(
    codec: String?,
    mime: String,
    pgsTextDeliveryEligible: Boolean = false,
): Boolean {
    val c = codec?.trim()?.lowercase(Locale.US).orEmpty()
    if (c.isEmpty()) return false
    val m = mime.lowercase(Locale.US)
    return when {
        // Server converts subrip/ass to WebVTT for HLS subtitle groups, so text/vtt is a valid
        // mime match for any text codec that the HLS manifest would carry.
        c.contains("subrip") || c == "srt" -> m.contains("subrip") || m.contains("x-subrip") || m.contains("vtt")
        c.contains("ass") || c.contains("ssa") -> m.contains("ass") || m.contains("ssa") || m.contains("vtt")
        c.contains("webvtt") || c == "text" || c == "mov_text" || c == "hdmv_text_subtitle" ->
            m.contains("vtt") || m.contains("webvtt")
        c.contains("ttml") || c == "tx3g" -> m.contains("ttml")
        c.contains("pgs") || c.contains("pgssub") || c == "hdmv_pgs_subtitle" ->
            m.contains("pgs") || m.contains("dvdsub") || (pgsTextDeliveryEligible && m.contains("vtt"))
        else -> false
    }
}
