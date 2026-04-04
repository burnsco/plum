package plum.tv.core.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.ReadOnlyComposable
import androidx.compose.runtime.compositionLocalOf
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.tv.material3.ColorScheme
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Shapes
import androidx.tv.material3.Typography

/**
 * Centralized scrim gradient constants for cinematic detail and hero layouts.
 * Using shared constants prevents divergence between screens.
 */
object PlumScrims {
    /** Vertical dark-at-bottom scrim for show/discover detail backdrops. */
    val backdropVertical: Brush = Brush.verticalGradient(
        0.0f to Color(0xCC000000),
        0.35f to Color(0xDD000000),
        1.0f to Color(0xF5000000),
    )

    /** Left-heavy horizontal scrim for movie detail — keeps right side semi-visible. */
    val backdropHorizontal: Brush = Brush.horizontalGradient(
        0.0f to Color(0xF2000000),
        0.55f to Color(0xCC000000),
        1.0f to Color(0x44000000),
    )

    /** Bottom-heavy vertical scrim for the home hero section. */
    val heroBottom: Brush = Brush.verticalGradient(
        0.0f to Color(0x44000000),
        0.45f to Color(0x88000000),
        1.0f to Color(0xEE000000),
    )

    /** Player controls overlay — darker at top and bottom for text readability, lighter center. */
    val playerControls: Brush = Brush.verticalGradient(
        0.0f to Color(0xDD000000),
        0.18f to Color(0x55000000),
        0.45f to Color(0x18000000),
        0.65f to Color(0x18000000),
        0.82f to Color(0x66000000),
        1.0f to Color(0xEE000000),
    )
}

/** Provides the server base URL so composables can resolve relative image paths. */
val LocalServerBaseUrl = compositionLocalOf { "" }

@Immutable
data class PlumTvPalette(
    val sidebar: Color,
    val topbar: Color,
    val background: Color,
    val panel: Color,
    val panelAlt: Color,
    val surface: Color,
    val surfaceHover: Color,
    val accent: Color,
    val accentSoft: Color,
    val accentGlow: Color,
    val accentSecondary: Color,
    val text: Color,
    val textSecondary: Color,
    val muted: Color,
    val border: Color,
    val borderStrong: Color,
    val ring: Color,
    val success: Color,
    val warning: Color,
    val error: Color,
)

@Immutable
data class PlumTvMetrics(
    val screenPadding: PaddingValues,
    val railPadding: PaddingValues,
    val sectionGap: Dp,
    val cardGap: Dp,
    val chipGap: Dp,
    val posterWidth: Dp,
    val posterHeight: Dp,
    val posterCompactWidth: Dp,
    val posterCompactHeight: Dp,
    val heroPosterWidth: Dp,
    val heroPosterHeight: Dp,
    /** Home hero section height — tall enough to feel cinematic at couch distance. */
    val heroHeight: Dp,
    val thumbnailWidth: Dp,
    val thumbnailHeight: Dp,
    val panelRadius: Dp,
    val tileRadius: Dp,
    /** Rounded corners for poster tiles (rails / library / discover). */
    val posterCornerRadius: Dp,
    val buttonRadius: Dp,
    val railWidth: Dp,
    val railCollapsedWidth: Dp,
)

private val plumPalette =
    PlumTvPalette(
        sidebar = Color(0xFF0A0A0F),
        topbar = Color(0xFF0A0A0F),
        background = Color(0xFF07070B),
        panel = Color(0xF1121218),
        panelAlt = Color(0xF0171720),
        surface = Color(0xE61A1A22),
        surfaceHover = Color(0xEB202029),
        accent = Color(0xFFCFB1FF),
        accentSoft = Color(0x1ECFB1FF),
        accentGlow = Color.Transparent,
        accentSecondary = Color(0xFFE879C0),
        text = Color(0xFFF0EEFF),
        textSecondary = Color(0xFFD8D3F2),
        muted = Color(0xFF9990BE),
        border = Color(0x18B57BFF),
        borderStrong = Color(0x38B57BFF),
        ring = Color(0x70B57BFF),
        success = Color(0xFF4ADE80),
        warning = Color(0xFFFBBF24),
        error = Color(0xFFF87171),
    )

