package plum.tv.core.player

import androidx.media3.common.C
import org.junit.Assert.assertEquals
import org.junit.Test

class SubtitleMetadataFlagsTest {
    @Test
    fun subtitleSelectionFlags_marksDefaultForcedAndAutoselect() {
        val flags =
            subtitleSelectionFlags(
                default = true,
                forced = true,
                hearingImpaired = false,
            )

        assertEquals(
            C.SELECTION_FLAG_DEFAULT or C.SELECTION_FLAG_FORCED or C.SELECTION_FLAG_AUTOSELECT,
            flags,
        )
    }

    @Test
    fun subtitleSelectionFlags_avoidsAutoselectForHearingImpairedOnlyTracks() {
        val flags =
            subtitleSelectionFlags(
                default = false,
                forced = false,
                hearingImpaired = true,
            )

        assertEquals(0, flags)
    }

    @Test
    fun subtitleRoleFlags_usesCaptionForHearingImpairedTracks() {
        val roleFlags = subtitleRoleFlags(hearingImpaired = true)

        assertEquals(C.ROLE_FLAG_CAPTION, roleFlags)
    }

    @Test
    fun subtitleRoleFlags_usesSubtitleForRegularTracks() {
        val roleFlags = subtitleRoleFlags(hearingImpaired = false)

        assertEquals(C.ROLE_FLAG_SUBTITLE, roleFlags)
    }
}
