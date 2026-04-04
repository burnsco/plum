package plum.tv.core.ui

import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.wrapContentHeight
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Icon
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusProperties
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.tv.material3.ClickableSurfaceDefaults
import androidx.tv.material3.Glow
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import coil3.compose.AsyncImage
import coil3.imageLoader
import coil3.request.CachePolicy
import coil3.request.ImageRequest
import coil3.request.crossfade
import androidx.compose.ui.graphics.vector.ImageVector

enum class PlumButtonVariant {
    Primary,
    Secondary,
    Ghost,
}

data class PlumRailItem(
    val key: String,
    val label: String,
    val icon: ImageVector,
    val selected: Boolean,
    val onClick: () -> Unit,
    val dividerAfter: Boolean = false,
)

/** Resolves a relative server path against the configured base URL. */
fun resolveImageUrl(base: String, path: String): String {
    val trimmed = path.trim()
    if (trimmed.startsWith("http://") || trimmed.startsWith("https://")) return trimmed
    if (base.isBlank()) return trimmed
    return "${base.trimEnd('/')}/${trimmed.trimStart('/')}"
}

private const val TMDB_IMAGE_BASE = "https://image.tmdb.org/t/p"

/**
 * TMDb poster/backdrop paths are a single segment, e.g. `/abc.jpg`.
 * Prefer loading these from the public CDN before proxied `/api/media/.../artwork/` paths so movie posters
 * still show if the server artwork proxy fails; TV rows often used show artwork URLs instead.
 */
private fun isLikelyTmdbRelativePath(path: String): Boolean {
    if (!path.startsWith("/") || path.length < 2) return false
    if (path.startsWith("/api")) return false
    if (path.contains("//")) return false
    return path.indexOf('/', startIndex = 1) < 0
}

private val TMDB_SIZE_REGEX = Regex("w\\d+", RegexOption.IGNORE_CASE)

/**
 * Backend metadata often stores full `image.tmdb.org/t/p/w500/...` URLs. Callers still pass a target
 * [tmdbSize] (e.g. [PlumImageSizes.BACKDROP_HERO]); rewrite the path segment so we do not upscale a tiny CDN asset.
 */
private fun withTmdbImageSize(url: String, tmdbSize: String): String {
    val marker = "/t/p/"
    val idx = url.indexOf(marker, startIndex = 0, ignoreCase = true)
    if (idx < 0) return url
    val after = idx + marker.length
    if (after >= url.length) return url
    val rest = url.substring(after)
    val slash = rest.indexOf('/')
    if (slash <= 0) return url
    val sizeToken = rest.substring(0, slash)
    if (sizeToken.equals("original", ignoreCase = true)) return url
    if (!sizeToken.matches(TMDB_SIZE_REGEX)) return url
    return url.substring(0, after) + tmdbSize + rest.substring(slash)
}

/** Resolves artwork from the authenticated backend or a TMDb poster/backdrop path. */
fun resolveArtworkUrl(
    base: String,
    artworkUrl: String?,
    artworkPath: String?,
    tmdbSize: String,
): String? {
    val pathTrim = artworkPath?.trim()?.takeIf { it.isNotEmpty() }
    val urlTrim = artworkUrl?.trim()?.takeIf { it.isNotEmpty() }

    if (pathTrim != null) {
        when {
            pathTrim.startsWith("http://", ignoreCase = true) ||
                pathTrim.startsWith("https://", ignoreCase = true) ->
                return withTmdbImageSize(pathTrim, tmdbSize)
            isLikelyTmdbRelativePath(pathTrim) -> return "$TMDB_IMAGE_BASE/$tmdbSize$pathTrim"
        }
    }

    if (urlTrim != null) {
        return withTmdbImageSize(resolveImageUrl(base, urlTrim), tmdbSize)
    }

    val resolvedPath = pathTrim ?: return null
    if (resolvedPath.startsWith("http://") || resolvedPath.startsWith("https://")) {
        return withTmdbImageSize(resolvedPath, tmdbSize)
    }
    return "$TMDB_IMAGE_BASE/$tmdbSize$resolvedPath"
}

private fun buildArtworkRequest(
    context: android.content.Context,
    url: String,
    widthPx: Int,
    heightPx: Int,
): ImageRequest =
    ImageRequest.Builder(context)
        .data(url)
        .size(widthPx, heightPx)
        .memoryCachePolicy(CachePolicy.ENABLED)
        .diskCachePolicy(CachePolicy.ENABLED)
        .build()

