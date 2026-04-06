package plum.tv.core.network

import org.junit.Assert.assertEquals
import org.junit.Test

class PlumHttpMessagesTest {
    @Test
    fun preferBody_usesTrimmedBodyWhenPresent() {
        assertEquals("nope", PlumHttpMessages.preferBody("Search", 500, "  nope  "))
    }

    @Test
    fun preferBody_fallsBackToLabelAndCode() {
        assertEquals("Search: 503", PlumHttpMessages.preferBody("Search", 503, null))
        assertEquals("Search: 503", PlumHttpMessages.preferBody("Search", 503, "   "))
    }

    @Test
    fun statusWithAppendedBody_matchesWebLibrariesShape() {
        assertEquals("Libraries: 500", PlumHttpMessages.statusWithAppendedBody("Libraries", 500, null))
        assertEquals("Libraries: 500 oops", PlumHttpMessages.statusWithAppendedBody("Libraries", 500, "oops"))
    }

    @Test
    fun deviceLoginFailed_defaultsLikeWeb() {
        assertEquals("Invalid credentials.", PlumHttpMessages.deviceLoginFailed(null))
        assertEquals("Bad password", PlumHttpMessages.deviceLoginFailed("Bad password"))
    }
}
