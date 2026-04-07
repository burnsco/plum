package plum.tv.app

import android.text.format.DateFormat
import android.view.Gravity
import android.widget.FrameLayout
import androidx.annotation.OptIn
import android.view.KeyEvent as AndroidKeyEvent
import android.view.ViewGroup
import android.view.WindowManager
import androidx.activity.ComponentActivity
import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.focusGroup
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.focusable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.automirrored.filled.VolumeUp
import androidx.compose.material.icons.filled.AspectRatio
import androidx.compose.material.icons.filled.Forward10
import androidx.compose.material.icons.filled.Pause
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Replay10
import androidx.compose.material.icons.filled.SkipNext
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.filled.SkipPrevious
import androidx.compose.material.icons.filled.Subtitles
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.LocalContentColor
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.withFrameNanos
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.zIndex
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusProperties
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.focusTarget
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onKeyEvent
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.coerceAtLeast
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.core.view.updateLayoutParams
import androidx.hilt.lifecycle.viewmodel.compose.hiltViewModel
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import androidx.media3.common.util.UnstableApi
import androidx.media3.ui.AspectRatioFrameLayout
import androidx.media3.ui.CaptionStyleCompat
import androidx.media3.ui.PlayerView
import androidx.media3.ui.R as Media3UiR
import coil3.compose.AsyncImage
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.Glow
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import java.util.Date
import android.graphics.Color as AndroidGraphicsColor
import kotlinx.coroutines.delay
import plum.tv.core.data.SubtitleAppearance
import plum.tv.core.data.VideoAspectRatioMode
import plum.tv.core.data.SubtitlePosition
import plum.tv.core.data.SubtitleSize
import plum.tv.core.player.TrackPicker
import plum.tv.core.player.TrackPickerOption
import plum.tv.core.player.UpNextOverlayState
import plum.tv.core.ui.LocalServerBaseUrl
import plum.tv.core.ui.PlumImageSizes
import plum.tv.core.ui.PlumScrims
import plum.tv.core.ui.resolveArtworkUrl
import plum.tv.core.ui.PlumTheme
import plum.tv.core.ui.plumBorder

private const val CONTROLS_HIDE_DELAY_MS = 3_000L

