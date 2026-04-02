package plum.tv.core.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.ReadOnlyComposable
import androidx.compose.runtime.compositionLocalOf
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
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
    val thumbnailWidth: Dp,
    val thumbnailHeight: Dp,
    val panelRadius: Dp,
    val tileRadius: Dp,
    val buttonRadius: Dp,
    val railWidth: Dp,
)

private val plumPalette =
    PlumTvPalette(
        sidebar = Color(0xFF0C0C14),
        topbar = Color(0xFF0C0C14),
        background = Color(0xFF090910),   // near-black with blue tint — cinematic
        panel = Color(0xF0111118),
        panelAlt = Color(0xF014141E),
        surface = Color(0xE61A1A28),
        surfaceHover = Color(0xEB20202E),
        accent = Color(0xFFCFB1FF),
        accentSoft = Color(0x2DB57BFF),
        accentGlow = Color(0x47B57BFF),
        accentSecondary = Color(0xFFE879C0),
        text = Color(0xFFF0EEFF),
        textSecondary = Color(0xFFD8D3F2),
        muted = Color(0xFF9990BE),
        border = Color(0x14B57BFF),
        borderStrong = Color(0x30B57BFF),
        ring = Color(0x70B57BFF),
        success = Color(0xFF4ADE80),
        warning = Color(0xFFFBBF24),
        error = Color(0xFFF87171),
    )

private val plumMetrics =
    PlumTvMetrics(
        screenPadding = PaddingValues(horizontal = 36.dp, vertical = 24.dp),
        railPadding = PaddingValues(horizontal = 0.dp, vertical = 20.dp),
        sectionGap = 32.dp,
        cardGap = 12.dp,
        chipGap = 8.dp,
        posterWidth = 140.dp,
        posterHeight = 172.dp,
        posterCompactWidth = 120.dp,
        posterCompactHeight = 148.dp,
        heroPosterWidth = 200.dp,
        heroPosterHeight = 248.dp,
        thumbnailWidth = 160.dp,
        thumbnailHeight = 90.dp,
        panelRadius = 14.dp,
        tileRadius = 10.dp,
        buttonRadius = 999.dp,
        railWidth = 196.dp,
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
        // Subtle purple radial glow in the top-left corner
        Box(
            modifier =
                Modifier
                    .fillMaxSize()
                    .background(
                        brush =
                            Brush.radialGradient(
                                colors = listOf(palette.accent.copy(alpha = 0.07f), Color.Transparent),
                                radius = 1400f,
                            ),
                    ),
        )
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
