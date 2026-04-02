package plum.tv.feature.details

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.tv.material3.Text
import coil.compose.AsyncImage
import plum.tv.core.network.TitleCastMemberJson
import plum.tv.core.ui.LocalServerBaseUrl
import plum.tv.core.ui.PlumActionButton
import plum.tv.core.ui.PlumButtonVariant
import plum.tv.core.ui.PlumMetadataChips
import plum.tv.core.ui.PlumPanel
import plum.tv.core.ui.PlumSectionHeader
import plum.tv.core.ui.PlumTheme
import plum.tv.core.ui.resolveArtworkUrl

@Composable
fun MovieDetailRoute(
    onBack: () -> Unit,
    onPlay: (mediaId: Int) -> Unit,
    viewModel: MovieDetailViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val serverBase = LocalServerBaseUrl.current

    when (val s = state) {
        is MovieDetailUiState.Loading -> Box(
            Modifier.fillMaxSize(),
            contentAlignment = Alignment.Center,
        ) {
            Text("Loading…", color = PlumTheme.palette.muted)
        }
        is MovieDetailUiState.Error -> Column(
            Modifier.fillMaxSize().padding(48.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            Text(s.message, color = PlumTheme.palette.muted)
            PlumActionButton("Retry", onClick = { viewModel.load() }, leadingBadge = "R")
            PlumActionButton("Back", onClick = onBack, variant = PlumButtonVariant.Ghost, leadingBadge = "B")
        }
        is MovieDetailUiState.Ready -> {
            val d = s.details
            val backdropUrl = resolveArtworkUrl(serverBase, d.backdropUrl, d.backdropPath, "w780")
            val posterUrl = resolveArtworkUrl(serverBase, d.posterUrl, d.posterPath, "w500")

            Box(modifier = Modifier.fillMaxSize()) {
                // Full-bleed backdrop
                if (backdropUrl != null) {
                    AsyncImage(
                        model = backdropUrl,
                        contentDescription = null,
                        modifier = Modifier.fillMaxSize(),
                        contentScale = ContentScale.Crop,
                    )
                }
                // Scrim over backdrop so text is readable
                Box(
                    modifier = Modifier
                        .fillMaxSize()
                        .background(
                            Brush.horizontalGradient(
                                0.0f to Color(0xF2000000),
                                0.55f to Color(0xCC000000),
                                1.0f to Color(0x44000000),
                            ),
                        ),
                )

                // Content layer
                Column(
                    modifier = Modifier
                        .fillMaxSize()
                        .verticalScroll(rememberScrollState())
                        .padding(horizontal = 36.dp, vertical = 32.dp),
                    verticalArrangement = Arrangement.spacedBy(28.dp),
                ) {
                    // Header: poster + info side by side
                    Row(horizontalArrangement = Arrangement.spacedBy(32.dp)) {
                        // Poster
                        if (posterUrl != null) {
                            AsyncImage(
                                model = posterUrl,
                                contentDescription = d.title,
                                modifier = Modifier
                                    .width(200.dp)
                                    .height(300.dp)
                                    .clip(RoundedCornerShape(10.dp)),
                                contentScale = ContentScale.Crop,
                            )
                        }

                        // Info column
                        Column(
                            modifier = Modifier.weight(1f),
                            verticalArrangement = Arrangement.spacedBy(16.dp),
                        ) {
                            Text(
                                text = d.title,
                                style = PlumTheme.typography.headlineLarge,
                                color = Color.White,
                                fontWeight = FontWeight.Bold,
                            )

                            PlumMetadataChips(
                                values = buildList {
                                    d.releaseDate?.take(4)?.takeIf { it.isNotBlank() }?.let(::add)
                                    d.runtime?.takeIf { it > 0 }?.let { add("${it} min") }
                                    d.imdbRating?.let { add("★ ${"%.1f".format(it)}") }
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

                            Spacer(modifier = Modifier.height(4.dp))

                            Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                                PlumActionButton("Play", onClick = { onPlay(d.mediaId) }, leadingBadge = "▶")
                                PlumActionButton("Back", onClick = onBack, variant = PlumButtonVariant.Secondary)
                            }
                        }
                    }

                    // Cast section
                    if (d.cast.isNotEmpty()) {
                        CastSection(cast = d.cast)
                    }
                }
            }
        }
    }
}

@Composable
private fun CastSection(cast: List<TitleCastMemberJson>) {
    PlumPanel(modifier = Modifier.fillMaxWidth()) {
        Column(verticalArrangement = Arrangement.spacedBy(16.dp)) {
            PlumSectionHeader(title = "Cast")
            val rows = cast.take(18).chunked(3)
            rows.forEach { rowItems ->
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(12.dp),
                ) {
                    rowItems.forEach { member ->
                        CastMemberCard(member = member, modifier = Modifier.weight(1f))
                    }
                    // Fill remaining columns so items align left
                    repeat(3 - rowItems.size) {
                        Spacer(modifier = Modifier.weight(1f))
                    }
                }
            }
        }
    }
}

@Composable
private fun CastMemberCard(member: TitleCastMemberJson, modifier: Modifier = Modifier) {
    val palette = PlumTheme.palette
    Box(
        modifier = modifier
            .clip(RoundedCornerShape(PlumTheme.metrics.tileRadius))
            .background(palette.panelAlt)
            .padding(horizontal = 14.dp, vertical = 10.dp),
    ) {
        Column(verticalArrangement = Arrangement.spacedBy(3.dp)) {
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
