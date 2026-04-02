package plum.tv.app

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.media3.common.util.UnstableApi
import androidx.media3.ui.PlayerView
import androidx.tv.material3.Button
import androidx.tv.material3.ButtonDefaults
import androidx.tv.material3.ExperimentalTvMaterial3Api
import androidx.tv.material3.Text

@OptIn(ExperimentalTvMaterial3Api::class)
@UnstableApi
@Composable
fun PlayerRoute(
    onClose: () -> Unit,
    viewModel: PlayerViewModel = hiltViewModel(),
) {
    val err by viewModel.error.collectAsState()
    val status by viewModel.status.collectAsState()

    BackHandler {
        onClose()
    }

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Color.Black),
    ) {
        AndroidView(
            modifier = Modifier.fillMaxSize(),
            factory = { ctx ->
                PlayerView(ctx).apply {
                    useController = true
                    controllerShowTimeoutMs = 3500
                    player = viewModel.player
                }
            },
            update = {
                if (it.player !== viewModel.player) {
                    it.player = viewModel.player
                }
            },
        )
        Column(
            modifier = Modifier
                .align(Alignment.TopStart)
                .padding(28.dp),
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(16.dp),
            ) {
                Button(
                    onClick = onClose,
                    modifier = Modifier.height(52.dp),
                    scale = ButtonDefaults.scale(focusedScale = 1.1f),
                ) {
                    Text("Back", style = MaterialTheme.typography.labelLarge)
                }
                Button(
                    onClick = { viewModel.cycleAudioTrack() },
                    modifier = Modifier.height(52.dp),
                    scale = ButtonDefaults.scale(focusedScale = 1.1f),
                ) {
                    Text("Audio", style = MaterialTheme.typography.labelLarge)
                }
                Button(
                    onClick = { viewModel.cycleSubtitles() },
                    modifier = Modifier.height(52.dp),
                    scale = ButtonDefaults.scale(focusedScale = 1.1f),
                ) {
                    Text("Subtitles", style = MaterialTheme.typography.labelLarge)
                }
            }
            Text(
                text = status,
                modifier = Modifier.padding(top = 16.dp),
                style = MaterialTheme.typography.bodyLarge,
                color = Color.White,
            )
            err?.let {
                Text(
                    text = it,
                    modifier = Modifier.padding(top = 8.dp),
                    style = MaterialTheme.typography.bodyMedium,
                    color = Color(0xFFFF6B6B),
                )
            }
        }
    }

}
