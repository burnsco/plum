package plum.tv.feature.details

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.itemsIndexed
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
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.lifecycle.viewmodel.compose.hiltViewModel
import kotlinx.coroutines.delay
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.Glow
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import coil3.compose.AsyncImage
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
            val returnFocusEpisodeId by viewModel.returnFocusEpisodeMediaId.collectAsState()
            val activeReturnMediaId = returnFocusEpisodeId.takeIf { it > 0 }
            val focusEpisodeIndex =
                remember(selectedEpisodes, activeReturnMediaId, s.selectedSeasonIndex, s.resumeSeasonIndex, s.resumeEpisodeIndex) {
                    val fromReturn = activeReturnMediaId?.let { id -> selectedEpisodes.indexOfFirst { it.id == id } }
                    when {
                        fromReturn != null && fromReturn >= 0 -> fromReturn
                        s.selectedSeasonIndex == s.resumeSeasonIndex && selectedEpisodes.isNotEmpty() ->
                            s.resumeEpisodeIndex.coerceIn(0, selectedEpisodes.lastIndex)
                        selectedEpisodes.isNotEmpty() -> 0
                        else -> 0
                    }
                }
            val listState = rememberLazyListState()
            val episodeFocus = remember { FocusRequester() }
            val seasonFocus = remember { FocusRequester() }
            val backFocus = remember { FocusRequester() }
            LaunchedEffect(d.showKey) {
                val skipHeroFocus = returnFocusEpisodeId > 0
                delay(48)
                if (skipHeroFocus) return@LaunchedEffect
                backFocus.requestFocus()
                listState.scrollToItem(0)
            }
            LaunchedEffect(returnFocusEpisodeId, d.showKey) {
                if (returnFocusEpisodeId <= 0) return@LaunchedEffect
                val mediaId = returnFocusEpisodeId
                viewModel.ensureSeasonSelectedForMediaId(mediaId)
                delay(48)
                val ready = viewModel.state.value as? ShowDetailUiState.Ready ?: return@LaunchedEffect
                val eps = ready.seasons.getOrNull(ready.selectedSeasonIndex)?.episodes.orEmpty()
                val epIdx = eps.indexOfFirst { it.id == mediaId }
                if (epIdx < 0) {
                    viewModel.clearReturnFocusEpisodeRequest()
                    return@LaunchedEffect
                }
                val lazyIdx = firstEpisodeRowLazyIndex(ready, eps) ?: return@LaunchedEffect
                val target = lazyIdx + epIdx
                listState.scrollToItem(target.coerceAtLeast(0))
                episodeFocus.requestFocus()
                viewModel.clearReturnFocusEpisodeRequest()
            }
            val backdropUrl =
                resolveArtworkUrl(serverBase, d.backdropUrl, d.backdropPath, PlumImageSizes.BACKDROP_HERO)
            val posterUrl = resolveArtworkUrl(serverBase, d.posterUrl, d.posterPath, PlumImageSizes.POSTER_DETAIL)
            val resumeEp = s.seasons.getOrNull(s.resumeSeasonIndex)
                ?.episodes?.getOrNull(s.resumeEpisodeIndex)

            PlumDetailBackground(
                backdropUrl = backdropUrl,
                scrim = PlumScrims.backdropVertical,
            ) {
                // LazyColumn avoids rendering all episodes upfront for long seasons.
                LazyColumn(
                    state = listState,
                    modifier = Modifier.fillMaxSize(),
                    contentPadding = PaddingValues(horizontal = 36.dp, vertical = 24.dp),
                    verticalArrangement = Arrangement.spacedBy(12.dp),
                ) {
                    item {
                        PlumDetailHeroHeader(posterUrl = posterUrl) {
                            Text(
                                text = d.name,
                                style = PlumTheme.typography.headlineMedium,
                                color = Color.White,
                                fontWeight = FontWeight.Bold,
                            )

                            PlumMetadataChips(
                                values = buildList {
                                    d.firstAirDate.take(4).takeIf { it.isNotBlank() }?.let(::add)
                                    add("${d.numberOfSeasons} seasons")
                                    d.imdbRating?.let { add("\u2605 ${"%.1f".format(it)}") }
                                    addAll(d.genres.take(3))
                                    val totalEps = s.seasons.sumOf { it.episodes.size }
                                    val left = s.seasons.sumOf { se -> se.episodes.count { it.completed != true } }
                                    when {
                                        totalEps > 0 && left == 0 -> add("Fully watched")
                                        left > 0 && left < totalEps -> add("$left episode${if (left == 1) "" else "s"} left")
                                    }
                                },
                            )

                            if (d.overview.isNotBlank()) {
                                Text(
                                    text = d.overview,
                                    maxLines = 3,
                                    overflow = TextOverflow.Ellipsis,
                                    style = PlumTheme.typography.bodyMedium,
                                    color = Color.White.copy(alpha = 0.8f),
                                )
                            }

                            Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                                if (resumeEp != null) {
                                    val isResume = (resumeEp.progressSeconds ?: 0.0) > 0
                                    PlumActionButton(
                                        modifier = Modifier.focusRequester(backFocus),
                                        label = if (isResume) "Resume" else "Play",
                                        onClick = {
                                            val resume = (resumeEp.progressSeconds ?: 0.0).toFloat()
                                            onPlayEpisode(resumeEp.id, resume, d.libraryId, d.showKey)
                                        },
                                        leadingBadge = "\u25B6",
                                    )
                                    PlumActionButton("Back", onClick = onBack, variant = PlumButtonVariant.Ghost)
                                } else {
                                    PlumActionButton(
                                        modifier = Modifier.focusRequester(backFocus),
                                        label = "Back",
                                        onClick = onBack,
                                        variant = PlumButtonVariant.Secondary,
                                    )
                                }
                            }
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
                                    PlumActionButton(
                                        modifier =
                                            if (index == s.selectedSeasonIndex) {
                                                Modifier.focusRequester(seasonFocus)
                                            } else {
                                                Modifier
                                            },
                                        label = season.label,
                                        onClick = { viewModel.selectSeason(index) },
                                        variant = if (index == s.selectedSeasonIndex) PlumButtonVariant.Primary else PlumButtonVariant.Ghost,
                                    )
                                }
                            }
                        }
                    }

                    if (selectedEpisodes.isNotEmpty()) {
                        item {
                            val selSeason = s.seasons.getOrNull(s.selectedSeasonIndex)
                            val epCount = selectedEpisodes.size
                            val unwatched = selectedEpisodes.count { it.completed != true }
                            val summary = buildString {
                                append("$epCount episode${if (epCount == 1) "" else "s"}")
                                when {
                                    epCount > 0 && unwatched == 0 -> append(" · Fully watched")
                                    unwatched in 1 until epCount -> append(" · $unwatched left")
                                }
                            }
                            PlumSectionHeader(
                                "${selSeason?.label ?: "Episodes"} — $summary",
                            )
                        }
                        itemsIndexed(selectedEpisodes, key = { _, ep -> ep.id }) { index, ep ->
                            EpisodeRow(
                                modifier =
                                    if (index == focusEpisodeIndex) {
                                        Modifier.focusRequester(episodeFocus)
                                    } else {
                                        Modifier
                                    },
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
                                serverBase = serverBase,
                                cast =
                                    d.cast.orEmpty().map { member ->
                                        PlumCastMember(
                                            name = member.name,
                                            character = member.character,
                                            profilePath = member.profilePath,
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

/** LazyColumn index of the first episode row for the current season (after hero + season strip + section header). */
private fun firstEpisodeRowLazyIndex(s: ShowDetailUiState.Ready, selectedEpisodes: List<LibraryBrowseItemJson>): Int? {
    if (selectedEpisodes.isEmpty()) return null
    var nextIndex = 1
    if (s.seasons.isNotEmpty()) nextIndex += 2
    return nextIndex + 1
}

@Composable
private fun EpisodeRow(
    modifier: Modifier = Modifier,
    ep: LibraryBrowseItemJson,
    serverBase: String,
    onPlay: () -> Unit,
) {
    val palette = PlumTheme.palette
    val shape = RoundedCornerShape(10.dp)
    val thumbUrl =
        resolveArtworkUrl(serverBase, ep.thumbnailUrl, ep.thumbnailPath, PlumImageSizes.THUMB_SMALL)
            ?: resolveArtworkUrl(serverBase, ep.posterUrl, ep.posterPath, PlumImageSizes.THUMB_SMALL)
            ?: resolveArtworkUrl(serverBase, ep.showPosterUrl, ep.showPosterPath, PlumImageSizes.THUMB_SMALL)
    val watched = ep.completed == true
    val progressFrac =
        ((ep.progressPercent ?: 0.0) / 100.0).coerceIn(0.0, 1.0).toFloat()
    val rowProgress = if (watched) 1f else progressFrac

    Surface(
        onClick = onPlay,
        modifier = modifier.fillMaxWidth(),
        shape = ClickableSurfaceDefaults.shape(shape = shape),
        colors = ClickableSurfaceDefaults.colors(
            containerColor = palette.panel,
            contentColor = palette.text,
            focusedContainerColor = palette.panelAlt,
            focusedContentColor = palette.text,
            pressedContainerColor = palette.panelAlt,
            pressedContentColor = palette.text,
        ),
        scale = ClickableSurfaceDefaults.scale(focusedScale = 1f),
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
            Box(
                modifier = Modifier
                    .width(160.dp)
                    .height(90.dp)
                    .clip(RoundedCornerShape(topStart = 10.dp, bottomStart = 10.dp)),
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
                if (watched) {
                    Box(
                        modifier = Modifier
                            .fillMaxSize()
                            .background(Color.Black.copy(alpha = 0.45f)),
                    )
                    Text(
                        text = "\u2713",
                        style = PlumTheme.typography.labelSmall,
                        color = Color.White,
                        fontWeight = FontWeight.Bold,
                        modifier = Modifier
                            .align(Alignment.TopEnd)
                            .padding(4.dp)
                            .clip(RoundedCornerShape(4.dp))
                            .background(palette.accent.copy(alpha = 0.85f))
                            .padding(horizontal = 5.dp, vertical = 2.dp),
                    )
                }
                if (!watched && rowProgress > 0.02f) {
                    Box(
                        modifier = Modifier
                            .fillMaxWidth()
                            .height(3.dp)
                            .align(Alignment.BottomCenter),
                    ) {
                        Box(
                            modifier = Modifier
                                .fillMaxSize()
                                .background(Color.White.copy(alpha = 0.18f)),
                        )
                        Box(
                            modifier = Modifier
                                .fillMaxWidth(rowProgress)
                                .fillMaxHeight()
                                .background(palette.accent),
                        )
                    }
                }
            }

            Column(
                modifier = Modifier
                    .weight(1f)
                    .padding(horizontal = 16.dp, vertical = 10.dp),
                verticalArrangement = Arrangement.spacedBy(3.dp),
            ) {
                val se = ep.season
                val epn = ep.episode
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    if (se != null && epn != null) {
                        Text(
                            text = "S${se.toString().padStart(2, '0')}E${epn.toString().padStart(2, '0')}",
                            style = PlumTheme.typography.labelSmall,
                            color = if (watched) palette.accent else palette.muted,
                            fontWeight = FontWeight.SemiBold,
                        )
                    }
                    Text(
                        text = ep.title,
                        style = PlumTheme.typography.titleSmall,
                        color = if (watched) palette.textSecondary else palette.text,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.weight(1f, fill = false),
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
}
