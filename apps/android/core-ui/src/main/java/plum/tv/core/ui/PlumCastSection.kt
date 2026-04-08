package plum.tv.core.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.tv.material3.Text
import coil3.compose.AsyncImage

data class PlumCastMember(
    val name: String,
    val character: String? = null,
    val profilePath: String? = null,
    val profileUrl: String? = null,
)

/** Compact horizontal cast strip — circular headshots with name + role underneath. */
@Composable
fun PlumCastSection(
    cast: List<PlumCastMember>,
    serverBase: String,
    modifier: Modifier = Modifier,
    title: String = "Cast",
) {
    val palette = PlumTheme.palette
    Column(modifier = modifier, verticalArrangement = Arrangement.spacedBy(8.dp)) {
        PlumSectionHeader(title = title)
        LazyRow(
            horizontalArrangement = Arrangement.spacedBy(14.dp),
            contentPadding = PaddingValues(vertical = 4.dp),
        ) {
            items(cast.take(20), key = { it.name }) { member ->
                val photo = resolveArtworkUrl(
                    serverBase,
                    member.profileUrl,
                    member.profilePath,
                    PlumImageSizes.THUMB_SMALL,
                )
                Column(
                    modifier = Modifier.width(72.dp),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.spacedBy(4.dp),
                ) {
                    Box(
                        modifier = Modifier
                            .size(56.dp)
                            .clip(CircleShape),
                    ) {
                        if (!photo.isNullOrBlank()) {
                            AsyncImage(
                                model = photo,
                                contentDescription = member.name,
                                modifier = Modifier.fillMaxSize(),
                                contentScale = ContentScale.Crop,
                            )
                        } else {
                            Box(
                                modifier = Modifier
                                    .fillMaxSize()
                                    .background(palette.surface),
                                contentAlignment = Alignment.Center,
                            ) {
                                Text(
                                    text = member.name.trim().firstOrNull()
                                        ?.uppercaseChar()?.toString() ?: "?",
                                    style = PlumTheme.typography.titleSmall,
                                    fontWeight = FontWeight.Bold,
                                    color = palette.muted,
                                )
                            }
                        }
                    }
                    Text(
                        text = member.name,
                        style = PlumTheme.typography.labelSmall,
                        color = palette.text,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        textAlign = TextAlign.Center,
                    )
                    member.character?.takeIf { it.isNotBlank() }?.let { character ->
                        Text(
                            text = character,
                            style = PlumTheme.typography.labelSmall,
                            color = palette.muted,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                            textAlign = TextAlign.Center,
                        )
                    }
                }
            }
        }
    }
}
