package plum.tv.core.player

import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.sync.Mutex
import plum.tv.core.network.PlaybackSessionJson

/**
 * Explicit lifecycle for burn-in subtitle changes that require tearing down the HLS session and
 * creating a new playback session. Serialized with a mutex so overlapping user taps are ignored
 * while a reload is in flight ([tryLockConcurrentBurnReload]).
 */
enum class SubtitleBurnReloadPhase {
    Idle,
    Detaching,
    CreatingSession,
    ApplyingPlayback,
    Failed,
}

/**
 * Host supplies I/O and player wiring; the coordinator owns ordering, phase transitions, and the
 * mutex policy around [reloadForBurnSubtitle].
 */
interface SubtitleBurnReloadHost {
    suspend fun detachCurrentHlsSession()

    fun cancelRevisionReadyPoll()

    fun notifyPreparingStream()

    suspend fun createBurnReloadPlaybackSession(): Result<PlaybackSessionJson>

    suspend fun applyPlaybackSessionAfterBurnReload(session: PlaybackSessionJson, resumeSec: Float)

    fun onBurnReloadSessionCreateFailed(error: Throwable)
}

class SubtitleBurnSessionCoordinator(
    private val host: SubtitleBurnReloadHost,
    private val mutex: Mutex = Mutex(),
) {
    @Volatile
    var phase: SubtitleBurnReloadPhase = SubtitleBurnReloadPhase.Idle
        private set

    fun tryLockConcurrentBurnReload(): Boolean = mutex.tryLock()

    fun unlockConcurrentBurnReload(lockHeld: Boolean) {
        if (lockHeld) {
            mutex.unlock()
        }
    }

    suspend fun reloadForBurnSubtitle(resumeSec: Float) {
        try {
            phase = SubtitleBurnReloadPhase.Detaching
            host.detachCurrentHlsSession()
            host.cancelRevisionReadyPoll()
            phase = SubtitleBurnReloadPhase.CreatingSession
            host.notifyPreparingStream()
            host.createBurnReloadPlaybackSession().fold(
                onSuccess = { session ->
                    phase = SubtitleBurnReloadPhase.ApplyingPlayback
                    host.applyPlaybackSessionAfterBurnReload(session, resumeSec)
                    phase = SubtitleBurnReloadPhase.Idle
                },
                onFailure = { e ->
                    phase = SubtitleBurnReloadPhase.Failed
                    host.onBurnReloadSessionCreateFailed(e)
                    phase = SubtitleBurnReloadPhase.Idle
                },
            )
        } catch (e: CancellationException) {
            phase = SubtitleBurnReloadPhase.Idle
            throw e
        } catch (e: Throwable) {
            phase = SubtitleBurnReloadPhase.Failed
            host.onBurnReloadSessionCreateFailed(e)
            phase = SubtitleBurnReloadPhase.Idle
        }
    }
}
