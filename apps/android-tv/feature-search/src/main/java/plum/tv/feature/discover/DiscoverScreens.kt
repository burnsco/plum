package plum.tv.feature.discover

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
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
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.itemsIndexed
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.lifecycle.viewmodel.compose.hiltViewModel
import androidx.tv.material3.Text
import plum.tv.core.network.DiscoverGenreJson
import plum.tv.core.network.DiscoverItemJson
import plum.tv.core.network.DownloadItemJson
import plum.tv.core.ui.LaunchedTvFocusTo
import plum.tv.core.ui.LocalServerBaseUrl
import plum.tv.core.ui.PlumActionButton
import plum.tv.core.ui.PlumButtonVariant
import plum.tv.core.ui.PlumDetailBackground
import plum.tv.core.ui.PlumDetailHeroHeader
import plum.tv.core.ui.PlumImageSizes
import plum.tv.core.ui.PlumMetadataChips
import plum.tv.core.ui.PlumPanel
import plum.tv.core.ui.PlumPosterCard
import plum.tv.core.ui.PlumScreenPadding
import plum.tv.core.ui.PlumScreenTitle
import plum.tv.core.ui.PlumScrims
import plum.tv.core.ui.PlumSectionHeader
import plum.tv.core.ui.PlumStatePanel
import plum.tv.core.ui.PlumTheme
import plum.tv.core.ui.resolveArtworkUrl

private val discoverCategoryOptions =
    listOf(
        DiscoverCategoryOption("trending", "Trending"),
        DiscoverCategoryOption("popular-movies", "Popular Movies"),
        DiscoverCategoryOption("popular-tv", "Popular TV"),
        DiscoverCategoryOption("now-playing", "Now Playing"),
        DiscoverCategoryOption("upcoming", "Upcoming"),
        DiscoverCategoryOption("on-the-air", "On The Air"),
        DiscoverCategoryOption("top-rated", "Top Rated"),
    )

private data class DiscoverCategoryOption(val id: String, val label: String)

// ── Main Discover landing ────────────────────────────────────────────────────

@Composable
fun DiscoverRoute(
    onOpenBrowse: (category: String?, mediaType: String?, genreId: Int?) -> Unit,
    onOpenTitle: (mediaType: String, tmdbId: Int) -> Unit,
    viewModel: DiscoverViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val serverBase = LocalServerBaseUrl.current

    when (val s = state) {
        is DiscoverUiState.Loading -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            PlumStatePanel(
                title = "Loading discover",
                message = "Pulling in shelves and genres\u2026",
            )
        }
        is DiscoverUiState.Error -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            PlumStatePanel(
                title = "Could not load discover",
                message = s.message,
                actions = {
                    PlumActionButton("Retry", onClick = { viewModel.refresh() }, leadingBadge = "R")
                },
            )
        }
        is DiscoverUiState.Ready -> {
            val leadFocus = remember { FocusRequester() }
            LaunchedTvFocusTo(
                s.discover.shelves.joinToString(",") { it.id },
                s.genres.movieGenres.size,
                s.genres.tvGenres.size,
                focusRequester = leadFocus,
            )
            LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = PlumScreenPadding(),
                verticalArrangement = Arrangement.spacedBy(20.dp),
            ) {
                item(key = "header") {
                    DiscoverHeader(onOpenBrowse = onOpenBrowse, leadFocus = leadFocus)
                }
                if (s.genres.movieGenres.isNotEmpty() || s.genres.tvGenres.isNotEmpty()) {
                    item(key = "genres") {
                        DiscoverGenres(
                            movieGenres = s.genres.movieGenres,
                            tvGenres = s.genres.tvGenres,
                            onOpenBrowse = onOpenBrowse,
                        )
                    }
                }
                items(s.discover.shelves, key = { it.id }) { shelf ->
                    DiscoverShelfRow(
                        title = shelf.title,
                        items = shelf.items,
                        serverBase = serverBase,
                        onOpenTitle = onOpenTitle,
                    )
                }
            }
        }
    }
}