@UnstableApi
@Composable
fun PlayerRoute(
    onClose: () -> Unit,
    viewModel: PlayerViewModel = hiltViewModel(),
) {
    val ui by viewModel.uiState.collectAsState()
    val wallClockMs by viewModel.wallClock.collectAsState()
    val trackPicker by viewModel.trackPicker.collectAsState()
    val upNext by viewModel.upNext.collectAsState()
    val subtitleAppearance by viewModel.subtitleAppearance.collectAsState()
    val subtitleStyleOverlayVisible by viewModel.subtitleStyleOverlayVisible.collectAsState()
    val videoAspectRatioMode by viewModel.videoAspectRatioMode.collectAsState()
    var aspectRatioOverlayVisible by remember { mutableStateOf(false) }
    val rootFocusRequester = remember { FocusRequester() }
    val playFocusRequester = remember { FocusRequester() }
    val seekBarFocusRequester = remember { FocusRequester() }
    val upNextPlayFocusRequester = remember { FocusRequester() }
    val subtitleStyleFocusRequester = remember { FocusRequester() }

    // Each time hideTimerKey changes, the LaunchedEffect restarts the hide countdown.
    var hideTimerKey by remember { mutableIntStateOf(0) }
    var controlsVisible by remember { mutableStateOf(true) }

    val context = LocalContext.current
    val lifecycleOwner = LocalLifecycleOwner.current
    val timeFormat = remember(context) { DateFormat.getTimeFormat(context) }

    DisposableEffect(lifecycleOwner, viewModel) {
        val observer = LifecycleEventObserver { _, event ->
            when (event) {
                Lifecycle.Event.ON_STOP -> viewModel.pauseWhenBackgrounded()
                Lifecycle.Event.ON_START -> viewModel.resumeWhenForegrounded()
                else -> Unit
            }
        }
        lifecycleOwner.lifecycle.addObserver(observer)
        onDispose { lifecycleOwner.lifecycle.removeObserver(observer) }
    }

    fun showControls() {
        controlsVisible = true
        hideTimerKey++
    }

    // Auto-hide controls after inactivity when playing (read StateFlow after delay so isPlaying is not stale).
    LaunchedEffect(hideTimerKey, trackPicker, subtitleStyleOverlayVisible, upNext, aspectRatioOverlayVisible) {
        delay(CONTROLS_HIDE_DELAY_MS)
        if (viewModel.uiState.value.isPlaying &&
            trackPicker == null &&
            !subtitleStyleOverlayVisible &&
            upNext == null &&
            !aspectRatioOverlayVisible
        ) {
            controlsVisible = false
        }
    }

    // Always show controls when paused or buffering
    LaunchedEffect(ui.isPlaying, ui.isBuffering) {
        if (!ui.isPlaying || ui.isBuffering) {
            showControls()
        }
    }

    // Surface the skip control when intro detection kicks in mid-playback (auto-hide would hide it).
    LaunchedEffect(ui.showSkipIntro) {
        if (ui.showSkipIntro) {
            showControls()
        }
    }

    LaunchedEffect(ui.showSkipCredits) {
        if (ui.showSkipCredits) {
            showControls()
        }
    }

    LaunchedEffect(upNext) {
        if (upNext != null) {
            showControls()
            upNextPlayFocusRequester.requestFocus()
        }
    }

    BackHandler(trackPicker != null && !aspectRatioOverlayVisible) {
        viewModel.dismissTrackPicker()
    }

    BackHandler(trackPicker == null && upNext != null && !aspectRatioOverlayVisible) {
        viewModel.dismissUpNext()
    }

    BackHandler(trackPicker == null && subtitleStyleOverlayVisible && !aspectRatioOverlayVisible) {
        viewModel.dismissSubtitleStyleSettings()
    }

    BackHandler(
        trackPicker == null &&
            upNext == null &&
            !subtitleStyleOverlayVisible &&
            !aspectRatioOverlayVisible,
    ) {
        if (controlsVisible) {
            onClose()
        } else {
            showControls()
        }
    }

    BackHandler(aspectRatioOverlayVisible) {
        aspectRatioOverlayVisible = false
    }

    LaunchedEffect(Unit) {
        rootFocusRequester.requestFocus()
    }

    // Do not move focus to play/root while a modal overlay owns focus — otherwise D-pad reaches controls under the scrim.
    LaunchedEffect(controlsVisible, trackPicker, subtitleStyleOverlayVisible, upNext, aspectRatioOverlayVisible) {
        if (trackPicker != null ||
            subtitleStyleOverlayVisible ||
            upNext != null ||
            aspectRatioOverlayVisible
        ) {
            return@LaunchedEffect
        }
        if (controlsVisible) {
            playFocusRequester.requestFocus()
        } else {
            rootFocusRequester.requestFocus()
        }
    }

    LaunchedEffect(trackPicker, subtitleStyleOverlayVisible, upNext) {
        if (trackPicker != null || subtitleStyleOverlayVisible || upNext != null) {
            aspectRatioOverlayVisible = false
        }
    }

    // Track picker rows hold focus; when the overlay is removed, restore focus or D-pad is stuck.
    var hadTrackPicker by remember { mutableStateOf(false) }
    LaunchedEffect(trackPicker) {
        if (hadTrackPicker && trackPicker == null) {
            withFrameNanos { }
            if (controlsVisible) {
                playFocusRequester.requestFocus()
            } else {
                rootFocusRequester.requestFocus()
            }
        }
        hadTrackPicker = trackPicker != null
    }

    var hadSubtitleStyleOverlay by remember { mutableStateOf(false) }
    LaunchedEffect(subtitleStyleOverlayVisible) {
        if (subtitleStyleOverlayVisible) {
            withFrameNanos { }
            subtitleStyleFocusRequester.requestFocus()
        } else if (hadSubtitleStyleOverlay) {
            withFrameNanos { }
            if (controlsVisible) {
                playFocusRequester.requestFocus()
            } else {
                rootFocusRequester.requestFocus()
            }
        }
        hadSubtitleStyleOverlay = subtitleStyleOverlayVisible
    }

    var hadAspectRatioOverlay by remember { mutableStateOf(false) }
    LaunchedEffect(aspectRatioOverlayVisible) {
        if (aspectRatioOverlayVisible) {
            withFrameNanos { }
        } else if (hadAspectRatioOverlay) {
            withFrameNanos { }
            if (controlsVisible) {
                playFocusRequester.requestFocus()
            } else {
                rootFocusRequester.requestFocus()
            }
        }
        hadAspectRatioOverlay = aspectRatioOverlayVisible
    }

    val keepAwake =
        ui.isPlaying ||
            ui.isBuffering ||
            upNext != null ||
            trackPicker != null ||
            subtitleStyleOverlayVisible ||
            aspectRatioOverlayVisible
    DisposableEffect(keepAwake) {
        val window = (context as ComponentActivity).window
        if (keepAwake) {
            window.addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
        } else {
            window.clearFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
        }
        onDispose {
            window.clearFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
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
                if (upNext != null) {
                    if (event.nativeKeyEvent.action != AndroidKeyEvent.ACTION_UP) return@onPreviewKeyEvent false
                    return@onPreviewKeyEvent when (event.nativeKeyEvent.keyCode) {
                        AndroidKeyEvent.KEYCODE_DPAD_CENTER,
                        AndroidKeyEvent.KEYCODE_ENTER -> {
                            viewModel.playUpNextNow()
                            true
                        }
                        else -> false
                    }
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
                    applyPlumSubtitleAppearance(viewModel.subtitleAppearance.value)
                    applyVideoAspectToPlayerView(this, viewModel.videoAspectRatioMode.value)
                }
            },
            update = { view ->
                if (view.player !== viewModel.player) {
                    view.player = viewModel.player
                }
                view.isFocusable = false
                view.isFocusableInTouchMode = false
                view.descendantFocusability = ViewGroup.FOCUS_BLOCK_DESCENDANTS
                view.applyPlumSubtitleAppearance(subtitleAppearance)
                applyVideoAspectToPlayerView(view, videoAspectRatioMode)
            },
        )

        // ── Gradient overlay (only when controls are visible; no fade — lighter GPU/compose work) ──
        if (controlsVisible) {
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .background(PlumScrims.playerControls),
            )
        }

        // ── Buffering spinner (always visible regardless of controls state) ───────
        if (ui.isBuffering) {
            Box(
                modifier = Modifier.fillMaxSize(),
                contentAlignment = Alignment.Center,
            ) {
                CircularProgressIndicator(
                    modifier = Modifier.size(48.dp),
                    color = PlumTheme.palette.accent.copy(alpha = 0.9f),
                    trackColor = Color.White.copy(alpha = 0.10f),
                    strokeWidth = 3.dp,
                )
            }
        }

        // ── Top metadata bar ─────────────────────────────────────────────────────
        if (controlsVisible) {
            Column(
                modifier = Modifier
                    .align(Alignment.TopStart)
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

        // ── Local time (top right, with controls) ────────────────────────────────
        if (controlsVisible) {
            Text(
                text = timeFormat.format(Date(wallClockMs)),
                style = PlumTheme.typography.titleMedium,
                color = Color.White.copy(alpha = 0.9f),
                fontWeight = FontWeight.Medium,
                modifier =
                    Modifier
                        .align(Alignment.TopEnd)
                        .padding(horizontal = 40.dp, vertical = 28.dp),
            )
        }

        // ── Bottom controls ──────────────────────────────────────────────────────
        if (controlsVisible) {
            Column(
                modifier = Modifier
                    .align(Alignment.BottomCenter)
                    .fillMaxWidth()
                    .padding(horizontal = 40.dp)
                    .padding(bottom = 12.dp),
                verticalArrangement = Arrangement.spacedBy(16.dp),
            ) {
                TimelineSeekRow(
                    positionMs = ui.positionMs,
                    remainingMs = ui.remainingMs,
                    durationMs = ui.durationMs,
                    progressFraction = ui.progressFraction,
                    seekBarFocusRequester = seekBarFocusRequester,
                    playFocusRequester = playFocusRequester,
                    onSeekStep = { viewModel.seekTimelineBySteps(it) },
                )

                // Buttons row: display (left) | transport (center) | tracks & exit (right)
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Row(
                        modifier = Modifier.weight(1f),
                        horizontalArrangement = Arrangement.Start,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        PlayerControlButton(
                            icon = Icons.Filled.AspectRatio,
                            contentDescription = "Aspect ratio",
                            onClick = {
                                viewModel.dismissTrackPicker()
                                aspectRatioOverlayVisible = true
                                showControls()
                            },
                            utility = true,
                            modifier = controlUpToSeekBar(ui.durationMs, seekBarFocusRequester),
                        )
                    }

                    Row(
                        horizontalArrangement = Arrangement.spacedBy(16.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        if (ui.canPrev) {
                            PlayerControlButton(
                                icon = Icons.Filled.SkipPrevious,
                                contentDescription = "Previous",
                                onClick = { viewModel.previousEpisode() },
                                modifier = controlUpToSeekBar(ui.durationMs, seekBarFocusRequester),
                            )
                        }
                        PlayerControlButton(
                            icon = Icons.Filled.Replay10,
                            contentDescription = "Rewind 10 seconds",
                            onClick = { viewModel.rewind10() },
                            modifier = controlUpToSeekBar(ui.durationMs, seekBarFocusRequester),
                        )
                        PlayerControlButton(
                            icon = if (ui.isPlaying) Icons.Filled.Pause else Icons.Filled.PlayArrow,
                            contentDescription = if (ui.isPlaying) "Pause" else "Play",
                            onClick = { viewModel.togglePlayPause() },
                            modifier =
                                controlUpToSeekBar(ui.durationMs, seekBarFocusRequester)
                                    .focusRequester(playFocusRequester),
                            primary = true,
                        )
                        PlayerControlButton(
                            icon = Icons.Filled.Forward10,
                            contentDescription = "Forward 10 seconds",
                            onClick = { viewModel.forward10() },
                            modifier = controlUpToSeekBar(ui.durationMs, seekBarFocusRequester),
                        )
                        if (ui.showSkipIntro) {
                            SkipIntroControl(
                                label = "Skip intro",
                                contentDescription = "Skip intro",
                                onClick = {
                                    viewModel.skipIntro()
                                    showControls()
                                },
                                modifier = controlUpToSeekBar(ui.durationMs, seekBarFocusRequester),
                            )
                        }
                        if (ui.showSkipCredits) {
                            SkipIntroControl(
                                label = "Skip credits",
                                contentDescription = "Skip credits",
                                onClick = {
                                    viewModel.skipCredits()
                                    showControls()
                                },
                                modifier = controlUpToSeekBar(ui.durationMs, seekBarFocusRequester),
                            )
                        }
                        if (ui.canNext) {
                            PlayerControlButton(
                                icon = Icons.Filled.SkipNext,
                                contentDescription = "Next",
                                onClick = { viewModel.nextEpisode() },
                                modifier = controlUpToSeekBar(ui.durationMs, seekBarFocusRequester),
                            )
                        }
                    }

                    Row(
                        modifier = Modifier.weight(1f),
                        horizontalArrangement = Arrangement.End,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                            if (ui.canCycleAudio) {
                                PlayerControlButton(
                                    icon = Icons.AutoMirrored.Filled.VolumeUp,
                                    contentDescription = audioContentDescription(ui.audioTrackLabel),
                                    onClick = {
                                        viewModel.openAudioPicker()
                                        showControls()
                                    },
                                    utility = true,
                                    modifier = controlUpToSeekBar(ui.durationMs, seekBarFocusRequester),
                                )
                            }
                            if (ui.canCycleSubtitles) {
                                PlayerControlButton(
                                    icon = Icons.Filled.Subtitles,
                                    contentDescription = subtitleContentDescription(ui.subtitleTrackLabel),
                                    onClick = {
                                        viewModel.openSubtitlePicker()
                                        showControls()
                                    },
                                    utility = true,
                                    modifier = controlUpToSeekBar(ui.durationMs, seekBarFocusRequester),
                                )
                            }
                            PlayerControlButton(
                                icon = Icons.Filled.Settings,
                                contentDescription = "Subtitle appearance: size, color, position",
                                onClick = {
                                    viewModel.openSubtitleStyleSettings()
                                    showControls()
                                },
                                utility = true,
                                modifier = controlUpToSeekBar(ui.durationMs, seekBarFocusRequester),
                            )
                            PlayerControlButton(
                                icon = Icons.AutoMirrored.Filled.ArrowBack,
                                contentDescription = "Back",
                                onClick = onClose,
                                ghost = true,
                                utility = true,
                                modifier = controlUpToSeekBar(ui.durationMs, seekBarFocusRequester),
                            )
                        }
                    }
                }
            }
        }

        upNext?.let { state ->
            UpNextInterstitial(
                modifier = Modifier.zIndex(4f),
                state = state,
                serverBase = LocalServerBaseUrl.current,
                onConfirm = { viewModel.playUpNextNow() },
                playNowFocusRequester = upNextPlayFocusRequester,
            )
        }

        trackPicker?.let { picker ->
            TrackPickerOverlay(
                modifier = Modifier.zIndex(5f),
                picker = picker,
                onSelect = { viewModel.selectTrackPickerOption(it) },
            )
        }

        if (aspectRatioOverlayVisible) {
            VideoAspectRatioPickerOverlay(
                modifier = Modifier.zIndex(6f),
                current = videoAspectRatioMode,
                detectedLabel = ui.detectedVideoAspectLabel,
                onSelect = { mode ->
                    viewModel.setVideoAspectRatioMode(mode)
                    aspectRatioOverlayVisible = false
                },
            )
        }

        if (subtitleStyleOverlayVisible) {
            SubtitleStyleOverlay(
                modifier = Modifier.zIndex(5f),
                appearance = subtitleAppearance,
                onAppearanceChange = { viewModel.setSubtitleAppearance(it) },
                onDismiss = { viewModel.dismissSubtitleStyleSettings() },
                firstFocusRequester = subtitleStyleFocusRequester,
            )
        }
    }
}

