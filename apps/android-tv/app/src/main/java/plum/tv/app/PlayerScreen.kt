package plum.tv.app

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.media3.common.util.UnstableApi
import androidx.media3.ui.PlayerView
import android.view.KeyEvent as AndroidKeyEvent
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.automirrored.filled.VolumeUp
import androidx.compose.material.icons.filled.Forward10
import androidx.compose.material.icons.filled.Pause
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Replay10
import androidx.compose.material.icons.filled.SkipNext
import androidx.compose.material.icons.filled.SkipPrevious
import androidx.compose.material.icons.filled.Subtitles
import androidx.tv.material3.Text
import plum.tv.core.ui.PlumActionButton
import plum.tv.core.ui.PlumButtonVariant
import plum.tv.core.ui.PlumMetadataChips
import plum.tv.core.ui.PlumPanel
import plum.tv.core.ui.PlumTheme
import plum.tv.core.ui.PlumTvScaffold

@UnstableApi
@Composable
fun PlayerRoute(
    onClose: () -> Unit,
    viewModel: PlayerViewModel = hiltViewModel(),
) {
    val ui by viewModel.uiState.collectAsState()
    val focusRequester = remember { FocusRequester() }

    BackHandler {
        onClose()
    }

    LaunchedEffect(Unit) {
        focusRequester.requestFocus()
    }

    Box(
        modifier =
                Modifier
                    .fillMaxSize()
                    .background(Color.Black)
                    .onPreviewKeyEvent { event ->
                        if (event.nativeKeyEvent.action != AndroidKeyEvent.ACTION_UP) return@onPreviewKeyEvent false
                        when (event.nativeKeyEvent.keyCode) {
                            AndroidKeyEvent.KEYCODE_MEDIA_PLAY_PAUSE,
                            AndroidKeyEvent.KEYCODE_MEDIA_PLAY,
                            AndroidKeyEvent.KEYCODE_MEDIA_PAUSE -> {
                                viewModel.togglePlayPause()
                                true
                            }
                            AndroidKeyEvent.KEYCODE_MEDIA_FAST_FORWARD -> {
                                viewModel.forward10()
                                true
                            }
                            AndroidKeyEvent.KEYCODE_MEDIA_REWIND -> {
                                viewModel.rewind10()
                                true
                            }
                            AndroidKeyEvent.KEYCODE_MEDIA_NEXT -> {
                                viewModel.nextEpisode()
                                true
                            }
                            AndroidKeyEvent.KEYCODE_MEDIA_PREVIOUS -> {
                                viewModel.previousEpisode()
                                true
                            }
                            else -> false
                        }
                    },
        ) {
        AndroidView(
            modifier = Modifier.fillMaxSize(),
            factory = { ctx ->
                PlayerView(ctx).apply {
                    useController = false
                    player = viewModel.player
                }
            },
            update = {
                if (it.player !== viewModel.player) {
                    it.player = viewModel.player
                }
            },
        )

        Box(
            modifier =
                Modifier
                    .fillMaxSize()
                    .background(
                        Brush.verticalGradient(
                            colors = listOf(
                                Color.Transparent,
                                Color(0x10000000),
                                Color(0x24000000),
                                Color(0x72000000),
                                Color(0xE1000000),
                            ),
                        ),
                    ),
        )

        Column(
            modifier =
                Modifier
                    .align(Alignment.TopStart)
                    .padding(horizontal = 36.dp, vertical = 24.dp)
                    .widthIn(max = 660.dp),
            verticalArrangement = Arrangement.spacedBy(6.dp),
        ) {
            Text(
                text = "Now Playing",
                style = PlumTheme.typography.labelSmall,
                color = PlumTheme.palette.muted,
            )
            Text(
                text = ui.title,
                style = PlumTheme.typography.headlineSmall,
                color = Color.White,
            )
            ui.subtitle?.takeIf { it.isNotBlank() }?.let { subtitle ->
                Text(
                    text = subtitle,
                    style = PlumTheme.typography.bodyMedium,
                    color = Color.White.copy(alpha = 0.68f),
                )
            }
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                PlumMetadataChips(
                    values = buildList {
                        add(ui.status)
                        val progress = formatPlayerTime(ui.positionMs)
                        val remaining = formatPlayerTime(ui.remainingMs)
                        if (progress.isNotBlank()) add(progress)
                        if (remaining.isNotBlank()) add("-$remaining")
                    },
                )
            }
            ui.error?.takeIf { it.isNotBlank() }?.let { err ->
                Text(
                    text = err,
                    style = PlumTheme.typography.bodySmall,
                    color = Color(0xFFFF8A8A),
                )
            }
        }

        PlumPanel(
            modifier =
                Modifier
                    .align(Alignment.BottomCenter)
                    .padding(horizontal = 28.dp, vertical = 24.dp)
                    .fillMaxWidth(),
            contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = 24.dp, vertical = 20.dp),
        ) {
            Column(verticalArrangement = Arrangement.spacedBy(16.dp)) {
                Box(
                    modifier =
                        Modifier
                            .fillMaxWidth()
                            .height(1.dp)
                            .background(PlumTheme.palette.borderStrong.copy(alpha = 0.35f)),
                )
                Box(
                    modifier =
                        Modifier
                            .fillMaxWidth()
                            .height(5.dp)
                            .background(PlumTheme.palette.surface),
                ) {
                    Box(
                        modifier =
                            Modifier
                                .fillMaxWidth(fraction = ui.progressFraction)
                                .height(5.dp)
                                .background(PlumTheme.palette.accent),
                    )
                }

                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                ) {
                    Text(
                        text = formatPlayerTime(ui.positionMs),
                        style = PlumTheme.typography.labelSmall,
                        color = PlumTheme.palette.textSecondary,
                    )
                    Text(
                        text = formatPlayerTime(ui.remainingMs),
                        style = PlumTheme.typography.labelSmall,
                        color = PlumTheme.palette.textSecondary,
                    )
                }

                Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        PlumActionButton(
                            label = "Previous",
                            onClick = { viewModel.previousEpisode() },
                            variant = PlumButtonVariant.Secondary,
                            leadingIcon = Icons.Filled.SkipPrevious,
                            enabled = ui.canPrev,
                        )
                        PlumActionButton(
                            label = "Rewind 10",
                            onClick = { viewModel.rewind10() },
                            variant = PlumButtonVariant.Secondary,
                            leadingIcon = Icons.Filled.Replay10,
                        )
                        PlumActionButton(
                            label = if (ui.isPlaying) "Pause" else "Play",
                            onClick = { viewModel.togglePlayPause() },
                            modifier = Modifier.focusRequester(focusRequester),
                            leadingIcon = if (ui.isPlaying) Icons.Filled.Pause else Icons.Filled.PlayArrow,
                        )
                        PlumActionButton(
                            label = "Forward 10",
                            onClick = { viewModel.forward10() },
                            variant = PlumButtonVariant.Secondary,
                            leadingIcon = Icons.Filled.Forward10,
                        )
                        PlumActionButton(
                            label = "Next",
                            onClick = { viewModel.nextEpisode() },
                            variant = PlumButtonVariant.Secondary,
                            leadingIcon = Icons.Filled.SkipNext,
                            enabled = ui.canNext,
                        )
                    }
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        PlumActionButton(
                            label = trackButtonLabel("Aud", ui.audioTrackLabel, ui.canCycleAudio),
                            onClick = { viewModel.cycleAudioTrack() },
                            variant = PlumButtonVariant.Secondary,
                            leadingIcon = Icons.AutoMirrored.Filled.VolumeUp,
                            enabled = ui.canCycleAudio,
                        )
                        PlumActionButton(
                            label = trackButtonLabel("CC", ui.subtitleTrackLabel, ui.canCycleSubtitles),
                            onClick = { viewModel.cycleSubtitles() },
                            variant = PlumButtonVariant.Secondary,
                            leadingIcon = Icons.Filled.Subtitles,
                            enabled = ui.canCycleSubtitles,
                        )
                        Spacer(modifier = Modifier.weight(1f))
                        PlumActionButton(
                            label = "Back",
                            onClick = onClose,
                            variant = PlumButtonVariant.Ghost,
                            leadingIcon = Icons.AutoMirrored.Filled.ArrowBack,
                        )
                    }
                }
            }
        }
    }
}

private fun trackButtonLabel(base: String, value: String?, enabled: Boolean): String =
    if (!enabled) {
        "$base: Off"
    } else if (!value.isNullOrBlank()) {
        "$base: ${compactTrackValue(value)}"
    } else {
        base
    }

private fun compactTrackValue(value: String): String {
    val cleaned = value.trim().replace(Regex("\\s+"), " ")
    return when {
        cleaned.length <= 10 -> cleaned
        cleaned.length <= 14 -> cleaned.take(12).trimEnd() + "…"
        else -> cleaned.take(9).trimEnd() + "…"
    }
}

private fun formatPlayerTime(ms: Long): String {
    if (ms <= 0) return "0:00"
    val totalSeconds = ms / 1000
    val hours = totalSeconds / 3600
    val minutes = (totalSeconds % 3600) / 60
    val seconds = totalSeconds % 60
    return if (hours > 0) {
        "%d:%02d:%02d".format(hours, minutes, seconds)
    } else {
        "%d:%02d".format(minutes, seconds)
    }
}