@Composable
private fun DiscoverHeader(
    onOpenBrowse: (String?, String?, Int?) -> Unit,
    leadFocus: FocusRequester,
) {
    Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
        PlumScreenTitle(title = "Discover", subtitle = "Trending titles, genres, and curated shelves.")
        LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            item {
                PlumActionButton(
                    modifier = Modifier.focusRequester(leadFocus),
                    label = "Browse All",
                    onClick = { onOpenBrowse(null, null, null) },
                    leadingBadge = "B",
                )
            }
            items(discoverCategoryOptions, key = { it.id }) { option ->
                PlumActionButton(
                    label = option.label,
                    onClick = { onOpenBrowse(option.id, null, null) },
                    variant = PlumButtonVariant.Secondary,
                )
            }
        }
    }
}

@Composable
private fun DiscoverGenres(
    movieGenres: List<DiscoverGenreJson>,
    tvGenres: List<DiscoverGenreJson>,
    onOpenBrowse: (String?, String?, Int?) -> Unit,
) {
    Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
        PlumSectionHeader(title = "Genres")
        if (movieGenres.isNotEmpty()) {
            DiscoverGenreRow("Movies", movieGenres, "movie", onOpenBrowse)
        }
        if (tvGenres.isNotEmpty()) {
            DiscoverGenreRow("TV Shows", tvGenres, "tv", onOpenBrowse)
        }
    }
}

@Composable
private fun DiscoverGenreRow(
    title: String,
    genres: List<DiscoverGenreJson>,
    mediaType: String,
    onOpenBrowse: (String?, String?, Int?) -> Unit,
) {
    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Text(
            text = title,
            style = PlumTheme.typography.labelLarge,
            color = PlumTheme.palette.textSecondary,
        )
        LazyRow(horizontalArrangement = Arrangement.spacedBy(7.dp)) {
            items(genres, key = { it.id }) { genre ->
                PlumActionButton(
                    label = genre.name,
                    onClick = { onOpenBrowse(null, mediaType, genre.id) },
                    variant = PlumButtonVariant.Secondary,
                )
            }
        }
    }
}

@Composable
private fun DiscoverShelfRow(
    title: String,
    items: List<DiscoverItemJson>,
    serverBase: String,
    onOpenTitle: (mediaType: String, tmdbId: Int) -> Unit,
) {
    if (items.isEmpty()) return
    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
        PlumSectionHeader(title = title)
        LazyRow(horizontalArrangement = Arrangement.spacedBy(PlumTheme.metrics.cardGap)) {
            items(items, key = { "${it.mediaType}-${it.tmdbId}" }) { item ->
                DiscoverPosterCard(
                    item = item,
                    serverBase = serverBase,
                    onClick = { onOpenTitle(item.mediaType, item.tmdbId) },
                )
            }
        }
    }
}

@Composable
private fun DiscoverPosterCard(
    item: DiscoverItemJson,
    serverBase: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    focusedScale: Float? = null,
) {
    val posterUrl = resolveArtworkUrl(serverBase, null, item.posterPath, PlumImageSizes.POSTER_GRID)
    PlumPosterCard(
        title = item.title,
        subtitle = discoverItemSubtitle(item),
        imageUrl = posterUrl,
        onClick = onClick,
        modifier = modifier,
        compact = true,
        focusedScale = focusedScale,
    )
}

// ── Browse grid with inline filters ──────────────────────────────────────────

