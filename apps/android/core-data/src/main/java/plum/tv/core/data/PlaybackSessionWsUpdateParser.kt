package plum.tv.core.data

import android.util.Log
import com.squareup.moshi.JsonAdapter
import plum.tv.core.network.PlaybackSessionUpdateEventJson

private const val TAG = "PlumTV"

internal fun parsePlaybackSessionWsUpdate(
    adapter: JsonAdapter<PlaybackSessionUpdateEventJson>,
    text: String,
): PlaybackSessionUpdateEventJson? {
    return runCatching { adapter.fromJson(text) }
        .onFailure { e ->
            Log.w(TAG, "ws playback_session_update parse failed: ${e.message} text=${text.take(256)}")
        }
        .getOrNull()
        ?.takeIf { it.type == "playback_session_update" }
}
