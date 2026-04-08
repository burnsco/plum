package plum.tv.core.network

import com.squareup.moshi.Json
import com.squareup.moshi.JsonClass
import retrofit2.Response
import retrofit2.http.Body
import retrofit2.http.DELETE
import retrofit2.http.GET
import retrofit2.http.PATCH
import retrofit2.http.POST
import retrofit2.http.PUT
import retrofit2.http.Path
import retrofit2.http.Query

@JsonClass(generateAdapter = true)
data class DeviceLoginRequest(
    @Json(name = "email") val email: String,
    @Json(name = "password") val password: String,
)

@JsonClass(generateAdapter = true)
data class QuickConnectRedeemRequest(
    @Json(name = "code") val code: String,
)

@JsonClass(generateAdapter = true)
data class UserJson(
    @Json(name = "id") val id: Int,
    @Json(name = "email") val email: String,
    @Json(name = "is_admin") val isAdmin: Boolean,
)

@JsonClass(generateAdapter = true)
data class DeviceLoginResponseJson(
    @Json(name = "user") val user: UserJson,
    @Json(name = "sessionToken") val sessionToken: String,
    @Json(name = "expiresAt") val expiresAt: String,
)

interface PlumApi {
    @POST("/api/auth/device-login")
    suspend fun deviceLogin(@Body body: DeviceLoginRequest): Response<DeviceLoginResponseJson>

    @POST("/api/auth/quick-connect/redeem")
    suspend fun redeemQuickConnect(@Body body: QuickConnectRedeemRequest): Response<DeviceLoginResponseJson>

    @POST("/api/auth/logout")
    suspend fun logout(): Response<Unit>

    @GET("/api/auth/me")
    suspend fun me(): Response<UserJson>

    @GET("/api/home")
    suspend fun homeDashboard(): Response<HomeDashboardJson>

    @GET("/api/libraries")
    suspend fun libraries(): Response<List<LibraryJson>>

    @GET("/api/libraries/{id}/media")
    suspend fun libraryMedia(
        @Path("id") libraryId: Int,
        @Query("offset") offset: Int? = null,
        @Query("limit") limit: Int? = null,
    ): Response<LibraryMediaPageJson>

    @GET("/api/libraries/{id}/scan")
    suspend fun libraryScanStatus(@Path("id") libraryId: Int): Response<LibraryScanStatusJson>

    @GET("/api/libraries/{libraryId}/movies/{mediaId}")
    suspend fun movieDetails(
        @Path("libraryId") libraryId: Int,
        @Path("mediaId") mediaId: Int,
    ): Response<LibraryMovieDetailsJson>

    @GET("/api/libraries/{libraryId}/shows/{showKey}/details")
    suspend fun showDetails(
        @Path("libraryId") libraryId: Int,
        @Path("showKey", encoded = true) showKey: String,
    ): Response<LibraryShowDetailsJson>

    @GET("/api/libraries/{libraryId}/shows/{showKey}/episodes")
    suspend fun showEpisodes(
        @Path("libraryId") libraryId: Int,
        @Path("showKey", encoded = true) showKey: String,
    ): Response<ShowEpisodesResponseJson>

    @PUT("/api/media/{id}/progress")
    suspend fun updateMediaProgress(
        @Path("id") mediaId: Int,
        @Body body: UpdateMediaProgressPayloadJson,
    ): Response<Unit>

    @POST("/api/playback/sessions/{id}")
    suspend fun createPlaybackSession(
        @Path("id") mediaId: Int,
        @Body body: CreatePlaybackSessionPayloadJson,
    ): Response<PlaybackSessionJson>

    @PATCH("/api/playback/sessions/{sessionId}/audio")
    suspend fun updatePlaybackSessionAudio(
        @Path("sessionId") sessionId: String,
        @Body body: UpdatePlaybackSessionAudioPayloadJson,
    ): Response<PlaybackSessionJson>

    @DELETE("/api/playback/sessions/{sessionId}")
    suspend fun closePlaybackSession(@Path("sessionId") sessionId: String): Response<Unit>

    @GET("/api/search")
    suspend fun searchLibraryMedia(
        @Query("q") q: String,
        @Query("library_id") libraryId: Int? = null,
        @Query("limit") limit: Int? = null,
        @Query("type") mediaType: String? = null,
        @Query("genre") genre: String? = null,
    ): Response<SearchResponseJson>

    @GET("/api/discover")
    suspend fun discover(
        @Query("origin_country") originCountry: String? = null,
    ): Response<DiscoverResponseJson>

    @GET("/api/discover/genres")
    suspend fun discoverGenres(): Response<DiscoverGenresResponseJson>

    @GET("/api/discover/browse")
    suspend fun browseDiscover(
        @Query("category") category: String? = null,
        @Query("media_type") mediaType: String? = null,
        @Query("genre") genreId: Int? = null,
        @Query("page") page: Int? = null,
        @Query("origin_country") originCountry: String? = null,
    ): Response<DiscoverBrowseResponseJson>

    @GET("/api/discover/search")
    suspend fun searchDiscover(@Query("q") query: String): Response<DiscoverSearchResponseJson>

    @GET("/api/discover/{mediaType}/{tmdbId}")
    suspend fun discoverTitleDetails(
        @Path("mediaType") mediaType: String,
        @Path("tmdbId") tmdbId: Int,
    ): Response<DiscoverTitleDetailsJson>

    @POST("/api/discover/{mediaType}/{tmdbId}/add")
    suspend fun addDiscoverTitle(
        @Path("mediaType") mediaType: String,
        @Path("tmdbId") tmdbId: Int,
    ): Response<DiscoverAcquisitionJson>

    @GET("/api/downloads")
    suspend fun downloads(): Response<DownloadsResponseJson>

    @POST("/api/downloads/remove")
    suspend fun removeDownload(@Body body: RemoveDownloadPayloadJson): Response<Unit>
}