@Composable
fun DiscoverBrowseRoute(
    category: String?,
    mediaType: String?,
    genreId: Int?,
    onOpenTitle: (mediaType: String, tmdbId: Int) -> Unit,
    onBack: () -> Unit,
    viewModel: DiscoverBrowseViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val serverBase = LocalServerBaseUrl.current

    LaunchedEffect(category, mediaType, genreId) {
        viewModel.refresh(category, mediaType, genreId)
    }

    when (val s = state) {
        is DiscoverBrowseUiState.Loading -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            PlumStatePanel(
                title = "Loading browse",
                message = "Fetching titles\u2026",
            )
        }
        is DiscoverBrowseUiState.Error -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            PlumStatePanel(
                title = "Could not load browse",
                message = s.message,
                actions = {
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        PlumActionButton("Retry", onClick = { viewModel.refresh(category, mediaType, genreId) }, leadingBadge = "R")
                        PlumActionButton("Back", onClick = onBack, variant = PlumButtonVariant.Ghost)
                    }
                },
            )
        }
        is DiscoverBrowseUiState.Ready -> {
            val metrics = PlumTheme.metrics
            val gridFirstFocus = remember { FocusRequester() }
            val backFocus = remember { FocusRequester() }
            LaunchedTvFocusTo(
                s.title,
                s.category,
                s.mediaType,
                s.genre?.id,
                s.items.firstOrNull()?.let { "${it.mediaType}-${it.tmdbId}" },
                focusRequester = if (s.items.isEmpty()) backFocus else gridFirstFocus,
            )
            val minCell = remember(metrics) { metrics.posterCompactWidth + metrics.cardGap }

            Column(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(PlumScreenPadding()),
                verticalArrangement = Arrangement.spacedBy(10.dp),
            ) {
                DiscoverBrowseToolbar(
                    title = s.title,
                    totalResults = s.totalResults,
                    refreshing = s.refreshing,
                    category = s.category,
                    mediaType = s.mediaType,
                    genreId = s.genre?.id,
                    genres = s.genres,
                    onBack = onBack,
                    onApplyFilter = { c, m, g -> viewModel.refresh(c, m, g) },
                    backFocusRequester = backFocus,
                )
                Box(modifier = Modifier.weight(1f).fillMaxWidth()) {
                    if (s.items.isEmpty()) {
                        Box(
                            modifier = Modifier.fillMaxSize(),
                            contentAlignment = Alignment.Center,
                        ) {
                            PlumStatePanel(
                                modifier = Modifier.fillMaxWidth(),
                                title = "No titles found",
                                message = "Try a different shelf, type, or genre.",
                            )
                        }
                    } else {
                        LazyVerticalGrid(
                            columns = GridCells.Adaptive(minSize = minCell),
                            modifier = Modifier.fillMaxSize(),
                            horizontalArrangement = Arrangement.spacedBy(metrics.cardGap),
                            verticalArrangement = Arrangement.spacedBy(metrics.cardGap),
                            contentPadding = PaddingValues(bottom = 24.dp),
                        ) {
                            itemsIndexed(
                                s.items,
                                key = { _, item -> "${item.mediaType}-${item.tmdbId}" },
                            ) { index, item ->
                                DiscoverPosterCard(
                                    item = item,
                                    serverBase = serverBase,
                                    onClick = { onOpenTitle(item.mediaType, item.tmdbId) },
                                    modifier = if (index == 0) Modifier.focusRequester(gridFirstFocus) else Modifier,
                                    focusedScale = 1f,
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun DiscoverBrowseToolbar(
    title: String,
    totalResults: Int,
    refreshing: Boolean,
    category: String?,
    mediaType: String?,
    genreId: Int?,
    genres: List<DiscoverGenreJson>,
    onBack: () -> Unit,
    onApplyFilter: (String?, String?, Int?) -> Unit,
    backFocusRequester: FocusRequester,
) {
    val palette = PlumTheme.palette
    PlumPanel(contentPadding = PaddingValues(horizontal = 12.dp, vertical = 8.dp)) {
        Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(12.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                PlumActionButton(
                    modifier = Modifier.focusRequester(backFocusRequester),
                    label = "Back",
                    onClick = onBack,
                    variant = PlumButtonVariant.Secondary,
                )
                Text(
                    text = title,
                    style = PlumTheme.typography.titleMedium,
                    color = palette.text,
                    fontWeight = FontWeight.SemiBold,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.weight(1f),
                )
                AnimatedVisibility(visible = refreshing, enter = fadeIn(), exit = fadeOut()) {
                    Box(
                        modifier = Modifier
                            .size(8.dp)
                            .clip(CircleShape)
                            .background(palette.accent),
                    )
                }
                Text(
                    text = "$totalResults titles",
                    style = PlumTheme.typography.labelMedium,
                    color = palette.muted,
                )
            }
            BrowseFilterRow(label = "Shelf") {
                LazyRow(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    item {
                        FilterChip(
                            label = "All",
                            selected = category == null && genreId == null,
                            onClick = { onApplyFilter(null, mediaType, null) },
                        )
                    }
                    items(discoverCategoryOptions, key = { it.id }) { option ->
                        FilterChip(
                            label = option.label,
                            selected = category == option.id,
                            onClick = { onApplyFilter(option.id, mediaType, null) },
                        )
                    }
                }
            }
            BrowseFilterRow(label = "Type") {
                Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    FilterChip(
                        label = "Any",
                        selected = mediaType == null,
                        onClick = { onApplyFilter(category, null, genreId) },
                    )
                    FilterChip(
                        label = "Movies",
                        selected = mediaType == "movie",
                        onClick = { onApplyFilter(category, "movie", genreId) },
                    )
                    FilterChip(
                        label = "TV",
                        selected = mediaType == "tv",
                        onClick = { onApplyFilter(category, "tv", genreId) },
                    )
                }
            }
            if (genres.isNotEmpty()) {
                BrowseFilterRow(label = "Genre") {
                    LazyRow(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                        item {
                            FilterChip(
                                label = "Any",
                                selected = genreId == null,
                                onClick = { onApplyFilter(category, mediaType, null) },
                            )
                        }
                        items(genres, key = { it.id }) { g ->
                            FilterChip(
                                label = g.name,
                                selected = genreId == g.id,
                                onClick = { onApplyFilter(category, mediaType, g.id) },
                            )
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun FilterChip(
    label: String,
    selected: Boolean,
    onClick: () -> Unit,
) {
    PlumActionButton(
        label = label,
        onClick = onClick,
        variant = if (selected) PlumButtonVariant.Primary else PlumButtonVariant.Ghost,
    )
}

@Composable
private fun BrowseFilterRow(
    label: String,
    content: @Composable () -> Unit,
) {
    Row(
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        Text(
            text = label,
            style = PlumTheme.typography.labelSmall,
            color = PlumTheme.palette.muted,
            modifier = Modifier.width(42.dp),
        )
        content()
    }
}

// ── Title detail ─────────────────────────────────────────────────────────────

@Composable
fun DiscoverDetailRoute(
    mediaType: String,
    tmdbId: Int,
    onOpenLibrary: (libraryId: Int, showKey: String?) -> Unit,
    onBack: () -> Unit,
    onOpenSettings: () -> Unit,
    viewModel: DiscoverDetailViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val serverBase = LocalServerBaseUrl.current

    LaunchedEffect(mediaType, tmdbId) {
        viewModel.refresh(mediaType, tmdbId)
    }

    when (val s = state) {
        is DiscoverDetailUiState.Loading -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            PlumStatePanel(
                title = "Loading title",
                message = "Fetching artwork and metadata\u2026",
            )
        }
        is DiscoverDetailUiState.Error -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            PlumStatePanel(
                title = "Could not load title",
                message = s.message,
                actions = {
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        PlumActionButton("Retry", onClick = { viewModel.refresh(mediaType, tmdbId) }, leadingBadge = "R")
                        PlumActionButton("Back", onClick = onBack, variant = PlumButtonVariant.Ghost)
                    }
                },
            )
        }
        is DiscoverDetailUiState.Ready -> {
            val d = s.details
            val primaryActionFocus = remember(mediaType, tmdbId) { FocusRequester() }
            LaunchedTvFocusTo(mediaType, tmdbId, d.title, focusRequester = primaryActionFocus)
            val backdropUrl = resolveArtworkUrl(serverBase, null, d.backdropPath, PlumImageSizes.BACKDROP_HERO)
            val posterUrl = resolveArtworkUrl(serverBase, null, d.posterPath, PlumImageSizes.POSTER_DETAIL)
            val primaryMatch = d.libraryMatches.firstOrNull()
            val isConfigured = d.acquisition?.isConfigured != false
            val canAdd = d.acquisition?.canAdd == true
            val addLabel = remember(d.acquisition?.state, isConfigured) {
                when (d.acquisition?.state) {
                    "available" -> "In Library"
                    "downloading" -> "Downloading\u2026"
                    "added" -> "Added"
                    else -> if (isConfigured) "Add" else "Unavailable"
                }
            }

            PlumDetailBackground(
                backdropUrl = backdropUrl,
                scrim = PlumScrims.backdropVertical,
            ) {
                Column(
                    modifier = Modifier
                        .fillMaxSize()
                        .verticalScroll(rememberScrollState())
                        .padding(horizontal = 36.dp, vertical = 28.dp),
                    verticalArrangement = Arrangement.spacedBy(18.dp),
                ) {
                    PlumDetailHeroHeader(posterUrl = posterUrl) {
                        Text(
                            text = d.title,
                            style = PlumTheme.typography.headlineLarge,
                            color = Color.White,
                            fontWeight = FontWeight.Bold,
                        )
                        PlumMetadataChips(
                            values = buildList {
                                val year = d.releaseDate?.take(4) ?: d.firstAirDate?.take(4)
                                if (!year.isNullOrBlank()) add(year)
                                d.runtime?.takeIf { it > 0 }?.let { add("$it min") }
                                d.numberOfSeasons?.takeIf { it > 0 }?.let { add("$it seasons") }
                                d.voteAverage?.let { add("TMDb ${"%.1f".format(it)}") }
                                d.imdbRating?.let { add("IMDb ${"%.1f".format(it)}") }
                                addAll(d.genres.take(3))
                            },
                        )
                        if (d.overview.isNotBlank()) {
                            Text(
                                text = d.overview,
                                maxLines = 6,
                                overflow = TextOverflow.Ellipsis,
                                style = PlumTheme.typography.bodyMedium,
                                color = Color.White.copy(alpha = 0.85f),
                            )
                        }
                        Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                            if (primaryMatch != null) {
                                PlumActionButton(
                                    modifier = Modifier.focusRequester(primaryActionFocus),
                                    label = "Open in Library",
                                    onClick = { onOpenLibrary(primaryMatch.libraryId, primaryMatch.showKey) },
                                )
                            } else {
                                PlumActionButton(
                                    modifier = Modifier.focusRequester(primaryActionFocus),
                                    label = addLabel,
                                    onClick = {
                                        when {
                                            !isConfigured -> onOpenSettings()
                                            canAdd -> viewModel.addTitle(mediaType, tmdbId)
                                        }
                                    },
                                    variant = PlumButtonVariant.Primary,
                                )
                            }
                            PlumActionButton("Back", onClick = onBack, variant = PlumButtonVariant.Secondary)
                        }
                    }
                    if (d.libraryMatches.size > 1) {
                        PlumPanel {
                            Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                                PlumSectionHeader("Also available in")
                                d.libraryMatches.drop(1).forEach { match ->
                                    PlumActionButton(
                                        label = match.libraryName,
                                        onClick = { onOpenLibrary(match.libraryId, match.showKey) },
                                        variant = PlumButtonVariant.Secondary,
                                    )
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}

// ── Downloads ────────────────────────────────────────────────────────────────

@Composable
fun DownloadsRoute(
    onOpenSettings: () -> Unit,
    viewModel: DownloadsViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()

    when (val s = state) {
        is DownloadsUiState.Loading -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            PlumStatePanel(
                title = "Loading downloads",
                message = "Checking the Radarr and Sonarr queues\u2026",
            )
        }
        is DownloadsUiState.Error -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            PlumStatePanel(
                title = "Could not load downloads",
                message = s.message,
                actions = {
                    PlumActionButton("Retry", onClick = { viewModel.refresh() }, leadingBadge = "R")
                },
            )
        }
        is DownloadsUiState.Ready -> {
            val refreshFocus = remember { FocusRequester() }
            LaunchedTvFocusTo(focusRequester = refreshFocus)
            Column(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(PlumScreenPadding()),
                verticalArrangement = Arrangement.spacedBy(18.dp),
            ) {
                Row(
                    horizontalArrangement = Arrangement.spacedBy(14.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    PlumScreenTitle(
                        title = "Downloads",
                        subtitle = "Live queue from Radarr and Sonarr.",
                    )
                    Spacer(Modifier.weight(1f))
                    PlumActionButton(
                        modifier = Modifier.focusRequester(refreshFocus),
                        label = "Refresh",
                        onClick = { viewModel.refresh() },
                        variant = PlumButtonVariant.Secondary,
                    )
                }

                when {
                    !s.configured ->
                        PlumStatePanel(
                            title = "Media stack not configured",
                            message = "Connect Radarr and Sonarr on the server to see download activity.",
                            actions = {
                                PlumActionButton("Open Settings", onClick = onOpenSettings)
                            },
                        )
                    s.items.isEmpty() ->
                        PlumStatePanel(
                            title = "No active downloads",
                            message = "Items you add from Discover will show up here while downloading.",
                        )
                    else ->
                        LazyColumn(
                            verticalArrangement = Arrangement.spacedBy(10.dp),
                            contentPadding = PaddingValues(bottom = 24.dp),
                        ) {
                            items(s.items, key = { it.id }) { item ->
                                DownloadRow(item)
                            }
                        }
                }
            }
        }
    }
}

@Composable
private fun DownloadRow(item: DownloadItemJson) {
    val palette = PlumTheme.palette
    val progress = item.progress?.coerceIn(0.0, 100.0) ?: 0.0
    PlumPanel {
        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
            Row(
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
                modifier = Modifier.fillMaxWidth(),
            ) {
                Column(
                    verticalArrangement = Arrangement.spacedBy(3.dp),
                    modifier = Modifier.weight(1f),
                ) {
                    Text(item.title, style = PlumTheme.typography.titleSmall, color = palette.text)
                    Text(item.statusText, style = PlumTheme.typography.bodySmall, color = palette.muted)
                }
                Text(
                    "${progress.toInt()}%",
                    style = PlumTheme.typography.labelLarge,
                    color = palette.textSecondary,
                )
            }
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .height(5.dp)
                    .clip(RoundedCornerShape(999.dp))
                    .background(palette.surface),
            ) {
                Box(
                    modifier = Modifier
                        .fillMaxWidth(fraction = (progress / 100.0).toFloat())
                        .height(5.dp)
                        .clip(RoundedCornerShape(999.dp))
                        .background(palette.accent),
                )
            }
            PlumMetadataChips(
                values = listOf(
                    item.mediaType.uppercase(),
                    item.source.uppercase(),
                    item.sizeLeftBytes?.let { formatBytes(it) } ?: "\u2014",
                    item.etaSeconds?.let { formatEta(it) } ?: "\u2014",
                ),
            )
            item.errorMessage?.takeIf { it.isNotBlank() }?.let {
                Text(it, style = PlumTheme.typography.bodySmall, color = palette.error)
            }
        }
    }
}

// ── Helpers ──────────────────────────────────────────────────────────────────

private fun discoverItemSubtitle(item: DiscoverItemJson): String? {
    val year = item.releaseDate?.take(4) ?: item.firstAirDate?.take(4)
    val value = listOfNotNull(year, item.mediaType.uppercase()).joinToString(" \u2022 ")
    return value.takeIf { it.isNotBlank() }
}

private fun formatBytes(value: Long): String {
    if (value <= 0) return "\u2014"
    val units = listOf("B", "KB", "MB", "GB", "TB")
    var size = value.toDouble()
    var index = 0
    while (size >= 1024 && index < units.lastIndex) {
        size /= 1024
        index += 1
    }
    return if (size >= 100 || index == 0) "${size.toInt()} ${units[index]}" else String.format("%.1f %s", size, units[index])
}

private fun formatEta(seconds: Double): String {
    if (seconds <= 0) return "\u2014"
    val total = seconds.toInt()
    val hours = total / 3600
    val minutes = (total % 3600) / 60
    return when {
        hours > 0 -> "${hours}h ${minutes}m"
        minutes > 0 -> "${minutes}m"
        else -> "${total}s"
    }
}
