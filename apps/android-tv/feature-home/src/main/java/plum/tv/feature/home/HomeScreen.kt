package plum.tv.feature.home

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.tv.material3.Text
import coil.compose.AsyncImage
import plum.tv.core.network.ContinueWatchingEntryJson
import plum.tv.core.network.MediaItemJson
import plum.tv.core.network.RecentlyAddedEntryJson
import plum.tv.core.ui.LocalServerBaseUrl
import plum.tv.core.ui.PlumActionButton
import plum.tv.core.ui.PlumButtonVariant
import plum.tv.core.ui.PlumPosterCard
import plum.tv.core.ui.PlumSectionHeader
import plum.tv.core.ui.PlumTheme
import plum.tv.core.ui.PlumTvMetrics
import plum.tv.core.ui.PlumImageSizes
import plum.tv.core.ui.resolveArtworkUrl
import plum.tv.core.ui.resolveImageUrl

@Composable
fun HomeRoute(
    onPlayMovie: (mediaId: Int, resumeSec: Float) -> Unit,
    onOpenShow: (libraryId: Int, showKey: String) -> Unit,
    viewModel: HomeViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    when (val s = state) {
        is HomeUiState.Loading -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            Text("Loading…", color = PlumTheme.palette.muted)
        }
        is HomeUiState.Error -> Column(
            modifier = Modifier.fillMaxSize().padding(48.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            Text(s.message, color = PlumTheme.palette.muted)
            PlumActionButton(
                label = "Retry",
                onClick = { viewModel.refresh() },
                variant = PlumButtonVariant.Primary,
            )
        }
        is HomeUiState.Ready -> HomeContent(
            continueWatching = s.continueWatching,
            recentlyAdded = s.recentlyAdded,
            onPlayMovie = onPlayMovie,
            onOpenShow = onOpenShow,
        )
    }
}

private fun formatRemainingTime(remainingSeconds: Double?): String? {
    val secs = remainingSeconds ?: return null
    if (secs <= 0) return null
    val totalMin = (secs / 60).toInt()
    if (totalMin <= 0) return null
    val hours = totalMin / 60
    val mins = totalMin % 60
    return if (hours > 0) "${hours}h ${mins}m remaining" else "${mins}m remaining"
}

