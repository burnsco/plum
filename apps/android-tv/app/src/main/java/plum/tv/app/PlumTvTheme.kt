package plum.tv.app

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography
import androidx.compose.runtime.Composable
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp

private val PlumTvTypography =
    Typography(
        displayLarge = TextStyle(fontSize = 64.sp, fontWeight = FontWeight.SemiBold, lineHeight = 72.sp),
        displayMedium = TextStyle(fontSize = 52.sp, fontWeight = FontWeight.SemiBold, lineHeight = 60.sp),
        displaySmall = TextStyle(fontSize = 44.sp, fontWeight = FontWeight.Medium, lineHeight = 52.sp),
        headlineLarge = TextStyle(fontSize = 40.sp, fontWeight = FontWeight.SemiBold, lineHeight = 48.sp),
        headlineMedium = TextStyle(fontSize = 34.sp, fontWeight = FontWeight.SemiBold, lineHeight = 42.sp),
        headlineSmall = TextStyle(fontSize = 28.sp, fontWeight = FontWeight.Medium, lineHeight = 36.sp),
        titleLarge = TextStyle(fontSize = 26.sp, fontWeight = FontWeight.SemiBold, lineHeight = 32.sp),
        titleMedium = TextStyle(fontSize = 22.sp, fontWeight = FontWeight.Medium, lineHeight = 28.sp),
        titleSmall = TextStyle(fontSize = 20.sp, fontWeight = FontWeight.Medium, lineHeight = 26.sp),
        bodyLarge = TextStyle(fontSize = 22.sp, fontWeight = FontWeight.Normal, lineHeight = 30.sp),
        bodyMedium = TextStyle(fontSize = 20.sp, fontWeight = FontWeight.Normal, lineHeight = 28.sp),
        bodySmall = TextStyle(fontSize = 18.sp, fontWeight = FontWeight.Normal, lineHeight = 24.sp),
        labelLarge = TextStyle(fontSize = 20.sp, fontWeight = FontWeight.Medium, lineHeight = 26.sp),
        labelMedium = TextStyle(fontSize = 18.sp, fontWeight = FontWeight.Medium, lineHeight = 24.sp),
        labelSmall = TextStyle(fontSize = 16.sp, fontWeight = FontWeight.Medium, lineHeight = 22.sp),
    )

@Composable
fun PlumTvTheme(content: @Composable () -> Unit) {
    MaterialTheme(typography = PlumTvTypography, content = content)
}