@Composable
fun PlumScreenTitle(
    title: String,
    subtitle: String? = null,
    modifier: Modifier = Modifier,
) {
    Column(modifier = modifier, verticalArrangement = Arrangement.spacedBy(5.dp)) {
        Text(
            text = title,
            style = PlumTheme.typography.headlineLarge,
            color = PlumTheme.palette.text,
        )
        subtitle?.let {
            Text(
                text = it,
                style = PlumTheme.typography.bodyMedium,
                color = PlumTheme.palette.muted,
            )
        }
    }
}

@Composable
fun PlumSectionHeader(
    title: String,
    subtitle: String? = null,
    modifier: Modifier = Modifier,
) {
    Column(modifier = modifier, verticalArrangement = Arrangement.spacedBy(3.dp)) {
        Text(
            text = title,
            style = PlumTheme.typography.titleLarge,
            color = PlumTheme.palette.text,
        )
        subtitle?.let {
            Text(
                text = it,
                style = PlumTheme.typography.bodySmall,
                color = PlumTheme.palette.muted,
            )
        }
    }
}

@Composable
fun PlumPanel(
    modifier: Modifier = Modifier,
    contentPadding: PaddingValues = PaddingValues(20.dp),
    content: @Composable () -> Unit,
) {
    val palette = PlumTheme.palette
    val shape = RoundedCornerShape(PlumTheme.metrics.panelRadius)
    Box(
        modifier =
            modifier
                .clip(shape)
                .background(palette.panel)
                .padding(contentPadding),
    ) {
        content()
    }
}

@Composable
fun PlumStatePanel(
    title: String,
    message: String,
    modifier: Modifier = Modifier,
    actions: @Composable (() -> Unit)? = null,
) {
    PlumPanel(
        modifier = modifier,
        contentPadding = PaddingValues(18.dp),
    ) {
        Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Text(
                text = title,
                style = PlumTheme.typography.titleLarge,
                color = PlumTheme.palette.text,
            )
            Text(
                text = message,
                style = PlumTheme.typography.bodyMedium,
                color = PlumTheme.palette.muted,
            )
            actions?.invoke()
        }
    }
}

/**
 * Full-bleed backdrop + scrim background for cinematic detail screens.
 * Renders an optional backdrop image behind a solid scrim, then layers [content] on top.
 * The backdrop is fixed (non-scrolling); scroll the content inside [content] as needed.
 */
@Composable
fun PlumDetailBackground(
    backdropUrl: String?,
    scrim: Brush = PlumScrims.backdropVertical,
    modifier: Modifier = Modifier,
    content: @Composable BoxScope.() -> Unit,
) {
    val context = LocalContext.current
    Box(modifier = modifier.fillMaxSize()) {
        if (backdropUrl != null) {
            val request = remember(backdropUrl) {
                ImageRequest.Builder(context)
                    .data(backdropUrl)
                    .crossfade(400)
                    .build()
            }
            AsyncImage(
                model = request,
                contentDescription = null,
                modifier = Modifier.fillMaxSize(),
                contentScale = ContentScale.Crop,
            )
        }
        Box(
            modifier = Modifier
                .fillMaxSize()
                .background(scrim),
        )
        content()
    }
}

/**
 * Standard detail-page header row: optional poster image on the left, info column on the right.
 * Adopting this in all detail screens keeps poster sizing and spacing consistent.
 */
@Composable
fun PlumDetailHeroHeader(
    posterUrl: String?,
    modifier: Modifier = Modifier,
    content: @Composable ColumnScope.() -> Unit,
) {
    val metrics = PlumTheme.metrics
    Row(
        modifier = modifier,
        horizontalArrangement = Arrangement.spacedBy(24.dp),
        verticalAlignment = Alignment.Top,
    ) {
        if (posterUrl != null) {
            AsyncImage(
                model = posterUrl,
                contentDescription = null,
                modifier = Modifier
                    .width(metrics.heroPosterWidth)
                    .height(metrics.heroPosterHeight)
                    .clip(RoundedCornerShape(metrics.tileRadius)),
                contentScale = ContentScale.Fit,
            )
        }
        Column(
            modifier = Modifier.weight(1f),
            verticalArrangement = Arrangement.spacedBy(10.dp),
            content = content,
        )
    }
}

