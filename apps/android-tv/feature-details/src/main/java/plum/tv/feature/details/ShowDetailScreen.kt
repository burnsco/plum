package plum.tv.feature.details

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.tv.material3.Text
import coil.compose.AsyncImage
import plum.tv.core.network.LibraryBrowseItemJson
import plum.tv.core.network.TitleCastMemberJson
import plum.tv.core.ui.LocalServerBaseUrl
import plum.tv.core.ui.PlumActionButton
import plum.tv.core.ui.PlumButtonVariant
import plum.tv.core.ui.PlumMetadataChips
import plum.tv.core.ui.PlumPanel
import plum.tv.core.ui.PlumSectionHeader
import plum.tv.core.ui.PlumImageSizes
import plum.tv.core.ui.PlumTheme
import plum.tv.core.ui.resolveArtworkUrl

@Composable
fun ShowDetailRoute(
    onBack: () -> Unit,
    onPlayEpisode: (mediaId: Int, resumeSec: Float, libraryId: Int, showKey: String) -> Unit,
    viewModel: ShowDetailViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val serverBase = LocalServerBaseUrl.current

    when (val s = state) {
        is ShowDetailUiState.Loading -> Box(
            Modifier.fillMaxSize(),
            contentAlignment = Alignment.Center,
        ) {
            Text("Loading…", color = PlumTheme.palette.muted)
        }
        is ShowDetailUiState.Error -> Column(
            Modifier.fillMaxSize().padding(48.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            Text(s.message, color = PlumTheme.palette.muted)
            PlumActionButton("Retry", onClick = { viewModel.load() }, leadingBadge = "R")
            PlumActionButton("Back", onClick = onBack, variant = PlumButtonVariant.Ghost)
        }
        is ShowDetailUiState.Ready -> {
            val d = s.details
            val selectedEpisodes = s.seasons.getOrNull(s.selectedSeasonIndex)?.episodes.orEmpty()
            val backdropUrl =
                resolveArtworkUrl(serverBase, d.backdropUrl, d.backdropPath, PlumImageSizes.BACKDROP_HERO)
            val posterUrl = resolveArtworkUrl(serverBase, d.posterUrl, d.posterPath, PlumImageSizes.POSTER_DETAIL)
            val metrics = PlumTheme.metrics

            Box(modifier = Modifier.fillMaxSize()) {
                // Full-bleed backdrop
                if (backdropUrl != null) {
                    AsyncImage(
                        model = backdropUrl,
                        contentDescription = null,
                        modifier = Modifier.fillMaxSize(),
                        contentScale = ContentScale.Crop,
                    )
                }
                Box(
                    modifier = Modifier
                        .fillMaxSize()
                        .background(
                            Brush.verticalGradient(
                                0.0f to Color(0xCC000000),
                                0.35f to Color(0xDD000000),
                                1.0f to Color(0xF5000000),
                            ),
                        ),
                )

                Column(
                    modifier = Modifier
                        .fillMaxSize()
                        .verticalScroll(rememberScrollState())
                        .padding(horizontal = 36.dp, vertical = 28.dp),
                    verticalArrangement = Arrangement.spacedBy(20.dp),
                ) {
                    // Header: poster + info
                    Row(horizontalArrangement = Arrangement.spacedBy(28.dp)) {
                        if (posterUrl != null) {
                            AsyncImage(
                                model = posterUrl,
                                contentDescription = d.name,
                                modifier = Modifier
                                    .width(metrics.heroPosterWidth)
                                    .height(metrics.heroPosterHeight)
                                    .clip(RoundedCornerShape(10.dp)),
                                contentScale = ContentScale.Crop,
                            )
                        }

                        Column(
                            modifier = Modifier.weight(1f),
                            verticalArrangement = Arrangement.spacedBy(14.dp),
                        ) {
                            Text(
                                text = d.name,
                                style = PlumTheme.typography.headlineLarge,
                                color = Color.White,
                                fontWeight = FontWeight.Bold,
                            )

                            PlumMetadataChips(
                                values = buildList {
                                    d.firstAirDate.take(4).takeIf { it.isNotBlank() }?.let(::add)
                                    add("${d.numberOfSeasons} seasons")
                                    d.imdbRating?.let { add("★ ${"%.1f".format(it)}") }
                                    addAll(d.genres.take(3))
                                },
                            )

                            if (d.overview.isNotBlank()) {
                                Text(
                                    text = d.overview,
                                    maxLines = 6,
                                    overflow = TextOverflow.Ellipsis,
                                    style = PlumTheme.typography.bodyLarge,
                                    color = Color.White.copy(alpha = 0.85f),
                                )
                            }

                            PlumActionButton("Back", onClick = onBack, variant = PlumButtonVariant.Secondary)
                        }
                    }

                    // Season picker
                    if (s.seasons.isNotEmpty()) {
                        PlumSectionHeader("Seasons")
                        LazyRow(
                            horizontalArrangement = Arrangement.spacedBy(10.dp),
                            contentPadding = PaddingValues(vertical = 4.dp),
                        ) {
                            itemsIndexed(s.seasons) { index, season ->
                                val count = season.episodes.size
                                val suffix = if (count == 1) "1 ep" else "$count eps"
                                PlumActionButton(
                                    label = "${season.label} · $suffix",
                                    onClick = { viewModel.selectSeason(index) },
                                    variant = if (index == s.selectedSeasonIndex) PlumButtonVariant.Primary else PlumButtonVariant.Ghost,
                                )
                            }
                        }
                    }

                    // Episode list
                    if (selectedEpisodes.isNotEmpty()) {
                        PlumSectionHeader("Episodes")
                        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                            selectedEpisodes.forEach { ep ->
                                    EpisodeRow(
                                        ep = ep,
                                        serverBase = serverBase,
                                        onPlay = {
                                            val resume = (ep.progressSeconds ?: 0.0).toFloat()
                                            onPlayEpisode(ep.id, resume, d.libraryId, d.showKey)
                                        },
                                    )
                                }
                            }
                    }

                    // Cast section
                    if (d.cast.isNotEmpty()) {
                        ShowCastSection(cast = d.cast)
                    }

                    Spacer(modifier = Modifier.height(16.dp))
                }
            }
        }
    }
}

