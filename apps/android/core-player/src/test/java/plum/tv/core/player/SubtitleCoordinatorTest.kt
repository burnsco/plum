package plum.tv.core.player

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import plum.tv.core.network.EmbeddedSubtitleDeliveryModeJson
import plum.tv.core.network.EmbeddedSubtitleJson
import plum.tv.core.network.SubtitleJson

class SubtitleCoordinatorTest {
    private val coordinator = SubtitleCoordinator()

    @Test
    fun logicalIdForSidecar_prefersWireValueAndKeepsFallback() {
        assertEquals(
            "ext:11",
            coordinator.logicalIdForSidecar(SubtitleJson(id = 11, logicalId = "ext:11")),
        )
        assertEquals(
            "ext:12",
            coordinator.logicalIdForSidecar(SubtitleJson(id = 12)),
        )
    }

    @Test
    fun buildPickerOptions_dropsDemuxedDuplicateOfSideload() {
        val options =
            coordinator.buildPickerOptions(
                SubtitlePickerBuildInput(
                    textDisabled = false,
                    textTracks =
                        listOf(
                            SubtitleTextTrackCandidate(
                                groupIndex = 0,
                                trackIndex = 0,
                                pickerId = "t:0:0",
                                logicalId = "emb:7",
                                label = "English",
                                detail = "WEBVTT",
                                selected = true,
                                sideLoadPriority = 300,
                                renderKind = SubtitleLogicalRenderKind.TextCue,
                                isCeaClosedCaption = false,
                            ),
                            SubtitleTextTrackCandidate(
                                groupIndex = 1,
                                trackIndex = 0,
                                pickerId = "t:1:0",
                                logicalId = "emb:7",
                                label = "English",
                                detail = "SUBRIP",
                                selected = false,
                                sideLoadPriority = 0,
                                renderKind = SubtitleLogicalRenderKind.TextCue,
                                isCeaClosedCaption = false,
                            ),
                        ),
                    burnTracks = emptyList(),
                ),
            )

        assertEquals(listOf("off", "t:0:0"), options.map { it.id })
    }

    @Test
    fun buildPickerOptions_keepsDemuxedFallbackWhenLabelDiffersFromSideload() {
        // When the manifest-only row has a different label/render kind than the sideload, it is
        // not a duplicate — keep it so the user can still pick a usable track.
        val options =
            coordinator.buildPickerOptions(
                SubtitlePickerBuildInput(
                    textDisabled = false,
                    textTracks =
                        listOf(
                            SubtitleTextTrackCandidate(
                                groupIndex = 0,
                                trackIndex = 0,
                                pickerId = "t:0:0",
                                logicalId = null,
                                label = "Spanish",
                                detail = "WEBVTT",
                                selected = true,
                                sideLoadPriority = 0,
                                renderKind = SubtitleLogicalRenderKind.TextCue,
                                isCeaClosedCaption = false,
                            ),
                            SubtitleTextTrackCandidate(
                                groupIndex = 1,
                                trackIndex = 0,
                                pickerId = "t:1:0",
                                logicalId = "ext:7",
                                label = "English",
                                detail = "WEBVTT",
                                selected = false,
                                sideLoadPriority = 300,
                                renderKind = SubtitleLogicalRenderKind.TextCue,
                                isCeaClosedCaption = false,
                            ),
                        ),
                    burnTracks = emptyList(),
                ),
            )

        assertEquals(listOf("off", "t:0:0", "t:1:0"), options.map { it.id })
        assertEquals("WEBVTT", options[1].detail)
        assertEquals("WEBVTT · sideload", options[2].detail)
    }

    @Test
    fun buildPickerOptions_fallsBackToCeaWhenThatIsAllExoReported() {
        val options =
            coordinator.buildPickerOptions(
                SubtitlePickerBuildInput(
                    textDisabled = false,
                    textTracks =
                        listOf(
                            SubtitleTextTrackCandidate(
                                groupIndex = 0,
                                trackIndex = 0,
                                pickerId = "t:0:0",
                                logicalId = null,
                                label = "CEA-608",
                                detail = "CEA608",
                                selected = true,
                                sideLoadPriority = 0,
                                renderKind = SubtitleLogicalRenderKind.TextCue,
                                isCeaClosedCaption = true,
                            ),
                        ),
                    burnTracks = emptyList(),
                ),
            )

        assertEquals(listOf("off", "t:0:0"), options.map { it.id })
    }

