package plum.tv.feature.discover

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.lazy.grid.itemsIndexed
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
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
import plum.tv.core.ui.PlumMetadataChips
import plum.tv.core.ui.PlumImageSizes
import plum.tv.core.ui.PlumPanel
import plum.tv.core.ui.PlumPosterCard
import plum.tv.core.ui.PlumScreenPadding
import plum.tv.core.ui.PlumScreenTitle
import plum.tv.core.ui.PlumScrims
import plum.tv.core.ui.PlumSectionHeader
import plum.tv.core.ui.PlumTheme
import plum.tv.core.ui.PlumStatePanel
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
                message = "Pulling in shelves, genres, and featured titles.",
            )
        }
        is DiscoverUiState.Error -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            PlumStatePanel(
                title = "Could not load discover",
                message = s.message,
                actions = {
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        PlumActionButton("Retry", onClick = { viewModel.refresh() }, leadingBadge = "R")
                    }
                },
            )
        }
        is DiscoverUiState.Ready -> {
            val discoverLeadFocus = remember { FocusRequester() }
            LaunchedTvFocusTo(
                s.discover.shelves.joinToString(separator = ",") { it.id },
                s.genres.movieGenres.size,
                s.genres.tvGenres.size,
                focusRequester = discoverLeadFocus,
            )
            LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = PlumScreenPadding(),
                verticalArrangement = Arrangement.spacedBy(16.dp),
            ) {
                item {
                    DiscoverHeader(
                        onOpenBrowse = onOpenBrowse,
                        browseLeadFocus = discoverLeadFocus,
                    )
                }
                item {
                    DiscoverGenres(
                        movieGenres = s.genres.movieGenres,
                        tvGenres = s.genres.tvGenres,
                        onOpenBrowse = onOpenBrowse,
                    )
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
    browseLeadFocus: FocusRequester? = null,
) {
    PlumPanel(contentPadding = PaddingValues(16.dp)) {
        Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
            PlumScreenTitle(
                title = "Discover",
                subtitle = "Browse shelves, filter genres, and jump into title detail.",
            )
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                PlumActionButton(
                    modifier = browseLeadFocus?.let { Modifier.focusRequester(it) } ?: Modifier,
                    label = "Browse All",
                    onClick = { onOpenBrowse(null, null, null) },
                    leadingBadge = "B",
                )
                discoverCategoryOptions.take(3).forEach { option ->
                    PlumActionButton(
                        label = option.label,
                        onClick = { onOpenBrowse(option.id, null, null) },
                        variant = PlumButtonVariant.Secondary,
                    )
                }
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
    val hasMovie = movieGenres.isNotEmpty()
    val hasTv = tvGenres.isNotEmpty()
    if (!hasMovie && !hasTv) return

    PlumPanel(contentPadding = PaddingValues(16.dp)) {
        Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
            PlumSectionHeader(title = "Browse by Genre", subtitle = "Jump straight into a catalog slice.")
            if (hasMovie) {
                DiscoverGenreRow("Movies", movieGenres, "movie", onOpenBrowse)
            }
            if (hasTv) {
                DiscoverGenreRow("TV", tvGenres, "tv", onOpenBrowse)
            }
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
        LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
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
) {
    val posterUrl = resolveArtworkUrl(serverBase, null, item.posterPath, PlumImageSizes.POSTER_GRID)
    PlumPosterCard(
        title = item.title,
        subtitle = discoverItemSubtitle(item),
        imageUrl = posterUrl,
        onClick = onClick,
        modifier = modifier,
        compact = true,
    )
}

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
                message = "Sharpening the filter set and loading titles.",
            )
        }
        is DiscoverBrowseUiState.Error -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            PlumStatePanel(
                title = "Could not load browse",
                message = s.message,
                actions = {
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        PlumActionButton("Retry", onClick = { viewModel.refresh(category, mediaType, genreId) }, leadingBadge = "R")
                    }
                },
            )
        }
        is DiscoverBrowseUiState.Ready -> {
            val metrics = PlumTheme.metrics
            val gridFirstFocus = remember { FocusRequester() }
            val editFiltersFocus = remember { FocusRequester() }
            val filterFormLeadFocus = remember { FocusRequester() }
            val startFiltersOpen =
                category == null && mediaType == null && genreId == null
            var filtersExpanded by remember(category, mediaType, genreId) {
                mutableStateOf(startFiltersOpen)
            }
            var draftCategory by remember { mutableStateOf<String?>(null) }
            var draftMediaType by remember { mutableStateOf<String?>(null) }
            var draftGenreId by remember { mutableStateOf<Int?>(null) }

            LaunchedEffect(filtersExpanded) {
                if (filtersExpanded) {
                    draftCategory = s.category
                    draftMediaType = s.mediaType
                    draftGenreId = s.genre?.id
                }
            }

            LaunchedTvFocusTo(
                s.title,
                s.category,
                s.mediaType,
                s.genre?.id,
                s.items.firstOrNull()?.let { "${it.mediaType}-${it.tmdbId}" },
                filtersExpanded,
                focusRequester = if (filtersExpanded) filterFormLeadFocus else gridFirstFocus,
            )
            // Adaptive grid: same approach as LibraryBrowseScreen so column count responds to width.
            val minCell = remember(metrics) { metrics.posterCompactWidth + metrics.cardGap }
            val filterScroll = rememberScrollState()

            Row(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(PlumScreenPadding()),
                horizontalArrangement = Arrangement.spacedBy(16.dp),
            ) {
                if (filtersExpanded) {
                    Column(
                        modifier =
                            Modifier
                                .width(392.dp)
                                .fillMaxHeight()
                                .verticalScroll(filterScroll),
                        verticalArrangement = Arrangement.spacedBy(12.dp),
                    ) {
                        PlumActionButton("Back", onClick = onBack, variant = PlumButtonVariant.Secondary)
                        PlumScreenTitle(
                            title = "Filters",
                            subtitle = "Choose catalog, movies or TV, and a genre — then OK to browse.",
                        )
                        DiscoverBrowseFilters(
                            category = draftCategory,
                            mediaType = draftMediaType,
                            genreId = draftGenreId,
                            genres = s.genres,
                            catalogLeadFocusRequester = filterFormLeadFocus,
                            onDraftChange = { c, m, g ->
                                draftCategory = c
                                draftMediaType = m
                                draftGenreId = g
                            },
                            onMediaTypeSelected = { m ->
                                draftMediaType = m
                                draftGenreId = null
                            },
                        )
                        Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                            PlumActionButton(
                                label = "Cancel",
                                onClick = { filtersExpanded = false },
                                variant = PlumButtonVariant.Ghost,
                            )
                            PlumActionButton(
                                label = "OK · Browse results",
                                onClick = {
                                    viewModel.refresh(draftCategory, draftMediaType, draftGenreId)
                                    filtersExpanded = false
                                },
                            )
                        }
                    }
                } else {
                    DiscoverBrowseCollapsedSidebar(
                        title = s.title,
                        totalResults = s.totalResults,
                        onBack = onBack,
                        onEditFilters = { filtersExpanded = true },
                        editFiltersFocusRequester = editFiltersFocus,
                    )
                }

                Box(
                    modifier =
                        Modifier
                            .weight(1f)
                            .fillMaxHeight(),
                ) {
                    if (s.items.isEmpty()) {
                        Box(
                            modifier = Modifier.fillMaxSize(),
                            contentAlignment = Alignment.Center,
                        ) {
                            PlumStatePanel(
                                modifier = Modifier.fillMaxWidth(),
                                title = "No titles found",
                                message = "Try a different category, media type, or genre filter.",
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
                                    modifier =
                                        if (index == 0) {
                                            Modifier.focusRequester(gridFirstFocus)
                                        } else {
                                            Modifier
                                        },
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
private fun DiscoverBrowseCollapsedSidebar(
    title: String,
    totalResults: Int,
    onBack: () -> Unit,
    onEditFilters: () -> Unit,
    editFiltersFocusRequester: FocusRequester,
) {
    val palette = PlumTheme.palette
    Column(
        modifier =
            Modifier
                .width(236.dp)
                .fillMaxHeight(),
        verticalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        PlumActionButton("Back", onClick = onBack, variant = PlumButtonVariant.Secondary)
        PlumPanel(contentPadding = PaddingValues(14.dp)) {
            Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                Text(
                    text = title,
                    style = PlumTheme.typography.titleMedium,
                    color = palette.text,
                    fontWeight = FontWeight.SemiBold,
                    maxLines = 3,
                    overflow = TextOverflow.Ellipsis,
                )
                Text(
                    text = "$totalResults titles",
                    style = PlumTheme.typography.bodySmall,
                    color = palette.muted,
                )
                Text(
                    text = "Press Right to browse posters. Use Edit filters to change catalog, media type, or genre.",
                    style = PlumTheme.typography.labelMedium,
                    color = palette.textSecondary,
                    maxLines = 5,
                    overflow = TextOverflow.Ellipsis,
                )
            }
        }
        PlumActionButton(
            modifier = Modifier.focusRequester(editFiltersFocusRequester),
            label = "Edit filters",
            onClick = onEditFilters,
            variant = PlumButtonVariant.Primary,
        )
    }
}

@Composable
private fun DiscoverBrowseFilters(
    category: String?,
    mediaType: String?,
    genreId: Int?,
    genres: List<DiscoverGenreJson>,
    catalogLeadFocusRequester: FocusRequester? = null,
    onDraftChange: (String?, String?, Int?) -> Unit,
    onMediaTypeSelected: (String) -> Unit,
) {
    PlumPanel(contentPadding = PaddingValues(14.dp)) {
        Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
            DiscoverFilterGroup(title = "Catalog shelf") {
                LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    item {
                        PlumActionButton(
                            modifier =
                                catalogLeadFocusRequester?.let { Modifier.focusRequester(it) } ?: Modifier,
                            label = "All",
                            onClick = { onDraftChange(null, mediaType, null) },
                            variant =
                                if (category == null && genreId == null) {
                                    PlumButtonVariant.Primary
                                } else {
                                    PlumButtonVariant.Secondary
                                },
                        )
                    }
                    discoverCategoryOptions.forEach { option ->
                        item {
                            PlumActionButton(
                                label = option.label,
                                onClick = { onDraftChange(option.id, mediaType, null) },
                                variant =
                                    if (category == option.id) {
                                        PlumButtonVariant.Primary
                                    } else {
                                        PlumButtonVariant.Secondary
                                    },
                            )
                        }
                    }
                }
            }
            DiscoverFilterGroup(title = "Media type") {
                LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    item {
                        PlumActionButton(
                            label = "Movies",
                            onClick = { onMediaTypeSelected("movie") },
                            variant = if (mediaType == "movie") PlumButtonVariant.Primary else PlumButtonVariant.Secondary,
                        )
                    }
                    item {
                        PlumActionButton(
                            label = "TV",
                            onClick = { onMediaTypeSelected("tv") },
                            variant = if (mediaType == "tv") PlumButtonVariant.Primary else PlumButtonVariant.Secondary,
                        )
                    }
                    item {
                        PlumActionButton(
                            label = "Clear genre",
                            onClick = { onDraftChange(category, mediaType, null) },
                            variant = if (genreId == null) PlumButtonVariant.Primary else PlumButtonVariant.Secondary,
                        )
                    }
                }
            }
            if (genres.isNotEmpty()) {
                DiscoverFilterGroup(title = "Genre (scroll)") {
                    LazyVerticalGrid(
                        columns = GridCells.Fixed(2),
                        modifier =
                            Modifier
                                .fillMaxWidth()
                                .height(272.dp),
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                        verticalArrangement = Arrangement.spacedBy(8.dp),
                    ) {
                        items(genres.size, key = { genres[it].id }) { index ->
                            val g = genres[index]
                            PlumActionButton(
                                label = g.name,
                                onClick = { onDraftChange(category, mediaType, g.id) },
                                variant = if (genreId == g.id) PlumButtonVariant.Primary else PlumButtonVariant.Secondary,
                                modifier = Modifier.fillMaxWidth(),
                            )
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun DiscoverFilterGroup(
    title: String,
    content: @Composable () -> Unit,
) {
    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
        Text(
            text = title,
            style = PlumTheme.typography.labelLarge,
            color = PlumTheme.palette.textSecondary,
        )
        content()
    }
}

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
                message = "Fetching artwork, metadata, and library matches.",
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
            val addLabel =
                when (d.acquisition?.state) {
                    "available" -> "In Library"
                    "downloading" -> "Downloading"
                    "added" -> "Added"
                    else -> if (isConfigured) "Add" else "Unavailable"
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
                                addAll(d.genres.take(4))
                            },
                        )
                        if (d.overview.isNotBlank()) {
                            Text(
                                text = d.overview,
                                maxLines = 8,
                                overflow = TextOverflow.Ellipsis,
                                style = PlumTheme.typography.bodyLarge,
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
                                        if (!isConfigured) {
                                            onOpenSettings()
                                        } else if (canAdd) {
                                            viewModel.addTitle(mediaType, tmdbId)
                                        }
                                    },
                                    variant = PlumButtonVariant.Primary,
                                )
                            }
                            PlumActionButton("Back", onClick = onBack, variant = PlumButtonVariant.Secondary)
                        }
                    }
                    if (d.libraryMatches.isNotEmpty()) {
                        PlumPanel {
                            Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                                PlumSectionHeader("Available in Plum")
                                d.libraryMatches.forEach { match ->
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
                message = "Checking the Radarr and Sonarr queues.",
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
                PlumPanel {
                    Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                        PlumScreenTitle(
                            title = "Downloads",
                            subtitle = "Live queue from Radarr and Sonarr TV.",
                        )
                        PlumActionButton(
                            modifier = Modifier.focusRequester(refreshFocus),
                            label = "Refresh",
                            onClick = { viewModel.refresh() },
                            variant = PlumButtonVariant.Secondary,
                        )
                    }
                }

                when {
                    !s.configured ->
                        PlumStatePanel(
                            title = "Media stack not configured",
                            message = "Connect Radarr and Sonarr TV on the server to see download activity.",
                            actions = {
                                PlumActionButton("Open Settings", onClick = onOpenSettings)
                            },
                        )
                    s.items.isEmpty() ->
                        PlumStatePanel(
                            title = "No active downloads",
                            message = "New items you add from Discover will show up here while the stack is working on them.",
                        )
                    else ->
                        Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                            s.items.forEach { item ->
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
    val progress = item.progress?.coerceIn(0.0, 100.0) ?: 0.0
    PlumPanel {
        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
            Row(horizontalArrangement = Arrangement.SpaceBetween, modifier = Modifier.fillMaxWidth()) {
                Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    Text(item.title, style = PlumTheme.typography.titleSmall, color = PlumTheme.palette.text)
                    Text(item.statusText, style = PlumTheme.typography.bodySmall, color = PlumTheme.palette.muted)
                }
                Text("${progress.toInt()}%", style = PlumTheme.typography.labelLarge, color = PlumTheme.palette.textSecondary)
            }
            Box(
                modifier =
                    Modifier
                        .fillMaxWidth()
                        .height(6.dp)
                        .clip(RoundedCornerShape(999.dp))
                        .background(PlumTheme.palette.surface),
            ) {
                Box(
                    modifier =
                        Modifier
                            .fillMaxWidth(fraction = (progress / 100.0).toFloat())
                            .height(6.dp)
                            .background(PlumTheme.palette.accent),
                )
            }
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                PlumMetadataChips(
                    values = listOf(
                        item.mediaType.uppercase(),
                        item.source.uppercase(),
                        item.sizeLeftBytes?.let { formatBytes(it) } ?: "—",
                        item.etaSeconds?.let { formatEta(it) } ?: "—",
                    ),
                )
            }
            item.errorMessage?.takeIf { it.isNotBlank() }?.let {
                Text(it, style = PlumTheme.typography.bodySmall, color = PlumTheme.palette.error)
            }
        }
    }
}

private fun discoverItemSubtitle(item: DiscoverItemJson): String? {
    val year = item.releaseDate?.take(4) ?: item.firstAirDate?.take(4)
    val value = listOfNotNull(year, item.mediaType.uppercase()).joinToString(" • ")
    return value.takeIf { it.isNotBlank() }
}

private fun formatBytes(value: Long): String {
    if (value <= 0) return "—"
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
    if (seconds <= 0) return "—"
    val total = seconds.toInt()
    val hours = total / 3600
    val minutes = (total % 3600) / 60
    return when {
        hours > 0 -> "${hours}h ${minutes}m"
        minutes > 0 -> "${minutes}m"
        else -> "${total}s"
    }
}