@Composable
fun PlumActionButton(
    label: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    variant: PlumButtonVariant = PlumButtonVariant.Primary,
    leadingIcon: ImageVector? = null,
    leadingBadge: String? = null,
    enabled: Boolean = true,
) {
    val palette = PlumTheme.palette
    val shape = RoundedCornerShape(PlumTheme.metrics.buttonRadius)
    val containerColor =
        when (variant) {
            PlumButtonVariant.Primary -> palette.accentSoft
            PlumButtonVariant.Secondary -> palette.surface
            PlumButtonVariant.Ghost -> Color.Transparent
        }
    val focusedContainerColor =
        when (variant) {
            PlumButtonVariant.Primary -> palette.accent
            PlumButtonVariant.Secondary -> palette.surfaceHover
            PlumButtonVariant.Ghost -> palette.accentSoft
        }
    val contentColor =
        when (variant) {
            PlumButtonVariant.Primary -> palette.text
            PlumButtonVariant.Secondary -> palette.text
            PlumButtonVariant.Ghost -> palette.textSecondary
        }
    val focusedContentColor =
        when (variant) {
            PlumButtonVariant.Primary -> Color(0xFF1A1030)
            PlumButtonVariant.Secondary -> palette.text
            PlumButtonVariant.Ghost -> palette.text
        }
    Surface(
        onClick = onClick,
        enabled = enabled,
        modifier = modifier.wrapContentHeight(),
        shape = ClickableSurfaceDefaults.shape(shape = shape),
        colors =
            ClickableSurfaceDefaults.colors(
                containerColor = containerColor,
                contentColor = contentColor,
                focusedContainerColor = focusedContainerColor,
                focusedContentColor = focusedContentColor,
                pressedContainerColor = focusedContainerColor,
                pressedContentColor = focusedContentColor,
                disabledContainerColor = palette.surface,
                disabledContentColor = palette.muted,
        ),
        scale = ClickableSurfaceDefaults.scale(focusedScale = 1f),
        border =
            ClickableSurfaceDefaults.border(
                border = plumBorder(if (variant == PlumButtonVariant.Ghost) Color.Transparent else palette.border, if (variant == PlumButtonVariant.Ghost) 0.dp else 1.dp, shape),
                focusedBorder = plumBorder(palette.borderStrong, 2.dp, shape),
                pressedBorder = plumBorder(palette.borderStrong, 2.dp, shape),
            ),
        glow = ClickableSurfaceDefaults.glow(focusedGlow = Glow(Color.Transparent, 0.dp)),
    ) {
        Row(
            modifier = Modifier.padding(horizontal = 18.dp, vertical = 11.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(9.dp),
        ) {
            when {
                leadingIcon != null -> {
                    Icon(
                        imageVector = leadingIcon,
                        contentDescription = null,
                        tint = palette.textSecondary,
                        modifier = Modifier.size(17.dp),
                    )
                }
                leadingBadge != null -> {
                    Box(
                        modifier =
                            Modifier
                                .size(24.dp)
                                .clip(RoundedCornerShape(8.dp))
                                .background(if (variant == PlumButtonVariant.Primary) palette.panelAlt else palette.accentSoft),
                        contentAlignment = Alignment.Center,
                    ) {
                        Text(
                            text = leadingBadge,
                            style = PlumTheme.typography.labelSmall,
                            color = palette.textSecondary,
                        )
                    }
                }
            }
            Text(
                text = label,
                style = PlumTheme.typography.labelLarge,
                fontWeight = FontWeight.SemiBold,
            )
        }
    }
}

/** Plex-style focus ring: bright neutral frame that reads well on dark backdrops. */
private val PosterFocusRing = Color(0xFFE8E8ED)

/**
 * Portrait poster tile: full-bleed artwork with title and subtitle **below** the image (Plex-style),
 * crisp 2:3 frame, subtle unfocused edge, and a clear focus treatment for D-pad navigation.
 */
@Composable
fun PlumPosterCard(
    title: String,
    subtitle: String? = null,
    imageUrl: String? = null,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    compact: Boolean = false,
    progressPercent: Double? = null,
    /** Fully watched — corner badge (library grids, etc.). */
    watched: Boolean = false,
    /**
     * TV focus zoom paints outside layout bounds; lazy grids with horizontal content padding clip
     * that overflow, so the first column beside the side rail looks cropped. Use `1f` there.
     */
    focusedScale: Float? = null,
    /** Library thumbnails can be wider than 2:3; crop keeps the frame edge-aligned with the artwork. */
    imageContentScale: ContentScale = ContentScale.Fit,
) {
    val palette = PlumTheme.palette
    val metrics = PlumTheme.metrics
    val serverBase = LocalServerBaseUrl.current
    val width = if (compact) metrics.posterCompactWidth else metrics.posterWidth
    val height = if (compact) metrics.posterCompactHeight else metrics.posterHeight
    val shape = RoundedCornerShape(metrics.posterCornerRadius)
    val cardShape = RoundedCornerShape(
        topStart = metrics.posterCornerRadius,
        topEnd = metrics.posterCornerRadius,
        bottomStart = 0.dp,
        bottomEnd = 0.dp,
    )
    val titleGap = if (compact) 6.dp else 8.dp
    val frameWidth = if (compact) 1.dp else 1.5.dp
    val focusFrameWidth = if (compact) 2.5.dp else 3.dp

    val resolvedUrl = imageUrl?.takeIf { it.isNotBlank() }?.let { resolveImageUrl(serverBase, it) }
    val context = LocalContext.current
    val density = LocalDensity.current
    val widthPx = remember(width, density) { with(density) { width.roundToPx().coerceAtLeast(1) } }
    val heightPx = remember(height, density) { with(density) { height.roundToPx().coerceAtLeast(1) } }
    val posterRequest =
        remember(resolvedUrl, widthPx, heightPx) {
            if (resolvedUrl == null) {
                null
            } else {
                buildArtworkRequest(context, resolvedUrl, widthPx, heightPx)
            }
        }

    var isFocused by remember { mutableStateOf(false) }
    val animatedProgress by animateFloatAsState(
        targetValue = ((progressPercent ?: 0.0) / 100.0).coerceIn(0.0, 1.0).toFloat(),
        label = "progress",
    )

    val titleStyle = PlumTheme.typography.titleSmall
    val subtitleStyle = PlumTheme.typography.labelMedium
    val titleWeight = if (compact) FontWeight.Medium else FontWeight.SemiBold
    val resolvedFocusedScale =
        focusedScale ?: if (compact) {
            1.055f
        } else {
            1.065f
        }

    Surface(
        onClick = onClick,
        modifier =
            modifier
                .width(width)
                .wrapContentHeight()
                .onFocusChanged { fs ->
                    isFocused = fs.isFocused
                    if (fs.isFocused && resolvedUrl != null) {
                        context.imageLoader.enqueue(
                            buildArtworkRequest(context, resolvedUrl, widthPx, heightPx),
                        )
                    }
                },
        shape = ClickableSurfaceDefaults.shape(shape = cardShape),
        colors =
            ClickableSurfaceDefaults.colors(
                containerColor = Color.Transparent,
                contentColor = palette.text,
                focusedContainerColor = Color.Transparent,
                focusedContentColor = palette.text,
                pressedContainerColor = Color.Transparent,
                pressedContentColor = palette.text,
                disabledContainerColor = Color.Transparent,
                disabledContentColor = palette.muted,
            ),
        scale = ClickableSurfaceDefaults.scale(focusedScale = resolvedFocusedScale),
        border =
            ClickableSurfaceDefaults.border(
                border = plumBorder(Color.Transparent, 0.dp, cardShape),
                focusedBorder = plumBorder(Color.Transparent, 0.dp, cardShape),
                pressedBorder = plumBorder(Color.Transparent, 0.dp, cardShape),
            ),
        glow = ClickableSurfaceDefaults.glow(focusedGlow = Glow(Color.Transparent, 0.dp)),
    ) {
        Column(modifier = Modifier.fillMaxWidth()) {
            Box(
                modifier =
                    Modifier
                        .fillMaxWidth()
                        .height(height)
                        .clip(shape)
                        .border(
                            width = if (isFocused) focusFrameWidth else frameWidth,
                            color =
                                if (isFocused) {
                                    PosterFocusRing
                                } else {
                                    Color.White.copy(alpha = 0.11f)
                                },
                            shape = shape,
                        ),
            ) {
                if (posterRequest != null) {
                    AsyncImage(
                        model = posterRequest,
                        contentDescription = title,
                        modifier = Modifier.fillMaxSize(),
                        contentScale = imageContentScale,
                    )
                } else {
                    Box(
                        modifier =
                            Modifier
                                .fillMaxSize()
                                .background(palette.panelAlt),
                        contentAlignment = Alignment.Center,
                    ) {
                        Text(
                            text = title,
                            style = if (compact) PlumTheme.typography.labelLarge else PlumTheme.typography.titleMedium,
                            color = palette.muted,
                            maxLines = 4,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier.padding(12.dp),
                        )
                    }
                }

                if (animatedProgress > 0f) {
                    Box(
                        modifier =
                            Modifier
                                .fillMaxWidth()
                                .height(5.dp)
                                .align(Alignment.BottomCenter)
                                .background(Color(0xB3000000)),
                    ) {
                        Box(
                            modifier =
                                Modifier
                                    .fillMaxSize()
                                    .background(Color.White.copy(alpha = 0.12f)),
                        )
                        Box(
                            modifier =
                                Modifier
                                    .fillMaxHeight()
                                    .fillMaxWidth(animatedProgress)
                                    .background(palette.accent),
                        )
                    }
                }

                if (watched) {
                    Box(
                        modifier =
                            Modifier
                                .align(Alignment.TopEnd)
                                .padding(5.dp)
                                .clip(RoundedCornerShape(999.dp))
                                .background(Color(0xE6000000))
                                .padding(horizontal = 6.dp, vertical = 2.dp),
                    ) {
                        Text(
                            text = "✓",
                            style = PlumTheme.typography.labelSmall,
                            color = PosterFocusRing,
                            fontWeight = FontWeight.Bold,
                        )
                    }
                }
            }

            Spacer(modifier = Modifier.height(titleGap))

            Column(
                modifier = Modifier.fillMaxWidth(),
                verticalArrangement = Arrangement.spacedBy(2.dp),
            ) {
                Text(
                    text = title,
                    style = titleStyle,
                    color = if (isFocused) palette.text else palette.textSecondary,
                    fontWeight = titleWeight,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                )
                if (!subtitle.isNullOrBlank()) {
                    Text(
                        text = subtitle,
                        style = subtitleStyle,
                        color = if (isFocused) palette.muted else palette.muted.copy(alpha = 0.85f),
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
            }
        }
    }
}

@Composable
fun PlumMetadataChips(
    values: List<String>,
    modifier: Modifier = Modifier,
) {
    val filtered = values.filter { it.isNotBlank() }
    if (filtered.isEmpty()) return
    Text(
        text = filtered.joinToString("  \u00B7  "),
        style = PlumTheme.typography.labelMedium,
        color = PlumTheme.palette.textSecondary,
        modifier = modifier,
    )
}

/**
 * Wide navigation rail with explicit labels, matching the web app's sidebar order.
 */
@Composable
fun PlumSideRail(
    items: List<PlumRailItem>,
    modifier: Modifier = Modifier,
    /** D-pad Right from a rail item jumps here (main NavHost content). */
    contentFocusRequester: FocusRequester? = null,
    /** When set, attached to the first rail item (e.g. Home) for initial / programmatic focus. */
    firstItemFocusRequester: FocusRequester? = null,
    footer: @Composable (() -> Unit)? = null,
) {
    val palette = PlumTheme.palette
    val metrics = PlumTheme.metrics
    var railHasFocus by remember { mutableStateOf(true) }
    val expanded = railHasFocus
    // No animate*AsState: width/labels/footer switch in one frame (avoids low-FPS stepped animation on TV).
    val railWidth = if (expanded) metrics.railWidth else metrics.railCollapsedWidth
    val railHorizontalPadding = if (expanded) 14.dp else 6.dp
    Column(
        modifier = modifier
            .width(railWidth)
            .fillMaxHeight()
            .background(palette.panel)
            .clip(RoundedCornerShape(0.dp))
            .onFocusChanged { railHasFocus = it.hasFocus }
            .padding(horizontal = railHorizontalPadding, vertical = 20.dp),
        horizontalAlignment = Alignment.Start,
        verticalArrangement = Arrangement.spacedBy(4.dp),
    ) {
        if (expanded) {
            Text(
                text = "Plum",
                style = PlumTheme.typography.titleLarge,
                color = palette.text,
                fontWeight = FontWeight.SemiBold,
                modifier = Modifier.padding(start = 4.dp),
                maxLines = 1,
            )
            Spacer(modifier = Modifier.height(16.dp))
        } else {
            Spacer(modifier = Modifier.height(8.dp))
        }

        items.forEachIndexed { index, item ->
            val firstMod =
                if (index == 0 && firstItemFocusRequester != null) {
                    Modifier.focusRequester(firstItemFocusRequester)
                } else {
                    Modifier
                }
            PlumRailButton(
                item = item,
                contentFocusRequester = contentFocusRequester,
                modifier = firstMod,
                railExpanded = expanded,
            )
            if (item.dividerAfter) {
                PlumRailDivider()
            }
        }

        Spacer(modifier = Modifier.weight(1f))

        if (footer != null && expanded) {
            footer()
        }
    }
}

@Composable
private fun PlumRailButton(
    item: PlumRailItem,
    contentFocusRequester: FocusRequester?,
    modifier: Modifier = Modifier,
    railExpanded: Boolean = true,
) {
    val palette = PlumTheme.palette
    val metrics = PlumTheme.metrics
    val shape = RoundedCornerShape(metrics.tileRadius)
    var isFocused by remember { mutableStateOf(false) }
    val iconSize = if (railExpanded) 20.dp else 26.dp
    Surface(
        onClick = item.onClick,
        modifier =
            modifier
                .fillMaxWidth()
                .onFocusChanged { isFocused = it.isFocused }
                .then(
                    if (contentFocusRequester != null) {
                        Modifier.focusProperties { right = contentFocusRequester }
                    } else {
                        Modifier
                    },
                ),
        shape = ClickableSurfaceDefaults.shape(shape = shape),
        colors =
            ClickableSurfaceDefaults.colors(
                containerColor = Color.Transparent,
                contentColor = if (item.selected) palette.accent else palette.muted,
                focusedContainerColor = palette.surface,
                focusedContentColor = if (item.selected) palette.accent else palette.text,
                pressedContainerColor = palette.surface,
                pressedContentColor = if (item.selected) palette.accent else palette.text,
            ),
        scale = ClickableSurfaceDefaults.scale(focusedScale = 1f),
        border =
            ClickableSurfaceDefaults.border(
                border = plumBorder(Color.Transparent, 0.dp, shape),
                focusedBorder = plumBorder(palette.accent.copy(alpha = 0.6f), 1.5.dp, shape),
                pressedBorder = plumBorder(palette.accent.copy(alpha = 0.6f), 1.5.dp, shape),
            ),
        glow = ClickableSurfaceDefaults.glow(focusedGlow = Glow(Color.Transparent, 0.dp)),
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .height(42.dp)
                .padding(
                    start = if (railExpanded) 4.dp else 0.dp,
                    end = if (railExpanded) 8.dp else 0.dp,
                ),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = if (railExpanded) Arrangement.Start else Arrangement.Center,
        ) {
            if (railExpanded) {
                Box(
                    modifier = Modifier
                        .width(3.dp)
                        .height(20.dp)
                        .clip(RoundedCornerShape(999.dp))
                        .background(if (item.selected) palette.accent else Color.Transparent),
                )
                Spacer(modifier = Modifier.width(10.dp))
            }
            Icon(
                imageVector = item.icon,
                contentDescription = item.label,
                tint = when {
                    item.selected -> palette.accent
                    isFocused -> palette.text
                    else -> palette.muted
                },
                modifier = Modifier.size(iconSize),
            )
            if (railExpanded) {
                Spacer(modifier = Modifier.width(12.dp))
                Text(
                    text = item.label,
                    style = PlumTheme.typography.labelLarge,
                    fontWeight = if (item.selected) FontWeight.SemiBold else FontWeight.Medium,
                    color = when {
                        item.selected -> palette.text
                        isFocused -> palette.text
                        else -> palette.textSecondary
                    },
                    maxLines = 1,
                )
            }
        }
    }
}

@Composable
private fun PlumRailDivider() {
    Box(
        modifier =
            Modifier
                .fillMaxWidth()
                .padding(vertical = 4.dp)
                .height(1.dp)
                .background(PlumTheme.palette.border),
    )
}

@Composable
fun plumOutlinedFieldColors() =
    OutlinedTextFieldDefaults.colors(
        focusedBorderColor = PlumTheme.palette.borderStrong,
        unfocusedBorderColor = PlumTheme.palette.border,
        focusedLabelColor = PlumTheme.palette.textSecondary,
        unfocusedLabelColor = PlumTheme.palette.muted,
        focusedTextColor = PlumTheme.palette.text,
        unfocusedTextColor = PlumTheme.palette.text,
        cursorColor = PlumTheme.palette.accent,
        focusedContainerColor = PlumTheme.palette.panel,
        unfocusedContainerColor = PlumTheme.palette.panel,
    )
