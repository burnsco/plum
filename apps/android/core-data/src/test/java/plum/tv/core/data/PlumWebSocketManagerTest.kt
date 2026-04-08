package plum.tv.core.data

import com.squareup.moshi.Moshi
import com.squareup.moshi.kotlin.reflect.KotlinJsonAdapterFactory
import io.mockk.every
import io.mockk.mockk
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.runBlocking
import okhttp3.OkHttpClient
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import okhttp3.mockwebserver.Dispatcher
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import okhttp3.mockwebserver.RecordedRequest
import org.junit.Assert.assertTrue
import org.junit.Test

class PlumWebSocketManagerTest {

    private val moshi =
        Moshi.Builder()
            .addLast(KotlinJsonAdapterFactory())
            .build()

    @Test
    fun start_opensWebSocketWhenCredentialsPresent() =
        runBlocking {
            val server = newWebSocketServer()
            server.mock.start()
            val baseUrl = server.mock.url("/").toString().trimEnd('/')
            try {
                val prefs = mockk<SessionPreferences>(relaxed = true)
                every { prefs.serverUrl } returns MutableStateFlow(baseUrl)
                val tokenBridge = AuthTokenBridge()
                tokenBridge.setToken("unit-test-token")
                val catalog = mockk<LibraryCatalogRefreshCoordinator>(relaxed = true)
                val mgr =
                    PlumWebSocketManager(
                        OkHttpClient(),
                        prefs,
                        tokenBridge,
                        moshi,
                        catalog,
                    )
                try {
                    mgr.start()
                    assertTrue(server.opened.await(20, TimeUnit.SECONDS))
                } finally {
                    mgr.stop()
                }
            } finally {
                server.mock.shutdown()
            }
        }

    @Test
    fun start_connectsAfterCredentialsAppearFollowingMissingAuth() =
        runBlocking {
            val server = newWebSocketServer()
            server.mock.start()
            val baseUrl = server.mock.url("/").toString().trimEnd('/')
            try {
                val urlState = MutableStateFlow<String?>(null)
                val prefs = mockk<SessionPreferences>(relaxed = true)
                every { prefs.serverUrl } returns urlState
                val tokenBridge = AuthTokenBridge()
                val catalog = mockk<LibraryCatalogRefreshCoordinator>(relaxed = true)
                val mgr =
                    PlumWebSocketManager(
                        OkHttpClient(),
                        prefs,
                        tokenBridge,
                        moshi,
                        catalog,
                    )
                try {
                    mgr.start()
                    delay(300)
                    urlState.value = baseUrl
                    tokenBridge.setToken("late-token")
                    assertTrue(server.opened.await(25, TimeUnit.SECONDS))
                } finally {
                    mgr.stop()
                }
            } finally {
                server.mock.shutdown()
            }
        }

    private fun newWebSocketServer(): WsServer {
        val opened = CountDownLatch(1)
        val mock = MockWebServer()
        mock.dispatcher =
            object : Dispatcher() {
                override fun dispatch(request: RecordedRequest): MockResponse {
                    if (request.path != "/ws") {
                        return MockResponse().setResponseCode(404)
                    }
                    return MockResponse()
                        .withWebSocketUpgrade(
                            object : WebSocketListener() {
                                override fun onOpen(
                                    webSocket: WebSocket,
                                    response: Response,
                                ) {
                                    opened.countDown()
                                }
                            },
                        )
                }
            }
        return WsServer(mock, opened)
    }

    private data class WsServer(
        val mock: MockWebServer,
        val opened: CountDownLatch,
    )
}
