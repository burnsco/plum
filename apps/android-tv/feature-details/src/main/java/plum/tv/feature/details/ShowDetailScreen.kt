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
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.Glow
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import coil.compose.AsyncImage
import plum.tv.core.network.LibraryBrowseItemJson
import plum.tv.core.ui.PlumCastMember
import plum.tv.core.ui.PlumCastSection
import plum.tv.core.ui.LocalServerBaseUrl
import plum.tv.core.ui.PlumActionButton
import plum.tv.core.ui.PlumButtonVariant
import plum.tv.core.ui.PlumDetailBackground
import plum.tv.core.ui.PlumDetailHeroHeader
import plum.tv.core.ui.PlumMetadataChips
import plum.tv.core.ui.PlumImageSizes
import plum.tv.core.ui.PlumScrims
import plum.tv.core.ui.PlumSectionHeader
import plum.tv.core.ui.PlumStatePanel
import plum.tv.core.ui.PlumTheme
import plum.tv.core.ui.plumBorder
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
            PlumStatePanel(
                title = "Loading",
                message = "Fetching show details…",
            )
        }
        is ShowDetailUiState.Error -> Box(
            Modifier.fillMaxSize(),
            contentAlignment = Alignment.Center,
        ) {
            PlumStatePanel(
                title = "Could not load show",
                message = s.message,
                actions = {
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        PlumActionButton("Retry", onClick = { viewModel.load() }, leadingBadge = "R")
                        PlumActionButton("Back", onClick = onBack, variant = PlumButtonVariant.Ghost)
                    }
                },
            )
        }
        is ShowDetailUiState.Ready -> {
            val d = s.details
            val selectedEpisodes = s.seasons.getOrNull(s.selectedSeasonIndex)?.episodes.orEmpty()
            val backdropUrl =
                resolveArtworkUrl(serverBase, d.backdropUrl, d.backdropPath, PlumImageSizes.BACKDROP_HERO)
            val posterUrl = resolveArtworkUrl(serverBase, d.posterUrl, d.posterPath, PlumImageSizes.POSTER_DETAIL)

            PlumDetailBackground(
                backdropUrl = backdropUrl,
                scrim = PlumScrims.backdropVertical,
            ) {
                // LazyColumn avoids rendering all episodes upfront for long seasons.
                LazyColumn(
                    modifier = Modifier.fillMaxSize(),
                    contentPadding = PaddingValues(horizontal = 36.dp, vertical = 28.dp),
                    verticalArrangement = Arrangement.spacedBy(20.dp),
                ) {
                    item {
                        PlumDetailHeroHeader(posterUrl = posterUrl) {
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

                    if (s.seasons.isNotEmpty()) {
                        item {
                            PlumSectionHeader("Seasons")
                        }
                        item {
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
                    }

                    if (selectedEpisodes.isNotEmpty()) {
                        item {
                            PlumSectionHeader("Episodes")
                        }
                        items(selectedEpisodes, key = { it.id }) { ep ->
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

                    if (!d.cast.isNullOrEmpty()) {
                        item {
                            PlumCastSection(
                                cast = d.cast.orEmpty().map { member ->
                                    PlumCastMember(
                                        name = member.name,
                                        character = member.character,
                                    )
                                },
                            )
                        }
                    }

                    item { Spacer(modifier = Modifier.height(16.dp)) }
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
    val metrics = PlumTheme.metrics
    val shape = RoundedCornerShape(metrics.tileRadius)
    val thumbUrl =
        resolveArtworkUrl(serverBase, ep.thumbnailUrl, ep.thumbnailPath, PlumImageSizes.THUMB_SMALL)
            ?: resolveArtworkUrl(serverBase, ep.posterUrl, ep.posterPath, PlumImageSizes.POSTER_GRID)
            ?: resolveArtworkUrl(serverBase, ep.showPosterUrl, ep.showPosterPath, PlumImageSizes.POSTER_GRID)

    // The whole row is a TV Surface so the user can focus it and press OK to play —
    // no need for a nested Play button.
    Surface(
        onClick = onPlay,
        modifier = Modifier.fillMaxWidth(),
        shape = ClickableSurfaceDefaults.shape(shape = shape),
        colors = ClickableSurfaceDefaults.colors(
            containerColor = palette.panel,
            contentColor = palette.text,
            focusedContainerColor = palette.panelAlt,
            focusedContentColor = palette.text,
            pressedContainerColor = palette.panelAlt,
            pressedContentColor = palette.text,
        ),
        scale = ClickableSurfaceDefaults.scale(focusedScale = 1.02f),
        border = ClickableSurfaceDefaults.border(
            border = plumBorder(Color.Transparent, 0.dp, shape),
            focusedBorder = plumBorder(palette.borderStrong, 2.dp, shape),
            pressedBorder = plumBorder(palette.borderStrong, 2.dp, shape),
        ),
        glow = ClickableSurfaceDefaults.glow(focusedGlow = Glow(Color.Transparent, 0.dp)),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            // Thumbnail
            Box(
                modifier = Modifier
                    .width(metrics.thumbnailWidth)
                    .height(metrics.thumbnailHeight)
                    .clip(RoundedCornerShape(topStart = metrics.tileRadius, bottomStart = metrics.tileRadius)),
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
                        modifier = Modifier
                            .fillMaxSize()
                            .background(palette.surface),
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

            // Episode info + play icon indicator
            Row(
                modifier = Modifier
                    .weight(1f)
                    .padding(horizontal = 16.dp, vertical = 12.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
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

                // Focused border on the Surface already signals this row is playable;
                // no redundant play button needed.
                Text(
                    text = "▶",
                    style = PlumTheme.typography.labelMedium,
                    color = palette.muted,
                    modifier = Modifier.padding(start = 12.dp),
                )
            }
        }
    }
}
