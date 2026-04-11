package plum.tv.core.player

import java.util.Locale
import plum.tv.core.network.SubtitleJson
import plum.tv.core.network.EmbeddedSubtitleJson

enum class SubtitleLogicalRenderKind {
    TextCue,
    Bitmap,
    Unknown,
}

data class SubtitleTextTrackCandidate(
    val groupIndex: Int,
    val trackIndex: Int,
    val pickerId: String,
    val logicalId: String?,
    val label: String,
    val detail: String?,
    val selected: Boolean,
    val sideLoadPriority: Int,
    val renderKind: SubtitleLogicalRenderKind,
    val isCeaClosedCaption: Boolean,
)

data class SubtitleBurnTrackCandidate(
    val pickerId: String,
    val logicalId: String,
    val streamIndex: Int,
    val label: String,
    val detail: String?,
    val selected: Boolean,
)

data class SubtitlePickerBuildInput(
    val textDisabled: Boolean,
    val textTracks: List<SubtitleTextTrackCandidate>,
    val burnTracks: List<SubtitleBurnTrackCandidate>,
)

data class SubtitleRestorePlan(
    val disabled: Boolean,
    val language: String?,
    val label: String?,
    val configurationId: String?,
)

sealed class SubtitleSelectionAction {
    data object NoOp : SubtitleSelectionAction()

    data object DisableText : SubtitleSelectionAction()

    data class ApplyTextTrack(val groupIndex: Int, val trackIndex: Int) : SubtitleSelectionAction()

    data class ReloadWithoutBurn(val restore: SubtitleRestorePlan) : SubtitleSelectionAction()

    data class ReloadWithBurn(val streamIndex: Int, val restore: SubtitleRestorePlan) : SubtitleSelectionAction()
}

class SubtitleCoordinator {
    fun logicalIdForSidecar(subtitle: SubtitleJson): String =
        subtitle.logicalId?.takeIf { it.isNotBlank() } ?: "ext:${subtitle.id}"

    fun buildPickerOptions(input: SubtitlePickerBuildInput): List<TrackPickerOption> {
        val visibleTextTracks = filterVisibleTextTracks(input.textTracks)
        var anyTrackSelected = false
        val options = mutableListOf<TrackPickerOption>()

        val offSelected = input.textDisabled || (visibleTextTracks.none { it.selected } && input.burnTracks.none { it.selected })
        options +=
            TrackPickerOption(
                id = SubtitlePickerTrackId.Off.toWireId(),
                label = "Off",
                selected = offSelected,
                detail = "Hide subtitles",
            )

        for (track in visibleTextTracks) {
            if (track.selected) {
                anyTrackSelected = true
            }
            options +=
                TrackPickerOption(
                    id = track.pickerId,
                    label = track.label,
                    selected = track.selected,
                    detail = track.detailWithSourceTag(visibleTextTracks),
                )
        }

        for (track in input.burnTracks) {
            if (track.selected) {
                anyTrackSelected = true
            }
            options +=
                TrackPickerOption(
                    id = track.pickerId,
                    label = track.label,
                    selected = track.selected,
                    detail = track.detail,
                )
        }

        if (!anyTrackSelected && options.isNotEmpty()) {
            options[0] = options[0].copy(selected = true)
        }
        return options
    }

    fun resolveSelectionAction(
        currentBurnStreamIndex: Int?,
        trackId: SubtitlePickerTrackId,
        selectedTextRestore: SubtitleRestorePlan?,
    ): SubtitleSelectionAction =
        when (trackId) {
            SubtitlePickerTrackId.Off ->
                if (currentBurnStreamIndex != null) {
                    SubtitleSelectionAction.ReloadWithoutBurn(
                        restore = SubtitleRestorePlan(disabled = true, language = null, label = null, configurationId = null),
                    )
                } else {
                    SubtitleSelectionAction.DisableText
                }
            is SubtitlePickerTrackId.BurnIn ->
                if (currentBurnStreamIndex == trackId.streamIndex) {
                    SubtitleSelectionAction.NoOp
                } else {
                    SubtitleSelectionAction.ReloadWithBurn(
                        streamIndex = trackId.streamIndex,
                        restore = SubtitleRestorePlan(disabled = true, language = null, label = null, configurationId = null),
                    )
                }
            is SubtitlePickerTrackId.TextTrack ->
                if (currentBurnStreamIndex != null) {
                    SubtitleSelectionAction.ReloadWithoutBurn(
                        restore =
                            selectedTextRestore
                                ?: SubtitleRestorePlan(
                                    disabled = false,
                                    language = null,
                                    label = null,
                                    configurationId = null,
                                ),
                    )
                } else {
                    SubtitleSelectionAction.ApplyTextTrack(trackId.groupIndex, trackId.trackIndex)
                }
        }

