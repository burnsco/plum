package plum.tv.feature.details

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.tv.material3.Text
import plum.tv.core.ui.PlumCastMember
import plum.tv.core.ui.LocalServerBaseUrl
import plum.tv.core.ui.PlumActionButton
import plum.tv.core.ui.PlumButtonVariant
import plum.tv.core.ui.PlumDetailBackground
import plum.tv.core.ui.PlumDetailHeroHeader
import plum.tv.core.ui.PlumImageSizes
import plum.tv.core.ui.PlumCastSection
import plum.tv.core.ui.PlumMetadataChips
import plum.tv.core.ui.PlumScrims
import plum.tv.core.ui.PlumStatePanel
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
            PlumStatePanel(
                title = "Loading",
                message = "Fetching movie details…",
            )
        }
        is MovieDetailUiState.Error -> Box(
            Modifier.fillMaxSize(),
            contentAlignment = Alignment.Center,
        ) {
            PlumStatePanel(
                title = "Could not load movie",
                message = s.message,
                actions = {
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        PlumActionButton("Retry", onClick = { viewModel.load() }, leadingBadge = "R")
                        PlumActionButton("Back", onClick = onBack, variant = PlumButtonVariant.Ghost)
                    }
                },
            )
        }
        is MovieDetailUiState.Ready -> {
            val d = s.details
            val backdropUrl =
                resolveArtworkUrl(serverBase, d.backdropUrl, d.backdropPath, PlumImageSizes.BACKDROP_HERO)
            val posterUrl = resolveArtworkUrl(serverBase, d.posterUrl, d.posterPath, PlumImageSizes.POSTER_DETAIL)

            PlumDetailBackground(
                backdropUrl = backdropUrl,
                scrim = PlumScrims.backdropHorizontal,
            ) {
                Column(
                    modifier = Modifier
                        .fillMaxSize()
                        .verticalScroll(rememberScrollState())
                        .padding(horizontal = 36.dp, vertical = 32.dp),
                    verticalArrangement = Arrangement.spacedBy(28.dp),
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

                    if (!d.cast.isNullOrEmpty()) {
                        PlumCastSection(
                            cast = d.cast.orEmpty().map { member ->
                                PlumCastMember(
                                    name = member.name,
                                    character = member.character,
                                )
                            },
                        )
                    }
                }
            }
        }
    }
}
