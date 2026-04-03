package plum.tv.app

import android.view.KeyEvent as AndroidKeyEvent
import android.view.ViewGroup
import androidx.activity.compose.BackHandler
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
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
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
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
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.LocalContentColor
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusTarget
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.coerceAtLeast
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.media3.common.util.UnstableApi
import androidx.media3.ui.AspectRatioFrameLayout
import androidx.media3.ui.PlayerView
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.Glow
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import kotlinx.coroutines.delay
import plum.tv.core.ui.PlumTheme
import plum.tv.core.ui.plumBorder

private const val CONTROLS_HIDE_DELAY_MS = 5_000L

@UnstableApi
@Composable
fun PlayerRoute(
    onClose: () -> Unit,
    viewModel: PlayerViewModel = hiltViewModel(),
) {
    val ui by viewModel.uiState.collectAsState()
    val rootFocusRequester = remember { FocusRequester() }
    val playFocusRequester = remember { FocusRequester() }

    // Each time hideTimerKey changes, the LaunchedEffect restarts the hide countdown.
    var hideTimerKey by remember { mutableIntStateOf(0) }
    var controlsVisible by remember { mutableStateOf(true) }

    fun showControls() {
        controlsVisible = true
        hideTimerKey++
    }

    // Auto-hide controls after inactivity when playing (read StateFlow after delay so isPlaying is not stale).
    LaunchedEffect(hideTimerKey) {
        delay(CONTROLS_HIDE_DELAY_MS)
        if (viewModel.uiState.value.isPlaying) {
            controlsVisible = false
        }
    }

    // Always show controls when paused or buffering
    LaunchedEffect(ui.isPlaying, ui.isBuffering) {
        if (!ui.isPlaying || ui.isBuffering) {
            showControls()
        }
    }

    BackHandler {
        if (controlsVisible) {
            onClose()
        } else {
            showControls()
        }
    }

    LaunchedEffect(Unit) {
        rootFocusRequester.requestFocus()
    }

    LaunchedEffect(controlsVisible) {
        if (controlsVisible) {
            playFocusRequester.requestFocus()
        } else {
            rootFocusRequester.requestFocus()
        }
    }

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Color.Black)
            .focusRequester(rootFocusRequester)
            .focusTarget()
            .onPreviewKeyEvent { event ->
                // Every key event (including d-pad navigation) resets the hide timer
                if (event.nativeKeyEvent.action == AndroidKeyEvent.ACTION_DOWN) {
                    showControls()
                }
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
        // ── Video surface ────────────────────────────────────────────────────────
        AndroidView(
            modifier = Modifier.fillMaxSize(),
            factory = { ctx ->
                PlayerView(ctx).apply {
                    useController = false
                    isFocusable = false
                    isFocusableInTouchMode = false
                    descendantFocusability = ViewGroup.FOCUS_BLOCK_DESCENDANTS
                    resizeMode = AspectRatioFrameLayout.RESIZE_MODE_FIT
                    player = viewModel.player
                }
            },
            update = { view ->
                if (view.player !== viewModel.player) {
                    view.player = viewModel.player
                }
                view.isFocusable = false
                view.isFocusableInTouchMode = false
                view.descendantFocusability = ViewGroup.FOCUS_BLOCK_DESCENDANTS
            },
        )

        // ── Gradient overlay (only when controls are visible) ────────────────────
        AnimatedVisibility(
            visible = controlsVisible,
            enter = fadeIn(),
            exit = fadeOut(),
        ) {
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .background(
                        Brush.verticalGradient(
                            0.0f to Color(0xCC000000),
                            0.25f to Color(0x44000000),
                            0.6f to Color(0x22000000),
                            0.8f to Color(0x66000000),
                            1.0f to Color(0xDD000000),
                        ),
                    ),
            )
        }

        // ── Buffering spinner (always visible regardless of controls state) ───────
        if (ui.isBuffering) {
            Box(
                modifier = Modifier.fillMaxSize(),
                contentAlignment = Alignment.Center,
            ) {
                CircularProgressIndicator(
                    modifier = Modifier.size(56.dp),
                    color = Color.White.copy(alpha = 0.85f),
                    strokeWidth = 3.dp,
                )
            }
        }

        // ── Top metadata bar ─────────────────────────────────────────────────────
        AnimatedVisibility(
            visible = controlsVisible,
            enter = fadeIn(),
            exit = fadeOut(),
            modifier = Modifier.align(Alignment.TopStart),
        ) {
            Column(
                modifier = Modifier
                    .padding(horizontal = 40.dp, vertical = 28.dp)
                    .widthIn(max = 640.dp),
                verticalArrangement = Arrangement.spacedBy(4.dp),
            ) {
                ui.subtitle?.takeIf { it.isNotBlank() }?.let { sub ->
                    Text(
                        text = sub,
                        style = PlumTheme.typography.labelMedium,
                        color = PlumTheme.palette.accent,
                        fontWeight = FontWeight.Bold,
                    )
                }
                Text(
                    text = ui.title,
                    style = PlumTheme.typography.headlineSmall,
                    color = Color.White,
                    fontWeight = FontWeight.SemiBold,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                )
                val showStatus = ui.status != "Playing" && ui.error == null
                if (showStatus) {
                    Text(
                        text = ui.status,
                        style = PlumTheme.typography.bodySmall,
                        color = Color.White.copy(alpha = 0.55f),
                    )
                }
                ui.error?.takeIf { it.isNotBlank() }?.let { err ->
                    Text(
                        text = err,
                        style = PlumTheme.typography.bodySmall,
                        color = PlumTheme.palette.error,
                        fontWeight = FontWeight.Medium,
                        maxLines = 2,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
            }
        }

        // ── Bottom controls ──────────────────────────────────────────────────────
        AnimatedVisibility(
            visible = controlsVisible,
            enter = fadeIn(),
            exit = fadeOut(),
            modifier = Modifier.align(Alignment.BottomCenter),
        ) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 40.dp, vertical = 28.dp),
                verticalArrangement = Arrangement.spacedBy(16.dp),
            ) {
                // Seek bar row
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(14.dp),
                ) {
                    Text(
                        text = formatPlayerTime(ui.positionMs),
                        style = PlumTheme.typography.labelMedium,
                        color = Color.White.copy(alpha = 0.75f),
                    )
                    PlexSeekBar(
                        fraction = ui.progressFraction,
                        modifier = Modifier.weight(1f),
                    )
                    Text(
                        text = "-${formatPlayerTime(ui.remainingMs)}",
                        style = PlumTheme.typography.labelMedium,
                        color = Color.White.copy(alpha = 0.75f),
                    )
                }

                // Buttons row: utility-left | center controls | utility-right
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    // Left spacer (keeps center controls truly centered)
                    Spacer(modifier = Modifier.weight(1f))

                    // Center: playback controls
                    Row(
                        horizontalArrangement = Arrangement.spacedBy(6.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        PlayerControlButton(
                            icon = Icons.Filled.SkipPrevious,
                            contentDescription = "Previous",
                            onClick = { viewModel.previousEpisode() },
                            enabled = ui.canPrev,
                        )
                        PlayerControlButton(
                            icon = Icons.Filled.Replay10,
                            contentDescription = "Rewind 10 seconds",
                            onClick = { viewModel.rewind10() },
                        )
                        PlayerControlButton(
                            icon = if (ui.isPlaying) Icons.Filled.Pause else Icons.Filled.PlayArrow,
                            contentDescription = if (ui.isPlaying) "Pause" else "Play",
                            onClick = { viewModel.togglePlayPause() },
                            modifier = Modifier.focusRequester(playFocusRequester),
                            primary = true,
                        )
                        PlayerControlButton(
                            icon = Icons.Filled.Forward10,
                            contentDescription = "Forward 10 seconds",
                            onClick = { viewModel.forward10() },
                        )
                        PlayerControlButton(
                            icon = Icons.Filled.SkipNext,
                            contentDescription = "Next",
                            onClick = { viewModel.nextEpisode() },
                            enabled = ui.canNext,
                        )
                    }

                    // Right: utility buttons
                    Row(
                        modifier = Modifier.weight(1f),
                        horizontalArrangement = Arrangement.End,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Row(horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                            PlayerControlButton(
                                icon = Icons.AutoMirrored.Filled.VolumeUp,
                                contentDescription = audioContentDescription(ui.audioTrackLabel, ui.canCycleAudio),
                                onClick = { viewModel.cycleAudioTrack() },
                                enabled = ui.canCycleAudio,
                                utility = true,
                            )
                            PlayerControlButton(
                                icon = Icons.Filled.Subtitles,
                                contentDescription = subtitleContentDescription(ui.subtitleTrackLabel, ui.canCycleSubtitles),
                                onClick = { viewModel.cycleSubtitles() },
                                enabled = ui.canCycleSubtitles,
                                utility = true,
                            )
                            PlayerControlButton(
                                icon = Icons.AutoMirrored.Filled.ArrowBack,
                                contentDescription = "Back",
                                onClick = onClose,
                                ghost = true,
                                utility = true,
                            )
                        }
                    }
                }
            }
        }
    }
}

