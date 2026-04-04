package plum.tv.app

import android.text.format.DateFormat
import android.view.KeyEvent as AndroidKeyEvent
import android.view.ViewGroup
import android.view.WindowManager
import androidx.activity.ComponentActivity
import androidx.activity.compose.BackHandler
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.foundation.background
import androidx.compose.foundation.border
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
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.withFrameNanos
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableLongStateOf
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
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import androidx.media3.common.util.UnstableApi
import androidx.media3.ui.AspectRatioFrameLayout
import androidx.media3.ui.CaptionStyleCompat
import androidx.media3.ui.PlayerView
import coil.compose.AsyncImage
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.Glow
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import java.util.Date
import kotlinx.coroutines.delay
import plum.tv.core.player.TrackPicker
import plum.tv.core.player.TrackPickerOption
import plum.tv.core.player.UpNextOverlayState
import plum.tv.core.ui.LocalServerBaseUrl
import plum.tv.core.ui.PlumImageSizes
import plum.tv.core.ui.PlumScrims
import plum.tv.core.ui.resolveArtworkUrl
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
    val trackPicker by viewModel.trackPicker.collectAsState()
    val upNext by viewModel.upNext.collectAsState()
    val rootFocusRequester = remember { FocusRequester() }
    val playFocusRequester = remember { FocusRequester() }
    val seekBarFocusRequester = remember { FocusRequester() }
    val upNextPlayFocusRequester = remember { FocusRequester() }

    // Each time hideTimerKey changes, the LaunchedEffect restarts the hide countdown.
    var hideTimerKey by remember { mutableIntStateOf(0) }
    var controlsVisible by remember { mutableStateOf(true) }

    val context = LocalContext.current
    val lifecycleOwner = LocalLifecycleOwner.current
    val timeFormat = remember(context) { DateFormat.getTimeFormat(context) }
    var nowMs by remember { mutableLongStateOf(System.currentTimeMillis()) }

    DisposableEffect(lifecycleOwner, viewModel) {
        val observer = LifecycleEventObserver { _, event ->
            if (event == Lifecycle.Event.ON_STOP) {
                viewModel.pauseWhenBackgrounded()
            }
        }
        lifecycleOwner.lifecycle.addObserver(observer)
        onDispose { lifecycleOwner.lifecycle.removeObserver(observer) }
    }

    LaunchedEffect(controlsVisible) {
        if (!controlsVisible) return@LaunchedEffect
        nowMs = System.currentTimeMillis()
        while (true) {
            delay(1_000)
            nowMs = System.currentTimeMillis()
        }
    }

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

    // Surface the skip control when intro detection kicks in mid-playback (auto-hide would hide it).
    LaunchedEffect(ui.showSkipIntro) {
        if (ui.showSkipIntro) {
            showControls()
        }
    }

    LaunchedEffect(upNext) {
        if (upNext != null) {
            showControls()
            upNextPlayFocusRequester.requestFocus()
        }
    }

    BackHandler(trackPicker != null) {
        viewModel.dismissTrackPicker()
    }

    BackHandler(trackPicker == null && upNext != null) {
        viewModel.dismissUpNext()
    }

    BackHandler(trackPicker == null && upNext == null) {
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

    val keepAwake =
        ui.isPlaying ||
            ui.isBuffering ||
            upNext != null ||
            trackPicker != null
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
                    configurePlumSubtitleOverlay()
                }
            },
            update = { view ->
                if (view.player !== viewModel.player) {
                    view.player = viewModel.player
                }
                view.isFocusable = false
                view.isFocusableInTouchMode = false
                view.descendantFocusability = ViewGroup.FOCUS_BLOCK_DESCENDANTS
                view.configurePlumSubtitleOverlay()
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

        // ── Local time (top right, with controls) ────────────────────────────────
        AnimatedVisibility(
            visible = controlsVisible,
            enter = fadeIn(),
            exit = fadeOut(),
            modifier = Modifier.align(Alignment.TopEnd),
        ) {
            Text(
                text = timeFormat.format(Date(nowMs)),
                style = PlumTheme.typography.titleMedium,
                color = Color.White.copy(alpha = 0.9f),
                fontWeight = FontWeight.Medium,
                modifier = Modifier.padding(horizontal = 40.dp, vertical = 28.dp),
            )
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
                TimelineSeekRow(
                    positionMs = ui.positionMs,
                    remainingMs = ui.remainingMs,
                    durationMs = ui.durationMs,
                    progressFraction = ui.progressFraction,
                    seekBarFocusRequester = seekBarFocusRequester,
                    playFocusRequester = playFocusRequester,
                    onSeekStep = { viewModel.seekTimelineBySteps(it) },
                )

                // Buttons row: utility-left | center controls | utility-right
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    // Left spacer (keeps center controls truly centered)
                    Spacer(modifier = Modifier.weight(1f))

                    // Center: playback controls
                    Row(
                        horizontalArrangement = Arrangement.spacedBy(14.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        PlayerControlButton(
                            icon = Icons.Filled.SkipPrevious,
                            contentDescription = "Previous",
                            onClick = { viewModel.previousEpisode() },
                            enabled = ui.canPrev,
                            modifier = controlUpToSeekBar(ui.durationMs, seekBarFocusRequester),
                        )
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
                                onClick = {
                                    viewModel.skipIntro()
                                    showControls()
                                },
                                modifier = controlUpToSeekBar(ui.durationMs, seekBarFocusRequester),
                            )
                        }
                        PlayerControlButton(
                            icon = Icons.Filled.SkipNext,
                            contentDescription = "Next",
                            onClick = { viewModel.nextEpisode() },
                            enabled = ui.canNext,
                            modifier = controlUpToSeekBar(ui.durationMs, seekBarFocusRequester),
                        )
                    }

                    // Right: utility buttons
                    Row(
                        modifier = Modifier.weight(1f),
                        horizontalArrangement = Arrangement.End,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                            PlayerControlButton(
                                icon = Icons.AutoMirrored.Filled.VolumeUp,
                                contentDescription = audioContentDescription(ui.audioTrackLabel, ui.canCycleAudio),
                                onClick = { viewModel.openAudioPicker() },
                                enabled = ui.canCycleAudio,
                                utility = true,
                                modifier = controlUpToSeekBar(ui.durationMs, seekBarFocusRequester),
                            )
                            PlayerControlButton(
                                icon = Icons.Filled.Subtitles,
                                contentDescription = subtitleContentDescription(ui.subtitleTrackLabel, ui.canCycleSubtitles),
                                onClick = { viewModel.openSubtitlePicker() },
                                enabled = ui.canCycleSubtitles,
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
                picker = picker,
                onDismiss = { viewModel.dismissTrackPicker() },
                onSelect = { viewModel.selectTrackPickerOption(it) },
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
            verticalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            Text(
                text = "UP NEXT",
                style = PlumTheme.typography.labelMedium,
                color = Color.White.copy(alpha = 0.55f),
                fontWeight = FontWeight.SemiBold,
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
                text = "Starting in ${state.secondsRemaining}s",
                style = PlumTheme.typography.bodyMedium,
                color = Color.White.copy(alpha = 0.78f),
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
                scale = ClickableSurfaceDefaults.scale(focusedScale = 1.04f),
                border =
                    ClickableSurfaceDefaults.border(
                        border = plumBorder(Color.White.copy(alpha = 0.2f), 1.dp, shape),
                        focusedBorder = plumBorder(Color.White.copy(alpha = 0.55f), 2.dp, shape),
                        pressedBorder = plumBorder(Color.White.copy(alpha = 0.55f), 2.dp, shape),
                    ),
                glow = ClickableSurfaceDefaults.glow(focusedGlow = Glow(palette.accent.copy(alpha = 0.45f), 10.dp)),
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

@Composable
private fun TrackPickerOverlay(
    picker: TrackPicker,
    onDismiss: () -> Unit,
    onSelect: (String) -> Unit,
) {
    val palette = PlumTheme.palette
    val options = picker.options
    val focusRequesters = remember(picker) { List(options.size) { FocusRequester() } }

    LaunchedEffect(picker) {
        val idx = options.indexOfFirst { it.selected }.let { if (it < 0) 0 else it }
        focusRequesters.getOrNull(idx)?.requestFocus()
    }

    Box(
        modifier =
            Modifier
                .fillMaxSize()
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
                        onClick = { onSelect(opt.id) },
                    )
                }
            }
            Spacer(modifier = Modifier.height(10.dp))
            Surface(
                onClick = onDismiss,
                modifier = Modifier.fillMaxWidth(),
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
                scale = ClickableSurfaceDefaults.scale(focusedScale = 1.02f),
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
                        text = "Cancel",
                        style = PlumTheme.typography.labelMedium,
                    )
                }
            }
        }
    }
}

