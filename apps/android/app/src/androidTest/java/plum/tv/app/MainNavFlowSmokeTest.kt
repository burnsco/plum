package plum.tv.app

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import kotlinx.coroutines.runBlocking
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import okhttp3.mockwebserver.Dispatcher
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import okhttp3.mockwebserver.RecordedRequest
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TestWatcher
import org.junit.runner.Description
import org.junit.runner.RunWith

private val Context.smokeSessionDataStore: DataStore<Preferences> by preferencesDataStore(name = "plum_session")

private val keyServerUrl = stringPreferencesKey("server_url")
private val keySessionToken = stringPreferencesKey("session_token")

private fun jsonResponse(
    code: Int,
    body: String,
): MockResponse =
    MockResponse()
        .setResponseCode(code)
        .addHeader("Content-Type", "application/json; charset=utf-8")
        .setBody(body)

@RunWith(AndroidJUnit4::class)
class MainNavFlowSmokeTest {

    private val server = MockWebServer()

    @get:Rule(order = 0)
    val setupRule: TestWatcher =
        object : TestWatcher() {
            override fun starting(description: Description) {
                server.dispatcher =
                    object : Dispatcher() {
                        override fun dispatch(request: RecordedRequest): MockResponse {
                            val path = request.path ?: ""
                            return when {
                                path == "/ws" ->
                                    MockResponse()
                                        .withWebSocketUpgrade(
                                            object : WebSocketListener() {
                                                override fun onOpen(
                                                    webSocket: WebSocket,
                                                    response: Response,
                                                ) {
                                                    // Keep test server quiet; app may send attach frames.
                                                }
                                            },
                                        )
                                request.method == "POST" && path == "/api/auth/device-login" ->
                                    jsonResponse(200, DEVICE_LOGIN_JSON)
                                request.method == "GET" && path == "/api/auth/me" ->
                                    jsonResponse(200, ME_JSON)
                                request.method == "GET" && path == "/api/home" ->
                                    jsonResponse(200, HOME_JSON)
                                request.method == "GET" && path == "/api/libraries" ->
                                    jsonResponse(200, "[]")
                                request.method == "POST" && path.startsWith("/api/playback/sessions/") ->
                                    jsonResponse(200, PLAYBACK_SESSION_JSON)
                                request.method == "PUT" && path.startsWith("/api/media/") ->
                                    MockResponse().setResponseCode(204)
                                else ->
                                    MockResponse()
                                        .setResponseCode(404)
                                        .setBody("unexpected ${request.method} $path")
                            }
                        }
                    }
                server.start()
                val base = server.url("/").toString().trimEnd('/')
                val ctx = InstrumentationRegistry.getInstrumentation().targetContext
                runBlocking {
                    ctx.smokeSessionDataStore.edit { prefs ->
                        prefs.clear()
                        prefs[keyServerUrl] = base
                        prefs.remove(keySessionToken)
                    }
                }
            }

            override fun finished(description: Description) {
                server.shutdown()
            }
        }

    @get:Rule(order = 1)
    val composeRule = createAndroidComposeRule<MainActivity>()

    @Test
    fun autoLogin_home_playAndBack_returnsToHome() {
        composeRule.waitUntil(timeoutMillis = 60_000) {
            composeRule
                .onAllNodesWithText("Smoke Test Movie", substring = false)
                .fetchSemanticsNodes()
                .isNotEmpty()
        }

        composeRule.onNodeWithText("Play", substring = false).performClick()

        composeRule.waitUntil(timeoutMillis = 90_000) {
            composeRule
                .onAllNodesWithText("Starting", substring = true)
                .fetchSemanticsNodes()
                .isNotEmpty() ||
                composeRule
                    .onAllNodesWithText("Playing", substring = false)
                    .fetchSemanticsNodes()
                    .isNotEmpty() ||
                composeRule
                    .onAllNodesWithText("Preparing", substring = true)
                    .fetchSemanticsNodes()
                    .isNotEmpty()
        }

        InstrumentationRegistry.getInstrumentation().sendKeyDownUpSync(android.view.KeyEvent.KEYCODE_BACK)

        composeRule.waitUntil(timeoutMillis = 30_000) {
            composeRule
                .onAllNodesWithText("Smoke Test Movie", substring = false)
                .fetchSemanticsNodes()
                .isNotEmpty()
        }
    }

    private companion object {
        val DEVICE_LOGIN_JSON =
            """
            {"user":{"id":1,"email":"admin@example.com","is_admin":true},"sessionToken":"smoke-token","expiresAt":"2099-01-01T00:00:00Z"}
            """.trimIndent()

        val ME_JSON =
            """
            {"id":1,"email":"admin@example.com","is_admin":true}
            """.trimIndent()

        val HOME_JSON =
            """
            {
              "continueWatching": [
                {
                  "kind": "movie",
                  "remaining_seconds": 3600,
                  "media": {
                    "id": 42,
                    "library_id": 1,
                    "title": "Smoke Test Movie",
                    "path": "/smoke.mkv",
                    "duration": 7200,
                    "type": "movie"
                  }
                }
              ],
              "recentlyAddedTvEpisodes": [],
              "recentlyAddedTvShows": [],
              "recentlyAddedMovies": [],
              "recentlyAddedAnimeEpisodes": [],
              "recentlyAddedAnimeShows": []
            }
            """.trimIndent()

        val PLAYBACK_SESSION_JSON =
            """
            {
              "delivery": "direct",
              "mediaId": 42,
              "sessionId": "smoke-session",
              "revision": 1,
              "audioIndex": 0,
              "status": "ready",
              "streamUrl": "https://storage.googleapis.com/exoplayer-test-media-0/play.mp3",
              "durationSeconds": 120.0
            }
            """.trimIndent()
    }
}