    fun isBurnInEmbeddedTrack(subtitle: EmbeddedSubtitleJson): Boolean {
        if (subtitle.supported == false) return false
        if (subtitle.preferredAndroidDeliveryMode == "burn_in") {
            return true
        }
        if (subtitle.deliveryModes?.any { it.mode == "burn_in" } == true) {
            return !subtitle.vttEligible && !subtitle.pgsBinaryEligible
        }
        return !subtitle.vttEligible && !subtitle.pgsBinaryEligible
    }

    fun logicalIdForEmbedded(subtitle: EmbeddedSubtitleJson): String =
        subtitle.logicalId?.takeIf { it.isNotBlank() } ?: "emb:${subtitle.streamIndex}"

    private fun filterVisibleTextTracks(
        textTracks: List<SubtitleTextTrackCandidate>,
    ): List<SubtitleTextTrackCandidate> {
        if (textTracks.isEmpty()) return emptyList()
        val sideLoaded = textTracks.filter { it.sideLoadPriority > 0 }
        val withoutCeaOrDeduped =
            textTracks.filter { candidate ->
                if (candidate.isCeaClosedCaption) return@filter false
                if (candidate.sideLoadPriority > 0) return@filter true
                sideLoaded.none { other -> shouldDropDemuxedDuplicate(candidate, other) }
            }
        if (withoutCeaOrDeduped.isNotEmpty()) return withoutCeaOrDeduped
        // Exo sometimes only reports CEA-608 before HLS WebVTT renditions arrive; hiding every row
        // left the TV app with a single "Off" option and [openSubtitlePicker] refused to open.
        val ceaOnly = textTracks.filter { it.isCeaClosedCaption }
        if (ceaOnly.isNotEmpty()) return ceaOnly
        // Dedupe removed everything (unexpected); show raw list so the user can still pick a track.
        return textTracks
    }

    private fun shouldDropDemuxedDuplicate(
        candidate: SubtitleTextTrackCandidate,
        other: SubtitleTextTrackCandidate,
    ): Boolean {
        val candidateLogical = candidate.logicalId
        val otherLogical = other.logicalId
        if (candidateLogical != null && otherLogical != null && candidateLogical == otherLogical) {
            return true
        }
        return false
    }

    private fun SubtitleTextTrackCandidate.detailWithSourceTag(
        allVisibleTextTracks: List<SubtitleTextTrackCandidate>,
    ): String? {
        val parts = mutableListOf<String>()
        detail?.trim()?.takeIf { it.isNotEmpty() }?.let { parts += it }
        val sourceTag =
            when {
                sideLoadPriority > 0 -> "sideload"
                allVisibleTextTracks.any {
                    it.sideLoadPriority > 0 &&
                        it.label == label &&
                        it.renderKind == renderKind
                } -> "fallback"
                else -> null
            }
        if (sourceTag != null) {
            parts += sourceTag
        }
        return parts.takeIf { it.isNotEmpty() }?.joinToString(" · ")
    }

    fun buildBurnTrackCandidate(
        subtitle: EmbeddedSubtitleJson,
        activeBurnSubtitleStreamIndex: Int?,
        label: String,
    ): SubtitleBurnTrackCandidate =
        SubtitleBurnTrackCandidate(
            pickerId = SubtitlePickerTrackId.BurnIn(subtitle.streamIndex).toWireId(),
            logicalId = logicalIdForEmbedded(subtitle),
            streamIndex = subtitle.streamIndex,
            label = label,
            detail = subtitle.codec?.uppercase(Locale.US),
            selected = activeBurnSubtitleStreamIndex == subtitle.streamIndex,
        )
}
