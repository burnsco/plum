package plum.tv.core.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
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
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
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
import coil.compose.AsyncImage

enum class PlumButtonVariant {
    Primary,
    Secondary,
    Ghost,
}

data class PlumRailItem(
    val key: String,
    val label: String,
    val badge: String,
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

/** Resolves artwork from the authenticated backend or a TMDb poster/backdrop path. */
fun resolveArtworkUrl(
    base: String,
    artworkUrl: String?,
    artworkPath: String?,
    tmdbSize: String,
): String? {
    val resolvedUrl = artworkUrl?.trim()?.takeIf { it.isNotEmpty() }
    if (resolvedUrl != null) {
        return resolveImageUrl(base, resolvedUrl)
    }

    val resolvedPath = artworkPath?.trim()?.takeIf { it.isNotEmpty() } ?: return null
    if (resolvedPath.startsWith("http://") || resolvedPath.startsWith("https://")) return resolvedPath
    return "$TMDB_IMAGE_BASE/$tmdbSize$resolvedPath"
}

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
fun PlumActionButton(
    label: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    variant: PlumButtonVariant = PlumButtonVariant.Primary,
    leadingBadge: String? = null,
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
        scale = ClickableSurfaceDefaults.scale(focusedScale = 1.05f),
        border =
            ClickableSurfaceDefaults.border(
                border = plumBorder(if (variant == PlumButtonVariant.Ghost) Color.Transparent else palette.border, if (variant == PlumButtonVariant.Ghost) 0.dp else 1.dp, shape),
                focusedBorder = plumBorder(palette.borderStrong, 2.dp, shape),
                pressedBorder = plumBorder(palette.borderStrong, 2.dp, shape),
            ),
        glow = ClickableSurfaceDefaults.glow(focusedGlow = Glow(palette.accentGlow, 14.dp)),
    ) {
        Row(
            modifier = Modifier.padding(horizontal = 20.dp, vertical = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            if (leadingBadge != null) {
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
            Text(
                text = label,
                style = PlumTheme.typography.labelLarge,
                fontWeight = FontWeight.SemiBold,
            )
        }
    }
}

/**
 * Poster card with the title overlaid on the image — no chunky text panel below.
 * When there's no image, the card shows the title centered on a surface-colored background.
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
) {
    val palette = PlumTheme.palette
    val metrics = PlumTheme.metrics
    val serverBase = LocalServerBaseUrl.current
    val width = if (compact) metrics.posterCompactWidth else metrics.posterWidth
    val height = if (compact) metrics.posterCompactHeight else metrics.posterHeight
    val shape = RoundedCornerShape(metrics.tileRadius)

    val resolvedUrl = imageUrl?.takeIf { it.isNotBlank() }?.let { resolveImageUrl(serverBase, it) }

    Surface(
        onClick = onClick,
        modifier = modifier.width(width),
        shape = ClickableSurfaceDefaults.shape(shape = shape),
        colors =
            ClickableSurfaceDefaults.colors(
                containerColor = palette.panel,
                contentColor = palette.text,
                focusedContainerColor = palette.panel,
                focusedContentColor = palette.text,
                pressedContainerColor = palette.surfaceHover,
                pressedContentColor = palette.text,
                disabledContainerColor = palette.panel,
                disabledContentColor = palette.muted,
            ),
        scale = ClickableSurfaceDefaults.scale(focusedScale = 1.08f),
        border =
            ClickableSurfaceDefaults.border(
                border = plumBorder(Color.Transparent, 0.dp, shape),
                focusedBorder = plumBorder(palette.accent.copy(alpha = 0.6f), 2.dp, shape),
                pressedBorder = plumBorder(palette.accent.copy(alpha = 0.6f), 2.dp, shape),
            ),
        glow = ClickableSurfaceDefaults.glow(focusedGlow = Glow(palette.accentGlow, 22.dp)),
    ) {
        Box(
            modifier = Modifier
                .width(width)
                .height(height),
        ) {
            if (resolvedUrl != null) {
                AsyncImage(
                    model = resolvedUrl,
                    contentDescription = title,
                    modifier = Modifier.fillMaxSize(),
                    contentScale = ContentScale.Crop,
                )
            } else {
                // No-image fallback: title centered on a dark surface
                Box(
                    modifier = Modifier.fillMaxSize().background(palette.surface),
                    contentAlignment = Alignment.Center,
                ) {
                    Text(
                        text = title,
                        style = PlumTheme.typography.titleSmall,
                        color = palette.textSecondary,
                        maxLines = 4,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.padding(14.dp),
                    )
                }
            }

            // Bottom scrim — always rendered so title is readable over any image
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .fillMaxHeight(0.55f)
                    .align(Alignment.BottomCenter)
                    .background(
                        brush = Brush.verticalGradient(
                            colors = listOf(
                                Color.Transparent,
                                Color(0xBB000000),
                                Color(0xEE000000),
                            ),
                        ),
                    ),
            )

            // Progress bar for continue-watching items
            val pct = progressPercent
            if (pct != null && pct > 0.0) {
                Box(
                    modifier = Modifier
                        .fillMaxWidth()
                        .height(3.dp)
                        .align(Alignment.BottomCenter),
                ) {
                    Box(
                        modifier = Modifier
                            .fillMaxWidth()
                            .fillMaxHeight()
                            .background(Color.White.copy(alpha = 0.2f)),
                    )
                    Box(
                        modifier = Modifier
                            .fillMaxWidth(fraction = (pct / 100.0).coerceIn(0.0, 1.0).toFloat())
                            .fillMaxHeight()
                            .background(palette.accent),
                    )
                }
            }

            // Title + subtitle overlaid on the scrim
            Column(
                modifier = Modifier
                    .align(Alignment.BottomStart)
                    .padding(horizontal = 10.dp, vertical = if (progressPercent != null && progressPercent > 0.0) 6.dp else 10.dp),
                verticalArrangement = Arrangement.spacedBy(2.dp),
            ) {
                Text(
                    text = title,
                    style = PlumTheme.typography.labelMedium,
                    color = Color.White,
                    fontWeight = FontWeight.SemiBold,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                )
                if (!subtitle.isNullOrBlank()) {
                    Text(
                        text = subtitle,
                        style = PlumTheme.typography.labelSmall,
                        color = Color.White.copy(alpha = 0.65f),
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
    Row(
        modifier = modifier,
        horizontalArrangement = Arrangement.spacedBy(PlumTheme.metrics.chipGap),
    ) {
        filtered.forEach { value ->
            Box(
                modifier =
                    Modifier
                        .clip(RoundedCornerShape(999.dp))
                        .background(PlumTheme.palette.surface)
                        .padding(horizontal = 10.dp, vertical = 5.dp),
            ) {
                Text(
                    text = value,
                    style = PlumTheme.typography.labelSmall,
                    color = PlumTheme.palette.textSecondary,
                )
            }
        }
    }
}

/**
 * Wide navigation rail with explicit labels, matching the web app's sidebar order.
 */
@Composable
fun PlumSideRail(
    items: List<PlumRailItem>,
    modifier: Modifier = Modifier,
    footer: @Composable (() -> Unit)? = null,
) {
    val palette = PlumTheme.palette
    val metrics = PlumTheme.metrics
    Column(
        modifier = modifier
            .width(metrics.railWidth)
            .fillMaxHeight()
            .background(palette.panel)
            .padding(horizontal = 14.dp, vertical = 20.dp),
        horizontalAlignment = Alignment.Start,
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        // Logo mark
        Box(
            modifier = Modifier
                .size(36.dp)
                .clip(RoundedCornerShape(10.dp))
                .background(palette.accentSoft),
            contentAlignment = Alignment.Center,
        ) {
            Text(
                text = "P",
                style = PlumTheme.typography.titleLarge,
                color = palette.accent,
                fontWeight = FontWeight.Bold,
            )
        }

        Spacer(modifier = Modifier.height(18.dp))

        items.forEach { item ->
            PlumRailButton(item)
            if (item.dividerAfter) {
                PlumRailDivider()
            }
        }

        Spacer(modifier = Modifier.weight(1f))

        footer?.invoke()
    }
}

@Composable
private fun PlumRailButton(item: PlumRailItem) {
    val palette = PlumTheme.palette
    val shape = RoundedCornerShape(12.dp)
    Surface(
        onClick = item.onClick,
        shape = ClickableSurfaceDefaults.shape(shape = shape),
        colors =
            ClickableSurfaceDefaults.colors(
                containerColor = if (item.selected) palette.accentSoft else Color.Transparent,
                contentColor = if (item.selected) palette.accent else palette.muted,
                focusedContainerColor = palette.surfaceHover,
                focusedContentColor = palette.text,
                pressedContainerColor = palette.surfaceHover,
                pressedContentColor = palette.text,
            ),
        scale = ClickableSurfaceDefaults.scale(focusedScale = 1.06f),
        border =
            ClickableSurfaceDefaults.border(
                border = plumBorder(Color.Transparent, 0.dp, shape),
                focusedBorder = plumBorder(palette.borderStrong, 1.5.dp, shape),
            ),
        glow = ClickableSurfaceDefaults.glow(focusedGlow = Glow(palette.accentGlow, 12.dp)),
    ) {
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .height(44.dp)
                .padding(horizontal = 8.dp),
            contentAlignment = Alignment.CenterStart,
        ) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(10.dp),
            ) {
                Box(
                    modifier =
                        Modifier
                            .size(24.dp)
                            .clip(RoundedCornerShape(8.dp))
                            .background(if (item.selected) palette.accentSoft else palette.surface),
                    contentAlignment = Alignment.Center,
                ) {
                    Text(
                        text = item.badge,
                        style = PlumTheme.typography.labelSmall,
                        fontWeight = if (item.selected) FontWeight.Bold else FontWeight.Normal,
                        color = if (item.selected) palette.accent else palette.muted,
                    )
                }
                Text(
                    text = item.label,
                    style = PlumTheme.typography.labelLarge,
                    fontWeight = if (item.selected) FontWeight.SemiBold else FontWeight.Normal,
                    color = if (item.selected) palette.text else palette.textSecondary,
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
