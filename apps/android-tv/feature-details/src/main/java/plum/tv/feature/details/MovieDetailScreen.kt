package plum.tv.feature.details

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.tv.material3.Button
import androidx.tv.material3.ButtonDefaults
import androidx.tv.material3.ExperimentalTvMaterial3Api
import androidx.tv.material3.Text
import coil.compose.AsyncImage
import androidx.compose.foundation.layout.Row

@OptIn(ExperimentalTvMaterial3Api::class)
@Composable
fun MovieDetailRoute(
    onBack: () -> Unit,
    onPlay: (mediaId: Int) -> Unit,
    viewModel: MovieDetailViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    when (val s = state) {
        is MovieDetailUiState.Loading -> Text("Loading…", modifier = Modifier.padding(48.dp))
        is MovieDetailUiState.Error -> Column(Modifier.padding(48.dp)) {
            Text(s.message)
            Button(onClick = { viewModel.load() }) { Text("Retry") }
            Button(
                onClick = onBack,
                modifier = Modifier.fillMaxWidth().height(52.dp),
                scale = ButtonDefaults.scale(focusedScale = 1.08f),
            ) { Text("Back") }
        }
        is MovieDetailUiState.Ready -> {
            val d = s.details
            Row(
                modifier = Modifier
                    .fillMaxSize()
                    .verticalScroll(rememberScrollState())
                    .padding(48.dp),
                horizontalArrangement = Arrangement.spacedBy(32.dp),
            ) {
                val poster = d.posterUrl
                if (poster != null) {
                    AsyncImage(
                        model = poster,
                        contentDescription = d.title,
                        modifier = Modifier
                            .width(320.dp)
                            .height(480.dp),
                        contentScale = ContentScale.Crop,
                    )
                }
                Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    Text(d.title)
                    d.releaseDate?.takeIf { it.isNotBlank() }?.let { releaseDate ->
                        Text(releaseDate)
                    }
                    d.runtime?.takeIf { it > 0 }?.let { runtime ->
                        Text("$runtime min")
                    }
                    if (d.genres.isNotEmpty()) Text(d.genres.joinToString(", "))
                    Text(
                        text = d.overview,
                        maxLines = 12,
                        overflow = TextOverflow.Ellipsis,
                    )
                    Button(
                        onClick = { onPlay(d.mediaId) },
                        modifier = Modifier
                            .padding(top = 20.dp)
                            .fillMaxWidth()
                            .height(52.dp),
                        scale = ButtonDefaults.scale(focusedScale = 1.08f),
                    ) {
                        Text("Play")
                    }
                    Button(
                        onClick = onBack,
                        modifier = Modifier.fillMaxWidth().height(52.dp),
                        scale = ButtonDefaults.scale(focusedScale = 1.08f),
                    ) { Text("Back") }
                }
            }
        }
    }
}