// ── Seek bar ─────────────────────────────────────────────────────────────────

@Composable
private fun PlexSeekBar(fraction: Float, modifier: Modifier = Modifier) {
    val accent = PlumTheme.palette.accent
    val f = fraction.coerceIn(0f, 1f)
    BoxWithConstraints(
        modifier = modifier.height(20.dp),
        contentAlignment = Alignment.CenterStart,
    ) {
        // Track background
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .height(4.dp)
                .clip(RoundedCornerShape(999.dp))
                .background(Color.White.copy(alpha = 0.18f)),
        )
        // Filled portion
        if (f > 0f) {
            Box(
                modifier = Modifier
                    .fillMaxWidth(fraction = f)
                    .height(4.dp)
                    .clip(RoundedCornerShape(999.dp))
                    .background(accent),
            )
        }
        // Thumb dot — offset to sit centered at the playhead position
        if (f > 0f) {
            val thumbRadius = 6.5.dp
            val thumbOffset = (maxWidth * f - thumbRadius).coerceAtLeast(0.dp)
            Box(
                modifier = Modifier
                    .padding(start = thumbOffset)
                    .size(13.dp)
                    .clip(CircleShape)
                    .background(Color.White),
            )
        }
    }
}

// ── Control button ────────────────────────────────────────────────────────────

