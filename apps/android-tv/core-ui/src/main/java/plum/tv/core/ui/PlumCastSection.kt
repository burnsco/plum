package plum.tv.core.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.tv.material3.Text
import coil.compose.AsyncImage

data class PlumCastMember(
    val name: String,
    val character: String? = null,
    val profilePath: String? = null,
    val profileUrl: String? = null,
)

@Composable
fun PlumCastSection(
    cast: List<PlumCastMember>,
    serverBase: String,
    modifier: Modifier = Modifier,
    title: String = "Cast",
) {
    PlumPanel(modifier = modifier.fillMaxWidth()) {
        Column(verticalArrangement = Arrangement.spacedBy(16.dp)) {
            PlumSectionHeader(title = title)
            val palette = PlumTheme.palette
            val rows = cast.take(18).chunked(3)
            rows.forEach { rowItems ->
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(12.dp),
                ) {
                    rowItems.forEach { member ->
                        val photo =
                            resolveArtworkUrl(
                                serverBase,
                                member.profileUrl,
                                member.profilePath,
                                PlumImageSizes.THUMB_SMALL,
                            )
                        Row(
                            modifier =
                                Modifier
                                    .weight(1f)
                                    .clip(RoundedCornerShape(PlumTheme.metrics.tileRadius))
                                    .background(palette.panelAlt)
                                    .padding(horizontal = 10.dp, vertical = 8.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(10.dp),
                        ) {
                            Box(
                                modifier =
                                    Modifier
                                        .size(width = 48.dp, height = 72.dp)
                                        .clip(RoundedCornerShape(8.dp)),
                            ) {
                                if (!photo.isNullOrBlank()) {
                                    AsyncImage(
                                        model = photo,
                                        contentDescription = null,
                                        modifier = Modifier.fillMaxSize(),
                                        contentScale = ContentScale.Crop,
                                    )
                                } else {
                                    Box(
                                        modifier =
                                            Modifier
                                                .fillMaxSize()
                                                .background(palette.panel),
                                        contentAlignment = Alignment.Center,
                                    ) {
                                        Text(
                                            text =
                                                member.name.trim().firstOrNull()?.uppercaseChar()?.toString()
                                                    ?: "?",
                                            style = PlumTheme.typography.titleSmall,
                                            fontWeight = FontWeight.Bold,
                                            color = palette.muted,
                                        )
                                    }
                                }
                            }
                            Column(
                                modifier = Modifier.weight(1f),
                                verticalArrangement = Arrangement.spacedBy(3.dp),
                            ) {
                                Text(
                                    text = member.name,
                                    style = PlumTheme.typography.titleSmall,
                                    color = palette.text,
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis,
                                )
                                member.character?.takeIf { it.isNotBlank() }?.let { character ->
                                    Text(
                                        text = character,
                                        style = PlumTheme.typography.bodySmall,
                                        color = palette.muted,
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis,
                                    )
                                }
                            }
                        }
                    }
                    repeat(3 - rowItems.size) {
                        Spacer(modifier = Modifier.weight(1f))
                    }
                }
            }
        }
    }
}