@Composable
private fun UpNextInterstitial(
    state: UpNextOverlayState,
    serverBase: String,
    onConfirm: () -> Unit,
    playNowFocusRequester: FocusRequester,
    modifier: Modifier = Modifier,
) {
    val palette = PlumTheme.palette
    val heroUrl =
        resolveArtworkUrl(serverBase, state.backdropUrl, state.backdropPath, PlumImageSizes.BACKDROP_HERO)
            ?: resolveArtworkUrl(serverBase, state.showPosterUrl, state.showPosterPath, PlumImageSizes.BACKDROP_HERO)
    val shape = RoundedCornerShape(12.dp)
    Box(
        modifier =
            modifier
                .fillMaxSize()
                .background(Color.Black),
    ) {
        if (!heroUrl.isNullOrBlank()) {
            AsyncImage(
                model = heroUrl,
                contentDescription = null,
                modifier = Modifier.fillMaxSize(),
                contentScale = ContentScale.Crop,
            )
        }
        Box(
            modifier =
                Modifier
                    .fillMaxSize()
                    .background(
                        Brush.verticalGradient(
                            0f to Color.Black.copy(alpha = 0.35f),
                            0.45f to Color.Black.copy(alpha = 0.78f),
                            1f to Color.Black.copy(alpha = 0.94f),
                        ),
                    ),
        )
        Column(
            modifier =
                Modifier
                    .align(Alignment.Center)
                    .padding(horizontal = 48.dp)
                    .widthIn(max = 520.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Text(
                text = "UP NEXT",
                style = PlumTheme.typography.labelMedium,
                color = palette.accent.copy(alpha = 0.8f),
                fontWeight = FontWeight.Bold,
            )
            Text(
                text = state.title,
                style = PlumTheme.typography.headlineSmall,
                color = Color.White,
                fontWeight = FontWeight.Bold,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )
            state.subtitle?.takeIf { it.isNotBlank() }?.let { sub ->
                Text(
                    text = sub,
                    style = PlumTheme.typography.titleSmall,
                    color = palette.accent,
                    fontWeight = FontWeight.SemiBold,
                )
            }
            Text(
                text = "${state.secondsRemaining}",
                style = PlumTheme.typography.displaySmall,
                color = Color.White,
                fontWeight = FontWeight.Bold,
            )
            Text(
                text = "seconds",
                style = PlumTheme.typography.labelMedium,
                color = Color.White.copy(alpha = 0.55f),
            )
            Surface(
                onClick = onConfirm,
                modifier =
                    Modifier
                        .padding(top = 8.dp)
                        .focusRequester(playNowFocusRequester)
                        .focusable(),
                shape = ClickableSurfaceDefaults.shape(shape = shape),
                colors =
                    ClickableSurfaceDefaults.colors(
                        containerColor = palette.accent.copy(alpha = 0.92f),
                        contentColor = Color.Black,
                        focusedContainerColor = palette.accent,
                        focusedContentColor = Color.Black,
                        pressedContainerColor = palette.accent,
                        pressedContentColor = Color.Black,
                    ),
                scale = ClickableSurfaceDefaults.scale(focusedScale = 1f),
                border =
                    ClickableSurfaceDefaults.border(
                        border = plumBorder(Color.White.copy(alpha = 0.2f), 1.dp, shape),
                        focusedBorder = plumBorder(Color.White.copy(alpha = 0.55f), 2.dp, shape),
                        pressedBorder = plumBorder(Color.White.copy(alpha = 0.55f), 2.dp, shape),
                    ),
                glow = ClickableSurfaceDefaults.glow(focusedGlow = Glow(Color.Transparent, 0.dp)),
            ) {
                Box(
                    modifier =
                        Modifier
                            .fillMaxWidth()
                            .padding(vertical = 12.dp, horizontal = 20.dp),
                    contentAlignment = Alignment.Center,
                ) {
                    Text(
                        text = "OK · Play now",
                        style = PlumTheme.typography.titleSmall,
                        fontWeight = FontWeight.Bold,
                    )
                }
            }
        }
    }
}

private fun applyVideoAspectToPlayerView(
    playerView: PlayerView,
    mode: VideoAspectRatioMode,
) {
    val contentFrame =
        playerView.findViewById<AspectRatioFrameLayout>(Media3UiR.id.exo_content_frame)
            ?: return
    when (mode) {
        VideoAspectRatioMode.AUTO -> {
            contentFrame.resizeMode = AspectRatioFrameLayout.RESIZE_MODE_FIT
            contentFrame.setAspectRatio(0f)
        }
        VideoAspectRatioMode.ZOOM -> {
            contentFrame.resizeMode = AspectRatioFrameLayout.RESIZE_MODE_ZOOM
            contentFrame.setAspectRatio(0f)
        }
        VideoAspectRatioMode.STRETCH -> {
            contentFrame.resizeMode = AspectRatioFrameLayout.RESIZE_MODE_FILL
            contentFrame.setAspectRatio(0f)
        }
        VideoAspectRatioMode.RATIO_16_9 -> {
            contentFrame.resizeMode = AspectRatioFrameLayout.RESIZE_MODE_FIT
            contentFrame.setAspectRatio(16f / 9f)
        }
        VideoAspectRatioMode.RATIO_4_3 -> {
            contentFrame.resizeMode = AspectRatioFrameLayout.RESIZE_MODE_FIT
            contentFrame.setAspectRatio(4f / 3f)
        }
        VideoAspectRatioMode.RATIO_21_9 -> {
            contentFrame.resizeMode = AspectRatioFrameLayout.RESIZE_MODE_FIT
            contentFrame.setAspectRatio(21f / 9f)
        }
    }
}

@Composable
private fun VideoAspectRatioPickerOverlay(
    current: VideoAspectRatioMode,
    detectedLabel: String?,
    onSelect: (VideoAspectRatioMode) -> Unit,
    modifier: Modifier = Modifier,
) {
    val options =
        remember(current, detectedLabel) {
            VideoAspectRatioMode.entries.map { mode ->
                val label =
                    when (mode) {
                        VideoAspectRatioMode.AUTO -> "Auto (stream)"
                        VideoAspectRatioMode.ZOOM -> "Zoom (crop to fill)"
                        VideoAspectRatioMode.STRETCH -> "Stretch to screen"
                        VideoAspectRatioMode.RATIO_16_9 -> "16:9 frame"
                        VideoAspectRatioMode.RATIO_4_3 -> "4:3 frame"
                        VideoAspectRatioMode.RATIO_21_9 -> "21:9 frame"
                    }
                val detail =
                    if (mode == VideoAspectRatioMode.AUTO && !detectedLabel.isNullOrBlank()) {
                        "Detected $detectedLabel"
                    } else {
                        null
                    }
                TrackPickerOption(
                    id = mode.storageValue,
                    label = label,
                    selected = mode == current,
                    detail = detail,
                )
            }
        }
    val focusRequesters = remember(options) { List(options.size) { FocusRequester() } }

    LaunchedEffect(options) {
        val idx = options.indexOfFirst { it.selected }.let { if (it < 0) 0 else it }
        focusRequesters.getOrNull(idx)?.requestFocus()
    }

    Box(
        modifier =
            modifier
                .fillMaxSize()
                .focusGroup()
                .background(Color.Black.copy(alpha = 0.75f)),
        contentAlignment = Alignment.Center,
    ) {
        val panelShape = RoundedCornerShape(16.dp)
        Column(
            modifier =
                Modifier
                    .widthIn(max = 420.dp)
                    .fillMaxWidth(0.82f)
                    .fillMaxHeight(0.82f)
                    .clip(panelShape)
                    .background(PlumTheme.palette.panel)
                    .padding(horizontal = 20.dp, vertical = 16.dp),
        ) {
            Text(
                text = "Aspect ratio",
                style = PlumTheme.typography.titleMedium,
                color = Color.White,
                fontWeight = FontWeight.SemiBold,
                modifier = Modifier.padding(bottom = 12.dp),
            )
            Column(
                verticalArrangement = Arrangement.spacedBy(8.dp),
                modifier =
                    Modifier
                        .weight(1f, fill = true)
                        .fillMaxWidth()
                        .verticalScroll(rememberScrollState()),
            ) {
                options.forEachIndexed { index, opt ->
                    TrackPickerRow(
                        option = opt,
                        modifier = Modifier.focusRequester(focusRequesters[index]),
                        onActivate = { onSelect(VideoAspectRatioMode.fromStorage(opt.id)) },
                    )
                }
            }
        }
    }
}

@Composable
private fun TrackPickerOverlay(
    picker: TrackPicker,
    onSelect: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    val options = picker.options
    val focusRequesters = remember(picker) { List(options.size) { FocusRequester() } }

    LaunchedEffect(picker) {
        val idx = options.indexOfFirst { it.selected }.let { if (it < 0) 0 else it }
        focusRequesters.getOrNull(idx)?.requestFocus()
    }

    Box(
        modifier =
            modifier
                .fillMaxSize()
                .focusGroup()
                .background(Color.Black.copy(alpha = 0.75f)),
        contentAlignment = Alignment.Center,
    ) {
        val panelShape = RoundedCornerShape(16.dp)
        Column(
            modifier =
                Modifier
                    .widthIn(max = 420.dp)
                    .fillMaxWidth(0.82f)
                    .fillMaxHeight(0.82f)
                    .clip(panelShape)
                    .background(PlumTheme.palette.panel)
                    .padding(horizontal = 20.dp, vertical = 16.dp),
        ) {
            Text(
                text = picker.title,
                style = PlumTheme.typography.titleMedium,
                color = Color.White,
                fontWeight = FontWeight.SemiBold,
                modifier = Modifier.padding(bottom = 12.dp),
            )
            Column(
                verticalArrangement = Arrangement.spacedBy(8.dp),
                modifier =
                    Modifier
                        .weight(1f, fill = true)
                        .fillMaxWidth()
                        .verticalScroll(rememberScrollState()),
            ) {
                options.forEachIndexed { index, opt ->
                    TrackPickerRow(
                        option = opt,
                        modifier = Modifier.focusRequester(focusRequesters[index]),
                        onActivate = { onSelect(opt.id) },
                    )
                }
            }
        }
    }
}

@Composable
private fun TrackPickerRow(
    option: TrackPickerOption,
    onActivate: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val palette = PlumTheme.palette
    val shape = RoundedCornerShape(10.dp)
    Surface(
        onClick = onActivate,
        modifier =
            modifier
                .fillMaxWidth()
                .onPreviewKeyEvent { event ->
                    if (event.nativeKeyEvent.action != AndroidKeyEvent.ACTION_UP) {
                        return@onPreviewKeyEvent false
                    }
                    when (event.nativeKeyEvent.keyCode) {
                        AndroidKeyEvent.KEYCODE_DPAD_CENTER,
                        AndroidKeyEvent.KEYCODE_ENTER,
                        AndroidKeyEvent.KEYCODE_NUMPAD_ENTER -> {
                            onActivate()
                            true
                        }
                        else -> false
                    }
                },
        shape = ClickableSurfaceDefaults.shape(shape = shape),
        colors =
            ClickableSurfaceDefaults.colors(
                containerColor = Color.White.copy(alpha = 0.06f),
                contentColor = Color.White,
                focusedContainerColor = Color.White.copy(alpha = 0.12f),
                focusedContentColor = Color.White,
                pressedContainerColor = Color.White.copy(alpha = 0.12f),
                pressedContentColor = Color.White,
            ),
        scale = ClickableSurfaceDefaults.scale(focusedScale = 1f),
        border =
            ClickableSurfaceDefaults.border(
                border = plumBorder(Color.White.copy(alpha = 0.08f), 1.dp, shape),
                focusedBorder = plumBorder(palette.accent.copy(alpha = 0.65f), 1.dp, shape),
                pressedBorder = plumBorder(palette.accent.copy(alpha = 0.65f), 1.dp, shape),
            ),
        glow = ClickableSurfaceDefaults.glow(focusedGlow = Glow(Color.Transparent, 0.dp)),
    ) {
        Row(
            modifier =
                Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 14.dp, vertical = 12.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column(
                modifier = Modifier.weight(1f),
                verticalArrangement = Arrangement.spacedBy(3.dp),
            ) {
                Text(
                    text = option.label,
                    style = PlumTheme.typography.bodyMedium,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                )
                option.detail?.takeIf { it.isNotBlank() }?.let { detail ->
                    Text(
                        text = detail,
                        style = PlumTheme.typography.bodySmall,
                        color = palette.muted,
                        maxLines = 3,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
            }
            if (option.selected) {
                Text(
                    text = "●",
                    style = PlumTheme.typography.labelSmall,
                    color = palette.accent,
                    modifier = Modifier.padding(start = 10.dp),
                )
            }
        }
    }
}

private data class SubtitleColorPreset(val label: String, val hex: String)

private val subtitleColorPresets =
    listOf(
        SubtitleColorPreset("White", "#ffffff"),
        SubtitleColorPreset("Yellow", "#ffff00"),
        SubtitleColorPreset("Lime", "#7cfc00"),
        SubtitleColorPreset("Cyan", "#00ffff"),
        SubtitleColorPreset("Pink", "#ffb6c1"),
        SubtitleColorPreset("Orange", "#ffab40"),
    )

private fun subtitleColorsEqual(stored: String, presetHex: String): Boolean {
    fun norm(s: String) = s.trim().lowercase().removePrefix("#")
    return norm(stored) == norm(presetHex)
}

@Composable
private fun SubtitleStyleOverlay(
    appearance: SubtitleAppearance,
    onAppearanceChange: (SubtitleAppearance) -> Unit,
    onDismiss: () -> Unit,
    firstFocusRequester: FocusRequester,
    modifier: Modifier = Modifier,
) {
    val palette = PlumTheme.palette
    val panelShape = RoundedCornerShape(16.dp)
    Box(
        modifier =
            modifier
                .fillMaxSize()
                .focusGroup()
                .background(Color.Black.copy(alpha = 0.75f)),
        contentAlignment = Alignment.Center,
    ) {
        Column(
            modifier =
                Modifier
                    .widthIn(max = 440.dp)
                    .fillMaxWidth(0.85f)
                    .fillMaxHeight(0.85f)
                    .clip(panelShape)
                    .background(palette.panel)
                    .padding(horizontal = 20.dp, vertical = 16.dp),
        ) {
            Text(
                text = "Subtitle appearance",
                style = PlumTheme.typography.titleMedium,
                color = Color.White,
                fontWeight = FontWeight.SemiBold,
                modifier = Modifier.padding(bottom = 12.dp),
            )
            Column(
                modifier =
                    Modifier
                        .weight(1f)
                        .fillMaxWidth()
                        .verticalScroll(rememberScrollState()),
                verticalArrangement = Arrangement.spacedBy(16.dp),
            ) {
                Text(
                    text = "Size",
                    style = PlumTheme.typography.labelMedium,
                    color = Color.White.copy(alpha = 0.72f),
                )
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    SubtitleSize.entries.forEachIndexed { index, size ->
                        val label =
                            when (size) {
                                SubtitleSize.SMALL -> "Small"
                                SubtitleSize.MEDIUM -> "Medium"
                                SubtitleSize.LARGE -> "Large"
                            }
                        SubtitleChoiceChip(
                            text = label,
                            selected = appearance.size == size,
                            onClick = { onAppearanceChange(appearance.copy(size = size)) },
                            modifier =
                                if (index == 0) {
                                    Modifier
                                        .weight(1f)
                                        .focusRequester(firstFocusRequester)
                                } else {
                                    Modifier.weight(1f)
                                },
                        )
                    }
                }
                Text(
                    text = "Position",
                    style = PlumTheme.typography.labelMedium,
                    color = Color.White.copy(alpha = 0.72f),
                )
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    listOf(SubtitlePosition.BOTTOM, SubtitlePosition.TOP).forEach { position ->
                        val label =
                            when (position) {
                                SubtitlePosition.BOTTOM -> "Bottom"
                                SubtitlePosition.TOP -> "Top"
                            }
                        SubtitleChoiceChip(
                            text = label,
                            selected = appearance.position == position,
                            onClick = { onAppearanceChange(appearance.copy(position = position)) },
                            modifier = Modifier.weight(1f),
                        )
                    }
                }
                Text(
                    text = "Color",
                    style = PlumTheme.typography.labelMedium,
                    color = Color.White.copy(alpha = 0.72f),
                )
                Row(
                    modifier =
                        Modifier
                            .fillMaxWidth()
                            .horizontalScroll(rememberScrollState()),
                    horizontalArrangement = Arrangement.spacedBy(10.dp),
                ) {
                    subtitleColorPresets.forEach { preset ->
                        SubtitleColorDot(
                            preset = preset,
                            selected = subtitleColorsEqual(appearance.colorHex, preset.hex),
                            onClick = {
                                onAppearanceChange(appearance.copy(colorHex = preset.hex))
                            },
                        )
                    }
                }
            }
            Spacer(modifier = Modifier.height(10.dp))
            Surface(
                onClick = onDismiss,
                modifier =
                    Modifier
                        .fillMaxWidth()
                        .onPreviewKeyEvent { event ->
                            if (event.nativeKeyEvent.action != AndroidKeyEvent.ACTION_UP) {
                                return@onPreviewKeyEvent false
                            }
                            when (event.nativeKeyEvent.keyCode) {
                                AndroidKeyEvent.KEYCODE_DPAD_CENTER,
                                AndroidKeyEvent.KEYCODE_ENTER,
                                AndroidKeyEvent.KEYCODE_NUMPAD_ENTER -> {
                                    onDismiss()
                                    true
                                }
                                else -> false
                            }
                        },
                shape = ClickableSurfaceDefaults.shape(shape = RoundedCornerShape(10.dp)),
                colors =
                    ClickableSurfaceDefaults.colors(
                        containerColor = Color.White.copy(alpha = 0.08f),
                        contentColor = Color.White.copy(alpha = 0.85f),
                        focusedContainerColor = Color.White.copy(alpha = 0.16f),
                        focusedContentColor = Color.White,
                        pressedContainerColor = Color.White.copy(alpha = 0.16f),
                        pressedContentColor = Color.White,
                    ),
                scale = ClickableSurfaceDefaults.scale(focusedScale = 1f),
                border =
                    ClickableSurfaceDefaults.border(
                        border = plumBorder(Color.White.copy(alpha = 0.1f), 1.dp, RoundedCornerShape(10.dp)),
                        focusedBorder = plumBorder(palette.accent.copy(alpha = 0.6f), 1.dp, RoundedCornerShape(10.dp)),
                        pressedBorder = plumBorder(palette.accent.copy(alpha = 0.6f), 1.dp, RoundedCornerShape(10.dp)),
                    ),
                glow = ClickableSurfaceDefaults.glow(focusedGlow = Glow(Color.Transparent, 0.dp)),
            ) {
                Box(
                    modifier =
                        Modifier
                            .fillMaxWidth()
                            .padding(vertical = 9.dp, horizontal = 12.dp),
                    contentAlignment = Alignment.Center,
                ) {
                    Text(
                        text = "Done",
                        style = PlumTheme.typography.labelMedium,
                    )
                }
            }
        }
    }
}

@Composable
private fun SubtitleChoiceChip(
    text: String,
    selected: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val palette = PlumTheme.palette
    val shape = RoundedCornerShape(10.dp)
    Surface(
        onClick = onClick,
        modifier = modifier,
        shape = ClickableSurfaceDefaults.shape(shape = shape),
        colors =
            ClickableSurfaceDefaults.colors(
                containerColor =
                    if (selected) {
                        Color.White.copy(alpha = 0.14f)
                    } else {
                        Color.White.copy(alpha = 0.06f)
                    },
                contentColor = Color.White,
                focusedContainerColor = Color.White.copy(alpha = 0.18f),
                focusedContentColor = Color.White,
                pressedContainerColor = Color.White.copy(alpha = 0.18f),
                pressedContentColor = Color.White,
            ),
        scale = ClickableSurfaceDefaults.scale(focusedScale = 1f),
        border =
            ClickableSurfaceDefaults.border(
                border =
                    plumBorder(
                        if (selected) {
                            palette.accent.copy(alpha = 0.75f)
                        } else {
                            Color.White.copy(alpha = 0.1f)
                        },
                        if (selected) {
                            2.dp
                        } else {
                            1.dp
                        },
                        shape,
                    ),
                focusedBorder = plumBorder(palette.accent.copy(alpha = 0.85f), 2.dp, shape),
                pressedBorder = plumBorder(palette.accent.copy(alpha = 0.85f), 2.dp, shape),
            ),
        glow = ClickableSurfaceDefaults.glow(focusedGlow = Glow(Color.Transparent, 0.dp)),
    ) {
        Box(
            modifier =
                Modifier
                    .fillMaxWidth()
                    .padding(vertical = 10.dp, horizontal = 6.dp),
            contentAlignment = Alignment.Center,
        ) {
            Text(
                text = text,
                style = PlumTheme.typography.labelLarge,
                fontWeight = if (selected) FontWeight.Bold else FontWeight.Medium,
            )
        }
    }
}

@Composable
private fun SubtitleColorDot(
    preset: SubtitleColorPreset,
    selected: Boolean,
    onClick: () -> Unit,
) {
    val palette = PlumTheme.palette
    val fill =
        runCatching { Color(AndroidGraphicsColor.parseColor(preset.hex)) }
            .getOrElse { Color.White }
    val shape = CircleShape
    val edge =
        if (preset.hex.equals("#ffffff", ignoreCase = true)) {
            Color.Black.copy(alpha = 0.4f)
        } else {
            Color.White.copy(alpha = 0.22f)
        }
    Surface(
        onClick = onClick,
        modifier =
            Modifier
                .size(44.dp)
                .semantics { contentDescription = "Subtitle color ${preset.label}" },
        shape = ClickableSurfaceDefaults.shape(shape = shape),
        colors =
            ClickableSurfaceDefaults.colors(
                containerColor = fill,
                contentColor = Color.White,
                focusedContainerColor = fill,
                focusedContentColor = Color.White,
                pressedContainerColor = fill,
                pressedContentColor = Color.White,
            ),
        scale = ClickableSurfaceDefaults.scale(focusedScale = 1f),
        border =
            ClickableSurfaceDefaults.border(
                border =
                    plumBorder(
                        if (selected) {
                            palette.accent
                        } else {
                            edge
                        },
                        if (selected) {
                            2.5.dp
                        } else {
                            1.dp
                        },
                        shape,
                    ),
                focusedBorder = plumBorder(palette.accent.copy(alpha = 0.95f), 2.5.dp, shape),
                pressedBorder = plumBorder(palette.accent.copy(alpha = 0.95f), 2.5.dp, shape),
            ),
        glow = ClickableSurfaceDefaults.glow(focusedGlow = Glow(Color.Transparent, 0.dp)),
    ) {
        Box(modifier = Modifier.fillMaxSize())
    }
}

// ── Seek bar ─────────────────────────────────────────────────────────────────

private fun controlUpToSeekBar(durationMs: Long, seekBarFocusRequester: FocusRequester): Modifier =
    if (durationMs > 0) {
        Modifier.focusProperties { up = seekBarFocusRequester }
    } else {
        Modifier
    }

@Composable
private fun TimelineSeekRow(
    positionMs: Long,
    remainingMs: Long,
    durationMs: Long,
    progressFraction: Float,
    seekBarFocusRequester: FocusRequester,
    playFocusRequester: FocusRequester,
    onSeekStep: (Int) -> Unit,
) {
    val interactionSource = remember { MutableInteractionSource() }
    val seekFocused by interactionSource.collectIsFocusedAsState()
    val seekEnabled = durationMs > 0

    Row(
        modifier = Modifier.fillMaxWidth(),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        Text(
            text = formatPlayerTime(positionMs),
            style = PlumTheme.typography.labelMedium,
            color = Color.White.copy(alpha = 0.75f),
        )
        Box(
            modifier =
                Modifier
                    .weight(1f)
                    .height(52.dp)
                    .then(
                        if (seekEnabled) {
                            Modifier
                                .semantics { contentDescription = "Timeline. Left or right to seek." }
                                .focusRequester(seekBarFocusRequester)
                                .focusProperties { down = playFocusRequester }
                                .focusable(interactionSource = interactionSource)
                                .onKeyEvent { event ->
                                    if (event.type != KeyEventType.KeyDown) return@onKeyEvent false
                                    when (event.key) {
                                        Key.DirectionLeft -> {
                                            onSeekStep(-1)
                                            true
                                        }
                                        Key.DirectionRight -> {
                                            onSeekStep(1)
                                            true
                                        }
                                        else -> false
                                    }
                                }
                        } else {
                            Modifier
                        },
                    ),
            contentAlignment = Alignment.Center,
        ) {
            PlexSeekBar(
                fraction = progressFraction,
                focused = seekFocused && seekEnabled,
                modifier = Modifier.fillMaxWidth(),
            )
        }
        Text(
            text = "-${formatPlayerTime(remainingMs)}",
            style = PlumTheme.typography.labelMedium,
            color = Color.White.copy(alpha = 0.75f),
        )
    }
}

@Composable
private fun PlexSeekBar(
    fraction: Float,
    modifier: Modifier = Modifier,
    focused: Boolean = false,
) {
    val accent = PlumTheme.palette.accent
    val f = fraction.coerceIn(0f, 1f)
    val barHeight = if (focused) 6.dp else 4.dp
    val thumbSize = if (focused) 16.dp else 12.dp
    val thumbRadius = thumbSize / 2
    BoxWithConstraints(
        modifier =
            modifier
                .height(if (focused) 28.dp else 20.dp)
                .then(
                    if (focused) {
                        Modifier
                            .border(
                                width = 1.5.dp,
                                color = accent.copy(alpha = 0.55f),
                                shape = RoundedCornerShape(10.dp),
                            )
                            .padding(horizontal = 10.dp, vertical = 6.dp)
                    } else {
                        Modifier
                    },
                ),
        contentAlignment = Alignment.CenterStart,
    ) {
        // Track background
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .height(barHeight)
                .clip(RoundedCornerShape(999.dp))
                .background(Color.White.copy(alpha = 0.15f)),
        )
        // Filled portion
        if (f > 0f) {
            Box(
                modifier = Modifier
                    .fillMaxWidth(fraction = f)
                    .height(barHeight)
                    .clip(RoundedCornerShape(999.dp))
                    .background(accent),
            )
        }
        // Thumb dot — offset to sit centered at the playhead position
        if (f > 0f) {
            val thumbOffset = (maxWidth * f - thumbRadius).coerceAtLeast(0.dp)
            Box(
                modifier = Modifier
                    .padding(start = thumbOffset)
                    .size(thumbSize)
                    .clip(CircleShape)
                    .then(
                        if (focused) {
                            Modifier.border(2.dp, accent.copy(alpha = 0.7f), CircleShape)
                        } else {
                            Modifier
                        },
                    )
                    .background(Color.White),
            )
        }
    }
}