@Composable
private fun PlayerControlButton(
    icon: ImageVector,
    contentDescription: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    primary: Boolean = false,
    ghost: Boolean = false,
    utility: Boolean = false,
) {
    val palette = PlumTheme.palette
    val buttonSize: Dp
    val iconSize: Dp
    val cornerRadius: Dp
    when {
        primary -> { buttonSize = 62.dp; iconSize = 32.dp; cornerRadius = 20.dp }
        utility -> { buttonSize = 42.dp; iconSize = 19.dp; cornerRadius = 14.dp }
        else    -> { buttonSize = 50.dp; iconSize = 24.dp; cornerRadius = 16.dp }
    }
    val shape = RoundedCornerShape(cornerRadius)
    val containerColor = when {
        ghost   -> Color.Transparent
        primary -> palette.accent
        else    -> Color.White.copy(alpha = 0.10f)
    }
    val focusedContainerColor = when {
        ghost   -> Color.White.copy(alpha = 0.12f)
        primary -> palette.accent
        else    -> Color.White.copy(alpha = 0.22f)
    }
    val contentColor = when {
        primary -> Color(0xFF1A1030)
        ghost   -> Color.White.copy(alpha = 0.70f)
        else    -> Color.White
    }

    Surface(
        onClick = onClick,
        enabled = enabled,
        modifier = modifier,
        shape = ClickableSurfaceDefaults.shape(shape = shape),
        colors = ClickableSurfaceDefaults.colors(
            containerColor = containerColor,
            contentColor = contentColor,
            focusedContainerColor = focusedContainerColor,
            focusedContentColor = if (primary) Color(0xFF1A1030) else Color.White,
            pressedContainerColor = focusedContainerColor,
            pressedContentColor = if (primary) Color(0xFF1A1030) else Color.White,
            disabledContainerColor = if (ghost) Color.Transparent else Color.White.copy(alpha = 0.05f),
            disabledContentColor = Color.White.copy(alpha = 0.25f),
        ),
        scale = ClickableSurfaceDefaults.scale(focusedScale = 1.06f),
        border = ClickableSurfaceDefaults.border(
            border = plumBorder(
                if (ghost) Color.Transparent else Color.White.copy(alpha = 0.12f),
                if (ghost) 0.dp else 1.dp,
                shape,
            ),
            focusedBorder = plumBorder(palette.accent.copy(alpha = 0.70f), 1.5.dp, shape),
            pressedBorder = plumBorder(palette.accent.copy(alpha = 0.70f), 1.5.dp, shape),
        ),
        glow = ClickableSurfaceDefaults.glow(focusedGlow = Glow(Color.Transparent, 0.dp)),
    ) {
        Box(
            modifier = Modifier.size(buttonSize),
            contentAlignment = Alignment.Center,
        ) {
            Icon(
                imageVector = icon,
                contentDescription = contentDescription,
                tint = LocalContentColor.current,
                modifier = Modifier.size(iconSize),
            )
        }
    }
}

// ── Helpers ───────────────────────────────────────────────────────────────────

private fun audioContentDescription(value: String?, enabled: Boolean): String =
    if (!enabled) "Audio unavailable"
    else if (!value.isNullOrBlank()) "Audio: ${value.trim()}"
    else "Change audio track"

private fun subtitleContentDescription(value: String?, enabled: Boolean): String =
    if (!enabled) "Subtitles unavailable"
    else if (!value.isNullOrBlank()) "Subtitles: ${value.trim()}"
    else "Change subtitles"

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