@Composable
private fun TrackPickerRow(
    option: TrackPickerOption,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val palette = PlumTheme.palette
    val shape = RoundedCornerShape(10.dp)
    Surface(
        onClick = onClick,
        modifier = modifier.fillMaxWidth(),
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
        scale = ClickableSurfaceDefaults.scale(focusedScale = 1.02f),
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
    val thumbSize = if (focused) 15.dp else 13.dp
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
                                color = accent.copy(alpha = 0.65f),
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
                .background(Color.White.copy(alpha = 0.18f)),
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
                    .background(Color.White),
            )
        }
    }
}

// ── Control button ────────────────────────────────────────────────────────────

@Composable
private fun SkipIntroControl(
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
                .semantics { contentDescription = "Skip intro" },
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
        scale = ClickableSurfaceDefaults.scale(focusedScale = 1.05f),
        border =
            ClickableSurfaceDefaults.border(
                border = plumBorder(palette.accent.copy(alpha = 0.5f), 1.5.dp, shape),
                focusedBorder = plumBorder(palette.accent.copy(alpha = 0.9f), 2.dp, shape),
                pressedBorder = plumBorder(palette.accent.copy(alpha = 0.9f), 2.dp, shape),
            ),
        glow = ClickableSurfaceDefaults.glow(focusedGlow = Glow(palette.accent.copy(alpha = 0.38f), 8.dp)),
    ) {
        Box(
            modifier = Modifier.padding(horizontal = 14.dp),
            contentAlignment = Alignment.Center,
        ) {
            Text(
                text = "Skip intro",
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
        scale = ClickableSurfaceDefaults.scale(focusedScale = 1.06f),
        border = ClickableSurfaceDefaults.border(
            border = plumBorder(
                if (ghost) Color.Transparent else Color.White.copy(alpha = 0.26f),
                if (ghost) 0.dp else 1.5.dp,
                shape,
            ),
            focusedBorder = plumBorder(palette.accent.copy(alpha = 0.85f), 2.dp, shape),
            pressedBorder = plumBorder(palette.accent.copy(alpha = 0.85f), 2.dp, shape),
        ),
        glow = ClickableSurfaceDefaults.glow(
            focusedGlow =
                when {
                    primary -> Glow(palette.accent.copy(alpha = 0.38f), 10.dp)
                    ghost -> Glow(Color.Transparent, 0.dp)
                    else -> Glow(Color.White.copy(alpha = 0.22f), 9.dp)
                },
        ),
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

private fun PlayerView.configurePlumSubtitleOverlay() {
    subtitleView?.apply {
        setStyle(
            CaptionStyleCompat(
                android.graphics.Color.WHITE,
                android.graphics.Color.TRANSPARENT,
                android.graphics.Color.TRANSPARENT,
                CaptionStyleCompat.EDGE_TYPE_DROP_SHADOW,
                android.graphics.Color.BLACK,
                null,
            ),
        )
        setApplyEmbeddedStyles(false)
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