private val plumMetrics =
    PlumTvMetrics(
        screenPadding = PaddingValues(horizontal = 16.dp, vertical = 24.dp),
        railPadding = PaddingValues(horizontal = 0.dp, vertical = 20.dp),
        sectionGap = 30.dp,
        cardGap = 14.dp,
        chipGap = 8.dp,
        posterWidth = 170.dp,
        posterHeight = 255.dp,
        posterCompactWidth = 138.dp,
        posterCompactHeight = 207.dp,
        heroPosterWidth = 180.dp,
        heroPosterHeight = 270.dp,
        heroHeight = 420.dp,
        thumbnailWidth = 160.dp,
        thumbnailHeight = 90.dp,
        panelRadius = 14.dp,
        tileRadius = 12.dp,
        posterCornerRadius = 8.dp,
        buttonRadius = 999.dp,
        railWidth = 200.dp,
        /** Wide enough for a centered ~26dp icon after padding; narrower rails clip/squash icons. */
        railCollapsedWidth = 76.dp,
    )

private val LocalPlumPalette = staticCompositionLocalOf { plumPalette }
private val LocalPlumMetrics = staticCompositionLocalOf { plumMetrics }

private val interFamily = FontFamily(Font(R.font.inter_variable, weight = FontWeight.Normal))
private val outfitFamily = FontFamily(Font(R.font.outfit_variable, weight = FontWeight.SemiBold))

private val plumTypography =
    Typography(
        displayLarge = TextStyle(fontFamily = outfitFamily, fontWeight = FontWeight.Bold, fontSize = 56.sp, lineHeight = 62.sp),
        displayMedium = TextStyle(fontFamily = outfitFamily, fontWeight = FontWeight.Bold, fontSize = 46.sp, lineHeight = 52.sp),
        displaySmall = TextStyle(fontFamily = outfitFamily, fontWeight = FontWeight.SemiBold, fontSize = 36.sp, lineHeight = 42.sp),
        headlineLarge = TextStyle(fontFamily = outfitFamily, fontWeight = FontWeight.SemiBold, fontSize = 30.sp, lineHeight = 36.sp),
        headlineMedium = TextStyle(fontFamily = outfitFamily, fontWeight = FontWeight.SemiBold, fontSize = 26.sp, lineHeight = 32.sp),
        headlineSmall = TextStyle(fontFamily = outfitFamily, fontWeight = FontWeight.Medium, fontSize = 22.sp, lineHeight = 28.sp),
        titleLarge = TextStyle(fontFamily = interFamily, fontWeight = FontWeight.SemiBold, fontSize = 20.sp, lineHeight = 26.sp),
        titleMedium = TextStyle(fontFamily = interFamily, fontWeight = FontWeight.SemiBold, fontSize = 18.sp, lineHeight = 24.sp),
        titleSmall = TextStyle(fontFamily = interFamily, fontWeight = FontWeight.Medium, fontSize = 15.sp, lineHeight = 21.sp),
        bodyLarge = TextStyle(fontFamily = interFamily, fontWeight = FontWeight.Normal, fontSize = 17.sp, lineHeight = 24.sp),
        bodyMedium = TextStyle(fontFamily = interFamily, fontWeight = FontWeight.Normal, fontSize = 15.sp, lineHeight = 22.sp),
        bodySmall = TextStyle(fontFamily = interFamily, fontWeight = FontWeight.Normal, fontSize = 13.sp, lineHeight = 19.sp),
        labelLarge = TextStyle(fontFamily = interFamily, fontWeight = FontWeight.SemiBold, fontSize = 15.sp, lineHeight = 21.sp),
        labelMedium = TextStyle(fontFamily = interFamily, fontWeight = FontWeight.Medium, fontSize = 13.sp, lineHeight = 19.sp),
        labelSmall = TextStyle(fontFamily = interFamily, fontWeight = FontWeight.Medium, fontSize = 11.sp, lineHeight = 16.sp),
    )

