package plum.tv.core.player

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class SubtitleTrackSourceLabelTest {
    @Test
    fun subtitleTrackSourceLabelForFormatId_marksSideloadTracks() {
        assertEquals("sideload", subtitleTrackSourceLabelForFormatId("ext:11"))
        assertEquals("sideload", subtitleTrackSourceLabelForFormatId("emb:7"))
    }

    @Test
    fun subtitleTrackSourceLabelForFormatId_marksFallbackTracks() {
        assertEquals("fallback", subtitleTrackSourceLabelForFormatId("hls:subtitle:0"))
    }

    @Test
    fun subtitleTrackSourceLabelForFormatId_ignoresBlankIds() {
        assertNull(subtitleTrackSourceLabelForFormatId("  "))
        assertNull(subtitleTrackSourceLabelForFormatId(null))
    }
}