    @Test
    fun isBurnInEmbeddedTrack_prefersNewDeliveryContract() {
        val burnOnly =
            EmbeddedSubtitleJson(
                streamIndex = 8,
                language = "en",
                title = "English",
                codec = "dvd_subtitle",
                supported = true,
                vttEligible = false,
                pgsBinaryEligible = false,
                deliveryModes = listOf(EmbeddedSubtitleDeliveryModeJson(mode = "burn_in", requiresReload = true)),
                preferredAndroidDeliveryMode = "burn_in",
            )

        assertTrue(coordinator.isBurnInEmbeddedTrack(burnOnly))
    }

    @Test
    fun isBurnInEmbeddedTrack_skipsWhenTextDeliveryAlsoAvailable() {
        // Anime ASS streams are vttEligible AND list burn_in as a fallback delivery; the picker
        // must not render both a text row and a burn-in row for the same source stream.
        val textPlusBurnFallback =
            EmbeddedSubtitleJson(
                streamIndex = 3,
                language = "en",
                title = "English",
                codec = "ass",
                supported = true,
                vttEligible = true,
                pgsBinaryEligible = false,
                deliveryModes =
                    listOf(
                        EmbeddedSubtitleDeliveryModeJson(mode = "direct_vtt", requiresReload = false),
                        EmbeddedSubtitleDeliveryModeJson(mode = "burn_in", requiresReload = true),
                    ),
                preferredAndroidDeliveryMode = "burn_in",
            )

        assertFalse(coordinator.isBurnInEmbeddedTrack(textPlusBurnFallback))
    }

    @Test
    fun isBurnInEmbeddedTrack_keepsPgsBurnFallbackVisible() {
        val pgsWithBurnFallback =
            EmbeddedSubtitleJson(
                streamIndex = 8,
                language = "en",
                title = "English PGS",
                codec = "hdmv_pgs_subtitle",
                supported = true,
                vttEligible = false,
                pgsBinaryEligible = true,
                deliveryModes =
                    listOf(
                        EmbeddedSubtitleDeliveryModeJson(mode = "pgs_binary", requiresReload = false),
                        EmbeddedSubtitleDeliveryModeJson(mode = "burn_in", requiresReload = true),
                    ),
                preferredAndroidDeliveryMode = "pgs_binary",
            )

        assertTrue(coordinator.isBurnInEmbeddedTrack(pgsWithBurnFallback))
    }

    @Test
    fun buildPickerOptions_dropsLogicalIdLessDemuxedDuplicateNextToSideload() {
        // Android may report both manifest/demuxed fallback rows and Plum sideload rows for the
        // same logical subtitles. Show the reliable sideload rows only.
        val options =
            coordinator.buildPickerOptions(
                SubtitlePickerBuildInput(
                    textDisabled = false,
                    textTracks =
                        listOf(
                            SubtitleTextTrackCandidate(
                                groupIndex = 0,
                                trackIndex = 0,
                                pickerId = "t:0:0",
                                logicalId = null,
                                label = "English",
                                detail = "WEBVTT",
                                selected = false,
                                sideLoadPriority = 0,
                                renderKind = SubtitleLogicalRenderKind.TextCue,
                                isCeaClosedCaption = false,
                            ),
                            SubtitleTextTrackCandidate(
                                groupIndex = 1,
                                trackIndex = 0,
                                pickerId = "t:1:0",
                                logicalId = "emb:3",
                                label = "English",
                                detail = "WEBVTT",
                                selected = true,
                                sideLoadPriority = 300,
                                renderKind = SubtitleLogicalRenderKind.TextCue,
                                isCeaClosedCaption = false,
                            ),
                            SubtitleTextTrackCandidate(
                                groupIndex = 2,
                                trackIndex = 0,
                                pickerId = "t:2:0",
                                logicalId = null,
                                label = "English (SDH)",
                                detail = "WEBVTT",
                                selected = false,
                                sideLoadPriority = 0,
                                renderKind = SubtitleLogicalRenderKind.TextCue,
                                isCeaClosedCaption = false,
                            ),
                            SubtitleTextTrackCandidate(
                                groupIndex = 3,
                                trackIndex = 0,
                                pickerId = "t:3:0",
                                logicalId = "emb:4",
                                label = "English (SDH)",
                                detail = "WEBVTT",
                                selected = false,
                                sideLoadPriority = 300,
                                renderKind = SubtitleLogicalRenderKind.TextCue,
                                isCeaClosedCaption = false,
                            ),
                        ),
                    burnTracks = emptyList(),
                ),
            )

        assertEquals(listOf("off", "t:1:0", "t:3:0"), options.map { it.id })
        assertEquals("WEBVTT · sideload", options[1].detail)
        assertEquals("WEBVTT · sideload", options[2].detail)
    }