// ── Control button ────────────────────────────────────────────────────────────

@Composable
private fun SkipIntroControl(
    label: String,
    contentDescription: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val palette = PlumTheme.palette
    val shape = RoundedCornerShape(16.dp)
    Surface(
        onClick = onClick,
        modifier =
            modifier
                .height(50.dp)
                .widthIn(min = 128.dp)
                .semantics { this.contentDescription = contentDescription },
        shape = ClickableSurfaceDefaults.shape(shape = shape),
        colors =
            ClickableSurfaceDefaults.colors(
                containerColor = palette.accent.copy(alpha = 0.24f),
                contentColor = Color.White,
                focusedContainerColor = palette.accent.copy(alpha = 0.42f),
                focusedContentColor = Color.White,
                pressedContainerColor = palette.accent.copy(alpha = 0.48f),
                pressedContentColor = Color.White,
            ),
        scale = ClickableSurfaceDefaults.scale(focusedScale = 1f),
        border =
            ClickableSurfaceDefaults.border(
                border = plumBorder(palette.accent.copy(alpha = 0.5f), 1.5.dp, shape),
                focusedBorder = plumBorder(palette.accent.copy(alpha = 0.9f), 2.dp, shape),
                pressedBorder = plumBorder(palette.accent.copy(alpha = 0.9f), 2.dp, shape),
            ),
        glow = ClickableSurfaceDefaults.glow(focusedGlow = Glow(Color.Transparent, 0.dp)),
    ) {
        Box(
            modifier = Modifier.padding(horizontal = 14.dp),
            contentAlignment = Alignment.Center,
        ) {
            Text(
                text = label,
                style = PlumTheme.typography.labelLarge,
                fontWeight = FontWeight.SemiBold,
            )
        }
    }
}

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
        utility -> { buttonSize = 48.dp; iconSize = 22.dp; cornerRadius = 14.dp }
        else    -> { buttonSize = 52.dp; iconSize = 26.dp; cornerRadius = 16.dp }
    }
    val shape = RoundedCornerShape(cornerRadius)
    val containerColor = when {
        ghost   -> Color.Transparent
        primary -> palette.accent
        else    -> Color.White.copy(alpha = 0.20f)
    }
    val focusedContainerColor = when {
        ghost   -> Color.White.copy(alpha = 0.14f)
        primary -> palette.accent
        else    -> Color.White.copy(alpha = 0.34f)
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
        scale = ClickableSurfaceDefaults.scale(focusedScale = 1f),
        border = ClickableSurfaceDefaults.border(
            border = plumBorder(
                if (ghost) Color.Transparent else Color.White.copy(alpha = 0.26f),
                if (ghost) 0.dp else 1.5.dp,
                shape,
            ),
            focusedBorder = plumBorder(palette.accent.copy(alpha = 0.85f), 2.dp, shape),
            pressedBorder = plumBorder(palette.accent.copy(alpha = 0.85f), 2.dp, shape),
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

private fun audioContentDescription(value: String?): String =
    if (!value.isNullOrBlank()) "Audio: ${value.trim()}"
    else "Change audio track"

private fun subtitleContentDescription(value: String?): String =
    if (!value.isNullOrBlank()) "Subtitles: ${value.trim()}"
    else "Change subtitles"

@OptIn(UnstableApi::class)
private fun PlayerView.applyPlumSubtitleAppearance(appearance: SubtitleAppearance) {
    subtitleView?.apply {
        val fg =
            runCatching { AndroidGraphicsColor.parseColor(appearance.colorHex) }
                .getOrElse { AndroidGraphicsColor.WHITE }
        setStyle(
            CaptionStyleCompat(
                fg,
                AndroidGraphicsColor.TRANSPARENT,
                AndroidGraphicsColor.TRANSPARENT,
                CaptionStyleCompat.EDGE_TYPE_DROP_SHADOW,
                AndroidGraphicsColor.BLACK,
                null,
            ),
        )
        setApplyEmbeddedStyles(false)
        setApplyEmbeddedFontSizes(false)
        val sp =
            when (appearance.size) {
                SubtitleSize.SMALL -> 26f
                SubtitleSize.MEDIUM -> 34f
                SubtitleSize.LARGE -> 42f
            }
        setFixedTextSize(android.util.TypedValue.COMPLEX_UNIT_SP, sp)
        setBottomPaddingFraction(
            when (appearance.position) {
                SubtitlePosition.BOTTOM -> 0.12f
                SubtitlePosition.TOP -> 0.04f
            },
        )
        updateLayoutParams<FrameLayout.LayoutParams> {
            gravity =
                when (appearance.position) {
                    SubtitlePosition.TOP ->
                        Gravity.TOP or Gravity.CENTER_HORIZONTAL
                    SubtitlePosition.BOTTOM ->
                        Gravity.BOTTOM or Gravity.CENTER_HORIZONTAL
                }
        }
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