@Composable
private fun HomeContent(
    continueWatching: List<ContinueWatchingEntryJson>,
    recentlyAdded: List<RecentlyAddedEntryJson>,
    onPlayMovie: (mediaId: Int, resumeSec: Float) -> Unit,
    onOpenShow: (libraryId: Int, showKey: String) -> Unit,
) {
    val metrics = PlumTheme.metrics
    val hero = continueWatching.firstOrNull()

    val recentlyAddedTv = recentlyAdded.filter { it.kind == "show" }
    val recentlyAddedMovies = recentlyAdded.filter { it.kind == "movie" }

    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        verticalArrangement = Arrangement.spacedBy(0.dp),
    ) {
        // Cinematic hero — first continue-watching item
        if (hero != null) {
            item {
                HeroSection(
                    entry = hero,
                    onPlay = {
                        when (hero.kind) {
                            "movie" -> onPlayMovie(hero.media.id, (hero.media.progressSeconds ?: 0.0).toFloat())
                            "show" -> {
                                val key = hero.showKey
                                val lib = hero.media.libraryId ?: 0
                                if (key != null) onOpenShow(lib, key)
                            }
                        }
                    },
                )
            }
        }

        // Continue Watching rail (skip the hero item since it's already featured)
        val cwRail = if (hero != null) continueWatching.drop(1) else continueWatching
        if (cwRail.isNotEmpty()) {
            item {
                HomeRail(
                    title = "Continue watching",
                    count = cwRail.size,
                    countSuffix = "active item",
                    metrics = metrics,
                    isLast = false,
                ) {
                    items(cwRail, key = { it.media.id }) { entry ->
                        val remaining = formatRemainingTime(entry.remainingSeconds)
                        val baseLabel = entry.episodeLabel ?: entry.showTitle
                        val subtitle = listOfNotNull(baseLabel, remaining).joinToString(" • ")
                        MediaEntryCard(
                            media = entry.media,
                            subtitle = subtitle.ifBlank { null },
                            progressPercent = entry.media.progressPercent,
                            onClick = {
                                when (entry.kind) {
                                    "movie" -> onPlayMovie(entry.media.id, (entry.media.progressSeconds ?: 0.0).toFloat())
                                    "show" -> {
                                        val key = entry.showKey
                                        val lib = entry.media.libraryId ?: 0
                                        if (key != null) onOpenShow(lib, key)
                                    }
                                }
                            },
                        )
                    }
                }
            }
        }

        // Recently added TV shows rail
        if (recentlyAddedTv.isNotEmpty()) {
            item {
                HomeRail(
                    title = "Recently added TV shows",
                    count = recentlyAddedTv.size,
                    countSuffix = "show",
                    metrics = metrics,
                    isLast = recentlyAddedMovies.isEmpty(),
                ) {
                    items(recentlyAddedTv, key = { "tv-${it.media.id}" }) { entry ->
                        MediaEntryCard(
                            media = entry.media,
                            subtitle = entry.episodeLabel ?: entry.showTitle,
                            progressPercent = null,
                            onClick = {
                                val key = entry.showKey
                                val lib = entry.media.libraryId ?: 0
                                if (key != null) onOpenShow(lib, key)
                            },
                        )
                    }
                }
            }
        }

        // Recently added movies rail
        if (recentlyAddedMovies.isNotEmpty()) {
            item {
                HomeRail(
                    title = "Recently added movies",
                    count = recentlyAddedMovies.size,
                    countSuffix = "film",
                    metrics = metrics,
                    isLast = true,
                ) {
                    items(recentlyAddedMovies, key = { "mv-${it.media.id}" }) { entry ->
                        MediaEntryCard(
                            media = entry.media,
                            subtitle = entry.media.releaseDate?.take(4),
                            progressPercent = null,
                            onClick = {
                                onPlayMovie(entry.media.id, 0f)
                            },
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun HomeRail(
    title: String,
    count: Int,
    countSuffix: String,
    metrics: PlumTvMetrics,
    isLast: Boolean,
    content: LazyListScope.() -> Unit,
) {
    Column(
        modifier = Modifier.padding(
            start = metrics.screenPadding.calculateLeftPadding(androidx.compose.ui.unit.LayoutDirection.Ltr),
            end = 0.dp,
            top = metrics.sectionGap,
            bottom = if (isLast) metrics.sectionGap else 0.dp,
        ),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth().padding(end = 36.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            PlumSectionHeader(title = title)
            Text(
                text = "$count $countSuffix${if (count == 1) "" else "s"}",
                style = PlumTheme.typography.bodySmall,
                color = PlumTheme.palette.muted,
            )
        }
        LazyRow(
            horizontalArrangement = Arrangement.spacedBy(metrics.cardGap),
            contentPadding = PaddingValues(end = 36.dp),
        ) {
            content()
        }
    }
}

@Composable
private fun HeroSection(
    entry: ContinueWatchingEntryJson,
    onPlay: () -> Unit,
) {
    val palette = PlumTheme.palette
    val serverBase = LocalServerBaseUrl.current
    val media = entry.media

    // Prefer backdrop for the wide hero, fall back to poster
    val heroImageUrl =
        resolveArtworkUrl(serverBase, media.backdropUrl, media.backdropPath, PlumImageSizes.BACKDROP_HERO)
            ?: resolveArtworkUrl(serverBase, media.posterUrl, media.posterPath, PlumImageSizes.POSTER_DETAIL)
            ?: resolveArtworkUrl(serverBase, media.showPosterUrl, media.showPosterPath, PlumImageSizes.POSTER_DETAIL)

    Box(
        modifier = Modifier
            .fillMaxWidth()
            .height(300.dp),
    ) {
        // Background artwork
        if (heroImageUrl != null) {
            AsyncImage(
                model = heroImageUrl,
                contentDescription = null,
                modifier = Modifier.fillMaxSize(),
                contentScale = ContentScale.Crop,
            )
        } else {
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .background(palette.panel),
            )
        }

        // Full scrim — darker at bottom, fades to semi-transparent at top
        Box(
            modifier = Modifier
                .fillMaxSize()
                .background(
                    Brush.verticalGradient(
                        0.0f to Color(0x44000000),
                        0.45f to Color(0x88000000),
                        1.0f to Color(0xEE000000),
                    ),
                ),
        )

        // Left-side content: title + metadata + play button
        Column(
            modifier = Modifier
                .align(Alignment.BottomStart)
                .padding(horizontal = 36.dp, vertical = 28.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            // Show title
            Text(
                text = media.title,
                style = PlumTheme.typography.headlineMedium,
                color = Color.White,
                fontWeight = FontWeight.Bold,
            )

            // Subtitle (episode label or show name for TV)
            val subtitle = entry.episodeLabel ?: entry.showTitle
            if (!subtitle.isNullOrBlank()) {
                Text(
                    text = subtitle,
                    style = PlumTheme.typography.bodyMedium,
                    color = Color.White.copy(alpha = 0.7f),
                )
            }

            Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                PlumActionButton(
                    label = if ((media.progressSeconds ?: 0.0) > 0) "Resume" else "Play",
                    onClick = onPlay,
                    variant = PlumButtonVariant.Primary,
                )
            }
        }
    }
}

@Composable
private fun MediaEntryCard(
    media: MediaItemJson,
    subtitle: String?,
    progressPercent: Double?,
    onClick: () -> Unit,
) {
    val serverBase = LocalServerBaseUrl.current
    PlumPosterCard(
        title = media.title,
        subtitle = subtitle,
        imageUrl =
            resolveArtworkUrl(serverBase, media.posterUrl, media.posterPath, PlumImageSizes.POSTER_GRID)
                ?: resolveArtworkUrl(serverBase, media.showPosterUrl, media.showPosterPath, PlumImageSizes.POSTER_GRID)
                ?: media.thumbnailUrl?.takeIf { it.isNotBlank() }?.let { resolveImageUrl(serverBase, it) }
                ?: media.thumbnailPath?.takeIf { it.isNotBlank() }?.let { resolveImageUrl(serverBase, it) },
        onClick = onClick,
        compact = true,
        progressPercent = progressPercent,
    )
}
