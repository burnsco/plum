package plum.tv.feature.details

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.itemsIndexed
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
import androidx.tv.material3.Card
import androidx.tv.material3.CardDefaults
import androidx.tv.material3.ExperimentalTvMaterial3Api
import androidx.tv.material3.Text
import coil.compose.AsyncImage
import plum.tv.core.network.LibraryBrowseItemJson

@OptIn(ExperimentalTvMaterial3Api::class)
@Composable
fun ShowDetailRoute(
    onBack: () -> Unit,
    onPlayEpisode: (mediaId: Int, resumeSec: Float) -> Unit,
    viewModel: ShowDetailViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    when (val s = state) {
        is ShowDetailUiState.Loading -> Text("Loading…", modifier = Modifier.padding(48.dp))
        is ShowDetailUiState.Error -> Column(Modifier.padding(48.dp)) {
            Text(s.message)
            Button(onClick = { viewModel.load() }) { Text("Retry") }
            Button(
                onClick = onBack,
                modifier = Modifier.fillMaxWidth().height(52.dp),
                scale = ButtonDefaults.scale(focusedScale = 1.08f),
            ) { Text("Back") }
        }
        is ShowDetailUiState.Ready -> {
            val d = s.details
            val selectedEpisodes = s.seasons.getOrNull(s.selectedSeasonIndex)?.episodes.orEmpty()
            Column(
                modifier = Modifier
                    .fillMaxSize()
                    .verticalScroll(rememberScrollState())
                    .padding(48.dp),
                verticalArrangement = Arrangement.spacedBy(20.dp),
            ) {
                Row(horizontalArrangement = Arrangement.spacedBy(32.dp)) {
                    val poster = d.posterUrl
                    if (poster != null) {
                        AsyncImage(
                            model = poster,
                            contentDescription = d.name,
                            modifier = Modifier
                                .width(280.dp)
                                .height(420.dp),
                            contentScale = ContentScale.Crop,
                        )
                    }
                    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        Text(d.name)
                        if (d.firstAirDate.isNotBlank()) Text(d.firstAirDate)
                        Text("${d.numberOfSeasons} seasons · ${d.numberOfEpisodes} episodes")
                        if (d.genres.isNotEmpty()) Text(d.genres.joinToString(", "))
                        Text(
                            text = d.overview,
                            maxLines = 10,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                }
                Text("Seasons", modifier = Modifier.padding(top = 8.dp))
                LazyRow(
                    horizontalArrangement = Arrangement.spacedBy(12.dp),
                    contentPadding = PaddingValues(vertical = 4.dp),
                ) {
                    itemsIndexed(s.seasons) { index, season ->
                        Button(
                            onClick = { viewModel.selectSeason(index) },
                            modifier = Modifier
                                .height(48.dp)
                                .padding(end = 4.dp),
                            scale = ButtonDefaults.scale(focusedScale = 1.08f),
                        ) {
                            val mark = if (index == s.selectedSeasonIndex) "▶ " else ""
                            Text(mark + season.label)
                        }
                    }
                }
                Text("Episodes", modifier = Modifier.padding(top = 8.dp))
                Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    for (ep in selectedEpisodes) {
                        EpisodeRow(
                            ep,
                            onPlay = {
                                val resume = (ep.progressSeconds ?: 0.0).toFloat()
                                onPlayEpisode(ep.id, resume)
                            },
                        )
                    }
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

@OptIn(ExperimentalTvMaterial3Api::class)
@Composable
private fun EpisodeRow(
    ep: LibraryBrowseItemJson,
    onPlay: () -> Unit,
) {
    Card(
        onClick = onPlay,
        scale = CardDefaults.scale(focusedScale = 1.05f),
    ) {
        Row(
            modifier = Modifier.padding(16.dp),
            horizontalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            val thumb = ep.thumbnailUrl ?: ep.posterUrl
            if (thumb != null) {
                AsyncImage(
                    model = thumb,
                    contentDescription = ep.title,
                    modifier = Modifier
                        .width(160.dp)
                        .height(90.dp),
                    contentScale = ContentScale.Crop,
                )
            }
            Column {
                Text(ep.title, maxLines = 2, overflow = TextOverflow.Ellipsis)
                val se = ep.season
                val epn = ep.episode
                if (se != null && epn != null) {
                    Text("S${se.toString().padStart(2, '0')}E${epn.toString().padStart(2, '0')}")
                }
            }
        }
    }
}
