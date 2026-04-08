package plum.tv.core.network

import org.junit.Assert.assertEquals
import org.junit.Test

class DiscoverOriginTest {
    @Test
    fun normalizeKey_emptyOrInvalid() {
        assertEquals("", DiscoverOrigin.normalizeKey(null))
        assertEquals("", DiscoverOrigin.normalizeKey(""))
        assertEquals("", DiscoverOrigin.normalizeKey("   "))
        assertEquals("", DiscoverOrigin.normalizeKey("G"))
        assertEquals("", DiscoverOrigin.normalizeKey("GBR"))
        assertEquals("", DiscoverOrigin.normalizeKey("g1"))
        assertEquals("", DiscoverOrigin.normalizeKey("1G"))
    }

    @Test
    fun normalizeKey_validAlpha2() {
        assertEquals("GB", DiscoverOrigin.normalizeKey("gb"))
        assertEquals("GB", DiscoverOrigin.normalizeKey(" GB "))
        assertEquals("US", DiscoverOrigin.normalizeKey("us"))
    }
}