private val plumShapes =
    Shapes(
        extraSmall = RoundedCornerShape(8.dp),
        small = RoundedCornerShape(10.dp),
        medium = RoundedCornerShape(12.dp),
        large = RoundedCornerShape(16.dp),
        extraLarge = RoundedCornerShape(24.dp),
    )

private val plumColorScheme =
    androidx.tv.material3.darkColorScheme(
        primary = plumPalette.accent,
        onPrimary = Color(0xFF160E28),
        primaryContainer = plumPalette.accentSoft,
        onPrimaryContainer = plumPalette.text,
        inversePrimary = plumPalette.accentSecondary,
        secondary = plumPalette.accentSecondary,
        onSecondary = Color(0xFF240F22),
        secondaryContainer = Color(0x40E879C0),
        onSecondaryContainer = plumPalette.text,
        tertiary = plumPalette.textSecondary,
        onTertiary = plumPalette.background,
        tertiaryContainer = plumPalette.panelAlt,
        onTertiaryContainer = plumPalette.text,
        background = plumPalette.background,
        onBackground = plumPalette.text,
        surface = plumPalette.surface,
        onSurface = plumPalette.text,
        surfaceVariant = plumPalette.panelAlt,
        onSurfaceVariant = plumPalette.textSecondary,
        surfaceTint = plumPalette.accent,
        inverseSurface = plumPalette.text,
        inverseOnSurface = plumPalette.background,
        error = plumPalette.error,
        onError = Color.White,
        errorContainer = Color(0x40F87171),
        onErrorContainer = Color.White,
        border = plumPalette.borderStrong,
        borderVariant = plumPalette.border,
        scrim = Color(0xCC000000),
    )

@Composable
fun PlumTvTheme(serverBaseUrl: String = "", content: @Composable () -> Unit) {
    androidx.compose.material3.MaterialTheme {
        MaterialTheme(
            colorScheme = plumColorScheme,
            shapes = plumShapes,
            typography = plumTypography,
        ) {
            androidx.compose.runtime.CompositionLocalProvider(
                LocalPlumPalette provides plumPalette,
                LocalPlumMetrics provides plumMetrics,
                LocalServerBaseUrl provides serverBaseUrl,
                content = content,
            )
        }
    }
}

object PlumTheme {
    val palette: PlumTvPalette
        @Composable
        @ReadOnlyComposable
        get() = LocalPlumPalette.current

    val metrics: PlumTvMetrics
        @Composable
        @ReadOnlyComposable
        get() = LocalPlumMetrics.current

    val typography: Typography
        @Composable
        @ReadOnlyComposable
        get() = MaterialTheme.typography

    val colors: ColorScheme
        @Composable
        @ReadOnlyComposable
        get() = MaterialTheme.colorScheme
}

@Composable
fun PlumTvScaffold(
    modifier: Modifier = Modifier,
    content: @Composable BoxScope.() -> Unit,
) {
    val palette = PlumTheme.palette
    Box(
        modifier =
            modifier
                .fillMaxSize()
                .background(palette.background),
    ) {
        // Subtle purple corner wash — confined to ~half the screen so we don't run a full-viewport
        // radial shader on low-end TV SoCs (same look at a glance, less blended area).
        val density = LocalDensity.current
        BoxWithConstraints(Modifier.fillMaxSize()) {
            val radiusPx =
                with(density) {
                    (maxOf(maxWidth, maxHeight) * 0.72f).toPx()
                }
            Box(
                modifier =
                    Modifier
                        .align(Alignment.TopStart)
                        .width(maxWidth * 0.58f)
                        .height(maxHeight * 0.48f)
                        .background(
                            brush =
                                Brush.radialGradient(
                                    colors = listOf(palette.accent.copy(alpha = 0.07f), Color.Transparent),
                                    center = Offset.Zero,
                                    radius = radiusPx,
                                ),
                        ),
            )
        }
        content()
    }
}

@Composable
fun PlumScreenPadding(): PaddingValues = PlumTheme.metrics.screenPadding

fun plumBorder(color: Color, width: Dp, shape: RoundedCornerShape): androidx.tv.material3.Border =
    androidx.tv.material3.Border(
        border = BorderStroke(width, color),
        inset = 0.dp,
        shape = shape,
    )
