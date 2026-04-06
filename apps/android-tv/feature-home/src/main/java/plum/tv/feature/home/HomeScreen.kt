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
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.Modifier
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.lifecycle.viewmodel.compose.hiltViewModel
import androidx.tv.material3.Text
import coil3.compose.AsyncImage
import coil3.request.CachePolicy
import coil3.request.ImageRequest
import coil3.request.crossfade
import plum.tv.core.network.ContinueWatchingEntryJson
import plum.tv.core.network.MediaItemJson
import plum.tv.core.network.RecentlyAddedEntryJson
import plum.tv.core.ui.LocalServerBaseUrl
import plum.tv.core.ui.PlumActionButton
import plum.tv.core.ui.PlumButtonVariant
import plum.tv.core.ui.PlumMetadataChips
import plum.tv.core.ui.PlumPosterCard
import plum.tv.core.ui.PlumScrims
import plum.tv.core.ui.PlumSectionHeader
import plum.tv.core.ui.PlumStatePanel
import plum.tv.core.ui.PlumTheme
import plum.tv.core.ui.PlumTvMetrics
import plum.tv.core.ui.PlumImageSizes
import plum.tv.core.ui.resolveArtworkUrl
import plum.tv.core.ui.resolveImageUrl

@Composable
fun HomeRoute(
    onPlayMedia: (
        mediaId: Int,
        resumeSec: Float,
        libraryId: Int?,
        showKey: String?,
        displayTitle: String?,
        displaySubtitle: String?,
    ) -> Unit,
    onOpenShow: (libraryId: Int, showKey: String) -> Unit,
    viewModel: HomeViewModel = hiltViewModel(),
) {
    LaunchedEffect(Unit) {
        viewModel.onAppear()
    }
    val state by viewModel.state.collectAsState()
    when (val s = state) {
        is HomeUiState.Loading -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            PlumStatePanel(
                title = "Loading",
                message = "Fetching your dashboard\u2026",
            )
        }
        is HomeUiState.Error -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            PlumStatePanel(
                title = "Something went wrong",
                message = s.message,
                actions = {
                    PlumActionButton(
                        label = "Retry",
                        onClick = { viewModel.refresh() },
                        variant = PlumButtonVariant.Primary,
                    )
                },
            )
        }
        is HomeUiState.Ready -> HomeContent(
            continueWatching = s.continueWatching,
            recentlyAdded = s.recentlyAdded,
            onPlayMedia = onPlayMedia,
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
    onPlayMedia: (
        mediaId: Int,
        resumeSec: Float,
        libraryId: Int?,
        showKey: String?,
        displayTitle: String?,
        displaySubtitle: String?,
    ) -> Unit,
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
                        val resume = (hero.media.progressSeconds ?: 0.0).toFloat()
                        when (hero.kind) {
                            "movie" ->
                                onPlayMedia(
                                    hero.media.id,
                                    resume,
                                    hero.media.libraryId,
                                    null,
                                    hero.media.title,
                                    hero.media.releaseDate?.take(4)?.takeIf { it.length == 4 },
                                )
                            "show" ->
                                onPlayMedia(
                                    hero.media.id,
                                    resume,
                                    hero.media.libraryId,
                                    hero.showKey,
                                    null,
                                    null,
                                )
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
                    metrics = metrics,
                    isLast = false,
                ) {
                    items(cwRail, key = { it.media.id }) { entry ->
                        val remaining = formatRemainingTime(entry.remainingSeconds)
                        // Show entries use showTitle as the card title; subtitle is episode context only.
                        val baseLabel =
                            if (entry.kind == "show") {
                                entry.episodeLabel?.takeIf { it.isNotBlank() }
                            } else {
                                entry.episodeLabel ?: entry.showTitle
                            }
                        val subtitle = listOfNotNull(baseLabel, remaining).joinToString(" • ")
                        MediaEntryCard(
                            media = entry.media,
                            preferShowPoster = entry.kind == "show",
                            title =
                                if (entry.kind == "show") {
                                    entry.showTitle?.takeIf { it.isNotBlank() }
                                } else {
                                    null
                                },
                            subtitle = subtitle.ifBlank { null },
                            progressPercent = entry.media.progressPercent,
                            onClick = {
                                val resume = (entry.media.progressSeconds ?: 0.0).toFloat()
                                when (entry.kind) {
                                    "movie" ->
                                        onPlayMedia(
                                            entry.media.id,
                                            resume,
                                            entry.media.libraryId,
                                            null,
                                            entry.media.title,
                                            entry.media.releaseDate?.take(4)?.takeIf { it.length == 4 },
                                        )
                                    "show" ->
                                        onPlayMedia(
                                            entry.media.id,
                                            resume,
                                            entry.media.libraryId,
                                            entry.showKey,
                                            null,
                                            null,
                                        )
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
                    metrics = metrics,
                    isLast = recentlyAddedMovies.isEmpty(),
                ) {
                    items(recentlyAddedTv, key = { "tv-${it.media.id}" }) { entry ->
                        MediaEntryCard(
                            media = entry.media,
                            preferShowPoster = true,
                            title = entry.showTitle?.takeIf { it.isNotBlank() },
                            subtitle = entry.episodeLabel?.takeIf { it.isNotBlank() },
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
                    metrics = metrics,
                    isLast = true,
                ) {
                    items(recentlyAddedMovies, key = { "mv-${it.media.id}" }) { entry ->
                        MediaEntryCard(
                            media = entry.media,
                            preferShowPoster = false,
                            subtitle = entry.media.releaseDate?.take(4),
                            progressPercent = null,
                            onClick = {
                                onPlayMedia(
                                    entry.media.id,
                                    0f,
                                    entry.media.libraryId,
                                    null,
                                    entry.media.title,
                                    entry.media.releaseDate?.take(4)?.takeIf { it.length == 4 },
                                )
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
    metrics: PlumTvMetrics,
    isLast: Boolean,
    content: LazyListScope.() -> Unit,
) {
    // Start inset lives on the LazyRow's contentPadding (not the Column) so the LazyRow's clip
    // boundary extends to the content area's left edge — giving the first focused card room to
    // scale without being cropped by the scroll container's clip.
    val startInset = metrics.screenPadding.calculateLeftPadding(androidx.compose.ui.unit.LayoutDirection.Ltr)
    Column(
        modifier = Modifier.padding(
            top = metrics.sectionGap,
            bottom = if (isLast) metrics.sectionGap else 0.dp,
        ),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        PlumSectionHeader(
            title = title,
            modifier = Modifier.padding(start = startInset, end = 28.dp),
        )
        LazyRow(
            horizontalArrangement = Arrangement.spacedBy(metrics.cardGap),
            contentPadding = PaddingValues(start = startInset, end = 28.dp),
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

    // Prefer backdrop for the wide hero; for shows, series poster before episode still/poster (no thumbnails).
    val heroImageUrl =
        resolveArtworkUrl(serverBase, media.backdropUrl, media.backdropPath, PlumImageSizes.BACKDROP_HERO)
            ?: if (entry.kind == "show") {
                resolveArtworkUrl(serverBase, media.showPosterUrl, media.showPosterPath, PlumImageSizes.BACKDROP_HERO)
                    ?: resolveArtworkUrl(serverBase, media.posterUrl, media.posterPath, PlumImageSizes.POSTER_DETAIL)
            } else {
                resolveArtworkUrl(serverBase, media.posterUrl, media.posterPath, PlumImageSizes.POSTER_DETAIL)
                    ?: resolveArtworkUrl(serverBase, media.showPosterUrl, media.showPosterPath, PlumImageSizes.POSTER_DETAIL)
            }

    val metrics = PlumTheme.metrics
    val context = LocalContext.current
    val density = LocalDensity.current
    val screenWidthDp = LocalConfiguration.current.screenWidthDp
    val heroImageRequest =
        remember(heroImageUrl, screenWidthDp, metrics.heroHeight, context) {
            val url = heroImageUrl ?: return@remember null
            val wPx = with(density) { screenWidthDp.dp.roundToPx().coerceAtLeast(1) }
            val hPx = with(density) { metrics.heroHeight.roundToPx().coerceAtLeast(1) }
            ImageRequest.Builder(context)
                .data(url)
                .size(wPx, hPx)
                .crossfade(300)
                .memoryCachePolicy(CachePolicy.ENABLED)
                .diskCachePolicy(CachePolicy.ENABLED)
                .build()
        }
    Box(
        modifier = Modifier
            .fillMaxWidth()
            .height(metrics.heroHeight),
    ) {
        // Background artwork
        if (heroImageRequest != null) {
            AsyncImage(
                model = heroImageRequest,
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
                .background(PlumScrims.heroBottom),
        )

        Column(
            modifier = Modifier
                .align(Alignment.BottomStart)
                .padding(horizontal = 28.dp, vertical = 24.dp)
                .widthIn(max = 640.dp),
            verticalArrangement = Arrangement.spacedBy(6.dp),
        ) {
            Text(
                text = "Continue watching",
                style = PlumTheme.typography.labelMedium,
                color = PlumTheme.palette.accent.copy(alpha = 0.85f),
                fontWeight = FontWeight.Bold,
            )
            val heroTitle =
                if (entry.kind == "show") {
                    entry.showTitle?.takeIf { it.isNotBlank() } ?: media.title
                } else {
                    media.title
                }
            Text(
                text = heroTitle,
                style = PlumTheme.typography.displaySmall,
                color = Color.White,
                fontWeight = FontWeight.Bold,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )

            val subtitle =
                when {
                    !entry.episodeLabel.isNullOrBlank() -> entry.episodeLabel
                    entry.kind != "show" && !entry.showTitle.isNullOrBlank() -> entry.showTitle
                    else -> null
                }
            if (!subtitle.isNullOrBlank()) {
                Text(
                    text = subtitle,
                    style = PlumTheme.typography.titleSmall,
                    color = Color.White.copy(alpha = 0.8f),
                    fontWeight = FontWeight.Medium,
                )
            }

            val chips = buildList {
                media.releaseDate?.take(4)?.takeIf { it.length == 4 }?.let { add(it) }
                (media.voteAverage ?: media.showVoteAverage)?.takeIf { it > 0 }?.let {
                    add("%.1f".format(it))
                }
                formatRemainingTime(entry.remainingSeconds)?.let { add(it) }
            }
            if (chips.isNotEmpty()) {
                PlumMetadataChips(values = chips)
            }

            val overview = media.overview
            if (!overview.isNullOrBlank()) {
                Text(
                    text = overview,
                    style = PlumTheme.typography.bodySmall,
                    color = Color.White.copy(alpha = 0.58f),
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                )
            }

            Row(
                modifier = Modifier.padding(top = 6.dp),
                horizontalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                PlumActionButton(
                    label = if ((media.progressSeconds ?: 0.0) > 0) "Resume" else "Play",
                    onClick = onPlay,
                    variant = PlumButtonVariant.Primary,
                    leadingIcon = Icons.Filled.PlayArrow,
                )
            }
        }
    }
}

@Composable
private fun MediaEntryCard(
    media: MediaItemJson,
    preferShowPoster: Boolean,
    title: String? = null,
    subtitle: String?,
    progressPercent: Double?,
    onClick: () -> Unit,
) {
    val serverBase = LocalServerBaseUrl.current
    val showArt =
        resolveArtworkUrl(serverBase, media.showPosterUrl, media.showPosterPath, PlumImageSizes.POSTER_GRID_COMPACT)
    val itemArt =
        resolveArtworkUrl(serverBase, media.posterUrl, media.posterPath, PlumImageSizes.POSTER_GRID_COMPACT)
    val imageUrl =
        if (preferShowPoster) {
            showArt ?: itemArt
        } else {
            itemArt ?: showArt
        }
            ?: media.thumbnailUrl?.takeIf { it.isNotBlank() }?.let { resolveImageUrl(serverBase, it) }
            ?: media.thumbnailPath?.takeIf { it.isNotBlank() }?.let { resolveImageUrl(serverBase, it) }
    PlumPosterCard(
        title = title?.takeIf { it.isNotBlank() } ?: media.title,
        subtitle = subtitle,
        imageUrl = imageUrl,
        onClick = onClick,
        compact = true,
        progressPercent = progressPercent,
    )
}
