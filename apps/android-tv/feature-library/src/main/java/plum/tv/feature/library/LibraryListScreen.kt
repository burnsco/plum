package plum.tv.feature.library

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.tv.material3.Button
import androidx.tv.material3.Card
import androidx.tv.material3.CardDefaults
import androidx.tv.material3.ExperimentalTvMaterial3Api
import androidx.tv.material3.Text

@OptIn(ExperimentalTvMaterial3Api::class)
@Composable
fun LibraryListRoute(
    onOpenLibrary: (libraryId: Int) -> Unit,
    viewModel: LibraryListViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    when (val s = state) {
        is LibraryListUiState.Loading -> Text("Loading libraries…", modifier = Modifier.padding(48.dp))
        is LibraryListUiState.Error -> Column(Modifier.padding(48.dp)) {
            Text(s.message)
            Button(onClick = { viewModel.refresh() }) { Text("Retry") }
        }
        is LibraryListUiState.Ready -> LazyVerticalGrid(
            columns = GridCells.Fixed(3),
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(48.dp),
            horizontalArrangement = Arrangement.spacedBy(24.dp),
            verticalArrangement = Arrangement.spacedBy(24.dp),
        ) {
            items(s.libraries, key = { it.id }) { lib ->
                Card(
                    onClick = { onOpenLibrary(lib.id) },
                    modifier = Modifier
                        .padding(4.dp)
                        .fillMaxWidth()
                        .height(120.dp),
                    scale = CardDefaults.scale(focusedScale = 1.06f),
                ) {
                    Column(Modifier.padding(24.dp)) {
                        Text(lib.name)
                        Text(lib.type)
                    }
                }
            }
        }
    }
}