@Composable
private fun EpisodeRow(
    ep: LibraryBrowseItemJson,
    serverBase: String,
    onPlay: () -> Unit,
) {
    val palette = PlumTheme.palette
    val thumbUrl =
        resolveArtworkUrl(serverBase, ep.thumbnailUrl, ep.thumbnailPath, PlumImageSizes.THUMB_SMALL)
            ?: resolveArtworkUrl(serverBase, ep.posterUrl, ep.posterPath, PlumImageSizes.POSTER_GRID)
            ?: resolveArtworkUrl(serverBase, ep.showPosterUrl, ep.showPosterPath, PlumImageSizes.POSTER_GRID)

    Row(
        modifier = Modifier
            .fillMaxWidth()
            .background(
                color = palette.panel,
                shape = RoundedCornerShape(10.dp),
            )
            .clip(RoundedCornerShape(10.dp)),
        horizontalArrangement = Arrangement.spacedBy(0.dp),
    ) {
        // Thumbnail
        Box(
            modifier = Modifier
                .width(180.dp)
                .height(100.dp),
        ) {
            if (thumbUrl != null) {
                AsyncImage(
                    model = thumbUrl,
                    contentDescription = ep.title,
                    modifier = Modifier.fillMaxSize(),
                    contentScale = ContentScale.Crop,
                )
            } else {
                Box(
                    modifier = Modifier.fillMaxSize().background(palette.surface),
                    contentAlignment = Alignment.Center,
                ) {
                    val se = ep.season
                    val epn = ep.episode
                    if (se != null && epn != null) {
                        Text(
                            text = "S${se.toString().padStart(2, '0')}E${epn.toString().padStart(2, '0')}",
                            style = PlumTheme.typography.labelMedium,
                            color = palette.muted,
                        )
                    }
                }
            }
        }

        // Episode info + play button
        Column(
            modifier = Modifier
                .weight(1f)
                .padding(horizontal = 16.dp, vertical = 12.dp),
            verticalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.Top,
            ) {
                Column(
                    modifier = Modifier.weight(1f),
                    verticalArrangement = Arrangement.spacedBy(3.dp),
                ) {
                    val se = ep.season
                    val epn = ep.episode
                    if (se != null && epn != null) {
                        Text(
                            text = "S${se.toString().padStart(2, '0')}E${epn.toString().padStart(2, '0')}",
                            style = PlumTheme.typography.labelSmall,
                            color = palette.muted,
                        )
                    }
                    Text(
                        text = ep.title,
                        style = PlumTheme.typography.titleSmall,
                        color = palette.text,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
                PlumActionButton(
                    label = "Play",
                    onClick = onPlay,
                    variant = PlumButtonVariant.Primary,
                    leadingBadge = "▶",
                )
            }

            ep.overview?.takeIf { it.isNotBlank() }?.let { overview ->
                Text(
                    text = overview,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                    style = PlumTheme.typography.bodySmall,
                    color = palette.muted,
                )
            }
        }
    }
}

@Composable
private fun ShowCastSection(cast: List<TitleCastMemberJson>) {
    PlumPanel(modifier = Modifier.fillMaxWidth()) {
        Column(verticalArrangement = Arrangement.spacedBy(16.dp)) {
            PlumSectionHeader(title = "Cast")
            val palette = PlumTheme.palette
            val rows = cast.take(18).chunked(3)
            rows.forEach { rowItems ->
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(12.dp),
                ) {
                    rowItems.forEach { member ->
                        Box(
                            modifier = Modifier
                                .weight(1f)
                                .clip(RoundedCornerShape(PlumTheme.metrics.tileRadius))
                                .background(palette.panelAlt)
                                .padding(horizontal = 14.dp, vertical = 10.dp),
                        ) {
                            Column(verticalArrangement = Arrangement.spacedBy(3.dp)) {
                                Text(
                                    text = member.name,
                                    style = PlumTheme.typography.titleSmall,
                                    color = palette.text,
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis,
                                )
                                member.character?.takeIf { it.isNotBlank() }?.let { character ->
                                    Text(
                                        text = character,
                                        style = PlumTheme.typography.bodySmall,
                                        color = palette.muted,
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis,
                                    )
                                }
                            }
                        }
                    }
                    repeat(3 - rowItems.size) {
                        Spacer(modifier = Modifier.weight(1f))
                    }
                }
            }
        }
    }
}
