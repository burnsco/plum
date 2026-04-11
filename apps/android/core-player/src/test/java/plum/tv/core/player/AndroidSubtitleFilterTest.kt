package plum.tv.core.player

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class AndroidSubtitleFilterTest {
    @Test
    fun isPreferredAndroidTvSubtitleLanguage_acceptsEnglishLanguageCodes() {
        assertTrue(isPreferredAndroidTvSubtitleLanguage(language = "en", title = ""))
        assertTrue(isPreferredAndroidTvSubtitleLanguage(language = "eng", title = ""))
    }

    @Test
    fun isPreferredAndroidTvSubtitleLanguage_acceptsBritishAndEnglishTitles() {
        assertTrue(isPreferredAndroidTvSubtitleLanguage(language = "", title = "British (sdh)"))
        assertTrue(isPreferredAndroidTvSubtitleLanguage(language = "", title = "English"))
    }

    @Test
    fun isPreferredAndroidTvSubtitleLanguage_rejectsNonEnglishTitlesAndLanguages() {
        assertFalse(isPreferredAndroidTvSubtitleLanguage(language = "ja", title = "Japanese"))
        assertFalse(isPreferredAndroidTvSubtitleLanguage(language = "es", title = "Latin American"))
    }
}
