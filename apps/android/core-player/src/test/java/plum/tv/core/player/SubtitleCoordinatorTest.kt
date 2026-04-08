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
