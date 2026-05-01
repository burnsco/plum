package plum.tv.core.player

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import plum.tv.core.network.EmbeddedSubtitleDeliveryModeJson
import plum.tv.core.network.EmbeddedSubtitleJson

class SubtitleDeliveryTest {
    @Test
    fun preferredAndroidEmbeddedSubtitleDelivery_prefersVttOverPgsWhenBothAreEligible() {
        val subtitle =
            EmbeddedSubtitleJson(
                codec = "hdmv_pgs_subtitle",
                supported = true,
                vttEligible = true,
                pgsBinaryEligible = true,
            )

        assertEquals(AndroidEmbeddedSubtitleDelivery.TextVtt, subtitle.preferredAndroidEmbeddedSubtitleDelivery())
    }

    @Test
    fun preferredAndroidEmbeddedSubtitleDelivery_usesPgsOnlyWhenTextDeliveryIsUnavailable() {
        val subtitle =
            EmbeddedSubtitleJson(
                codec = "hdmv_pgs_subtitle",
                supported = true,
                vttEligible = false,
                pgsBinaryEligible = true,
            )

        assertEquals(AndroidEmbeddedSubtitleDelivery.PgsBinary, subtitle.preferredAndroidEmbeddedSubtitleDelivery())
    }

    @Test
    fun preferredAndroidEmbeddedSubtitleDelivery_honorsDeliveryModesContract() {
        val subtitle =
            EmbeddedSubtitleJson(
                codec = "hdmv_pgs_subtitle",
                supported = true,
                deliveryModes =
                    listOf(
                        EmbeddedSubtitleDeliveryModeJson(mode = "direct_vtt"),
                        EmbeddedSubtitleDeliveryModeJson(mode = "pgs_binary"),
                    ),
            )

        assertEquals(AndroidEmbeddedSubtitleDelivery.TextVtt, subtitle.preferredAndroidEmbeddedSubtitleDelivery())
    }

    @Test
    fun embeddedSubtitleCodecMatchesTextFormat_allowsPgsToMatchVttOnlyWhenTextDeliveryIsEligible() {
        assertTrue(
            embeddedSubtitleCodecMatchesTextFormat(
                codec = "hdmv_pgs_subtitle",
                mime = "text/vtt",
                pgsTextDeliveryEligible = true,
            ),
        )
        assertFalse(
            embeddedSubtitleCodecMatchesTextFormat(
                codec = "hdmv_pgs_subtitle",
                mime = "text/vtt",
                pgsTextDeliveryEligible = false,
            ),
        )
    }
}
