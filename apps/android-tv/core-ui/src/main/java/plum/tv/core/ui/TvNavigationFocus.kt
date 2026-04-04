package plum.tv.core.ui

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.focus.FocusRequester
import kotlinx.coroutines.delay

private const val TV_FOCUS_SETTLE_DELAY_MS = 48L

/**
 * After [keys] change, requests focus on [focusRequester] once composition has settled.
 * Use when async content becomes ready so focus leaves the side rail without an extra D-pad move.
 */
@Composable
fun LaunchedTvFocusTo(vararg keys: Any?, focusRequester: FocusRequester) {
    if (keys.isEmpty()) {
        LaunchedEffect(Unit) {
            delay(TV_FOCUS_SETTLE_DELAY_MS)
            focusRequester.requestFocus()
        }
    } else {
        LaunchedEffect(*keys) {
            delay(TV_FOCUS_SETTLE_DELAY_MS)
            focusRequester.requestFocus()
        }
    }
}
