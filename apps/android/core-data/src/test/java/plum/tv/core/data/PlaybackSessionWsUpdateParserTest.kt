package plum.tv.core.data

import com.squareup.moshi.Moshi
import com.squareup.moshi.kotlin.reflect.KotlinJsonAdapterFactory
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test
import plum.tv.core.network.PlaybackSessionUpdateEventJson

class PlaybackSessionWsUpdateParserTest {
    private val adapter =
        Moshi.Builder()
            .addLast(KotlinJsonAdapterFactory())
            .build()
            .adapter(PlaybackSessionUpdateEventJson::class.java)

    @Test
    fun parsePlaybackSessionUpdate_acceptsValidPayload() {
        val json =
            """
            {
              "type": "playback_session_update",
              "sessionId": "s1",
              "delivery": "hls",
              "mediaId": 42,
              "revision": 3,
              "audioIndex": 0,
              "status": "ready",
              "streamUrl": "https://example/stream.m3u8",
              "durationSeconds": 3600.0
            }
            """.trimIndent()

        val parsed = parsePlaybackSessionWsUpdate(adapter, json)
        requireNotNull(parsed)
        assertEquals("playback_session_update", parsed.type)
        assertEquals("s1", parsed.sessionId)
        assertEquals(42, parsed.mediaId)
        assertEquals(3, parsed.revision)
    }

    @Test
    fun parsePlaybackSessionUpdate_rejectsWrongType() {
        val json =
            """
            {
              "type": "other_event",
              "sessionId": "s1",
              "delivery": "hls",
              "mediaId": 1,
              "audioIndex": 0,
              "status": "ready",
              "streamUrl": "x",
              "durationSeconds": 1.0
            }
            """.trimIndent()

        assertNull(parsePlaybackSessionWsUpdate(adapter, json))
    }

    @Test
    fun parsePlaybackSessionUpdate_returnsNullOnInvalidJson() {
        assertNull(parsePlaybackSessionWsUpdate(adapter, "not json"))
    }
}