    @Test
    fun buildPickerOptions_dropsMultipleSameLabelFallbackRowsWhenSideloadExists() {
        val options =
            coordinator.buildPickerOptions(
                SubtitlePickerBuildInput(
                    textDisabled = false,
                    textTracks =
                        listOf(
                            SubtitleTextTrackCandidate(
                                groupIndex = 0,
                                trackIndex = 0,
                                pickerId = "t:0:0",
                                logicalId = null,
                                label = "English",
                                detail = "WEBVTT",
                                selected = false,
                                sideLoadPriority = 0,
                                renderKind = SubtitleLogicalRenderKind.TextCue,
                                isCeaClosedCaption = false,
                            ),
                            SubtitleTextTrackCandidate(
                                groupIndex = 1,
                                trackIndex = 0,
                                pickerId = "t:1:0",
                                logicalId = null,
                                label = "English",
                                detail = "WEBVTT",
                                selected = false,
                                sideLoadPriority = 0,
                                renderKind = SubtitleLogicalRenderKind.TextCue,
                                isCeaClosedCaption = false,
                            ),
                            SubtitleTextTrackCandidate(
                                groupIndex = 2,
                                trackIndex = 0,
                                pickerId = "t:2:0",
                                logicalId = "emb:3",
                                label = "English",
                                detail = "WEBVTT",
                                selected = true,
                                sideLoadPriority = 300,
                                renderKind = SubtitleLogicalRenderKind.TextCue,
                                isCeaClosedCaption = false,
                            ),
                        ),
                    burnTracks = emptyList(),
                ),
            )

        assertEquals(listOf("off", "t:2:0"), options.map { it.id })
    }

    @Test
    fun buildPickerOptions_preservesSelectionWhenSelectedFallbackDedupesToSideload() {
        val options =
            coordinator.buildPickerOptions(
                SubtitlePickerBuildInput(
                    textDisabled = false,
                    textTracks =
                        listOf(
                            SubtitleTextTrackCandidate(
                                groupIndex = 0,
                                trackIndex = 0,
                                pickerId = "t:0:0",
                                logicalId = null,
                                label = "English",
                                detail = "WEBVTT",
                                selected = true,
                                sideLoadPriority = 0,
                                renderKind = SubtitleLogicalRenderKind.TextCue,
                                isCeaClosedCaption = false,
                            ),
                            SubtitleTextTrackCandidate(
                                groupIndex = 1,
                                trackIndex = 0,
                                pickerId = "t:1:0",
                                logicalId = "emb:3",
                                label = "English",
                                detail = "WEBVTT",
                                selected = false,
                                sideLoadPriority = 300,
                                renderKind = SubtitleLogicalRenderKind.TextCue,
                                isCeaClosedCaption = false,
                            ),
                        ),
                    burnTracks = emptyList(),
                ),
            )

        assertEquals(listOf("off", "t:1:0"), options.map { it.id })
        assertEquals(false, options[0].selected)
        assertEquals(true, options[1].selected)
    }

    @Test
    fun resolveSelectionAction_switchingTextWhileBurningReloadsWithoutBurnAndRestoresTrack() {
        val action =
            coordinator.resolveSelectionAction(
                currentBurnStreamIndex = 9,
                trackId = SubtitlePickerTrackId.TextTrack(groupIndex = 2, trackIndex = 1),
                selectedTextRestore =
                    SubtitleRestorePlan(
                        disabled = false,
                        language = "eng",
                        label = "English",
                        configurationId = "emb:7",
                    ),
            )

        val reload = action as SubtitleSelectionAction.ReloadWithoutBurn
        assertFalse(reload.restore.disabled)
        assertEquals("emb:7", reload.restore.configurationId)
    }

    @Test
    fun resolveSelectionAction_offWhileBurningReloadsWithoutBurnAndDisablesText() {
        val action =
            coordinator.resolveSelectionAction(
                currentBurnStreamIndex = 4,
                trackId = SubtitlePickerTrackId.Off,
                selectedTextRestore = null,
            )

        val reload = action as SubtitleSelectionAction.ReloadWithoutBurn
        assertTrue(reload.restore.disabled)
    }

    @Test
    fun resolveSelectionAction_sameBurnTrackIsNoOp() {
        val action =
            coordinator.resolveSelectionAction(
                currentBurnStreamIndex = 12,
                trackId = SubtitlePickerTrackId.BurnIn(streamIndex = 12),
                selectedTextRestore = null,
            )

        assertTrue(action is SubtitleSelectionAction.NoOp)
    }
}
