import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { mediaStreamUrl, tmdbBackdropUrl, tmdbPosterUrl } from '@plum/shared'
import {
  Expand,
  Minimize2,
  Pause,
  Play,
  Repeat,
  Shuffle,
  SkipBack,
  SkipForward,
  Volume2,
  VolumeX,
  X,
} from 'lucide-react'
import type { MediaItem } from '../api'
import { BASE_URL } from '../api'
import { usePlayer } from '../contexts/PlayerContext'

type PlaybackState = {
  currentTime: number
  duration: number
  isPlaying: boolean
}

function formatClock(totalSeconds: number): string {
  if (!Number.isFinite(totalSeconds) || totalSeconds <= 0) return '0:00'
  const wholeSeconds = Math.floor(totalSeconds)
  const hours = Math.floor(wholeSeconds / 3600)
  const minutes = Math.floor((wholeSeconds % 3600) / 60)
  const seconds = wholeSeconds % 60
  if (hours > 0) {
    return `${hours}:${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
  }
  return `${minutes}:${String(seconds).padStart(2, '0')}`
}

function getSeasonEpisodeLabel(item: MediaItem): string | null {
  const season = item.season ?? 0
  const episode = item.episode ?? 0
  if (season <= 0 && episode <= 0) return null
  return `S${String(season).padStart(2, '0')}E${String(episode).padStart(2, '0')}`
}

function getVideoMetadata(item: MediaItem): string {
  const bits = [item.type === 'movie' ? 'Movie' : item.type === 'anime' ? 'Anime' : 'TV']
  const seasonEpisode = getSeasonEpisodeLabel(item)
  const releaseYear =
    item.release_date?.split('-')[0] ||
    (item.type === 'movie' ? item.title.match(/\((\d{4})\)$/)?.[1] : undefined)

  if (seasonEpisode) bits.push(seasonEpisode)
  if (releaseYear) bits.push(releaseYear)
  if (item.duration > 0) bits.push(formatClock(item.duration))
  return bits.join(' • ')
}

function getMusicMetadata(item: MediaItem, queueIndex: number, queueSize: number): string {
  const bits = [item.artist || 'Unknown Artist']
  if (item.album) bits.push(item.album)
  if (queueSize > 0) bits.push(`${queueIndex + 1}/${queueSize}`)
  return bits.join(' • ')
}

export function PlaybackDock() {
  const videoRef = useRef<HTMLVideoElement | null>(null)
  const audioRef = useRef<HTMLAudioElement | null>(null)
  const [playbackState, setPlaybackState] = useState<PlaybackState>({
    currentTime: 0,
    duration: 0,
    isPlaying: false,
  })
  const [selectedSubtitleKey, setSelectedSubtitleKey] = useState('off')
  const {
    activeItem,
    activeMode,
    isDockOpen,
    viewMode,
    queue,
    queueIndex,
    shuffle,
    repeatMode,
    volume,
    muted,
    wsConnected,
    lastEvent,
    registerMediaElement,
    togglePlayPause,
    seekTo,
    setMuted,
    setVolume,
    enterFullscreen,
    exitFullscreen,
    dismissDock,
    playNextInQueue,
    playPreviousInQueue,
    toggleShuffle,
    cycleRepeatMode,
  } = usePlayer()

  const isVideo = activeMode === 'video' && activeItem != null
  const isMusic = activeMode === 'music' && activeItem != null
  const isFullscreen = isVideo && viewMode === 'fullscreen'

  const subtitleTracks = useMemo(() => {
    if (!isVideo || !activeItem) return []
    const external =
      activeItem.subtitles?.map((subtitle) => ({
        key: `ext-${subtitle.id}`,
        label: subtitle.title || subtitle.language,
        src: `${BASE_URL || ''}/api/subtitles/${subtitle.id}`,
      })) ?? []
    const embedded =
      activeItem.embeddedSubtitles?.map((subtitle) => ({
        key: `emb-${subtitle.streamIndex}`,
        label: subtitle.title || subtitle.language,
        src: `${BASE_URL || ''}/api/media/${activeItem.id}/subtitles/embedded/${subtitle.streamIndex}`,
      })) ?? []
    return [...external, ...embedded]
  }, [activeItem, isVideo])

  const syncPlaybackState = useCallback((element: HTMLMediaElement | null) => {
    if (!element) {
      setPlaybackState({ currentTime: 0, duration: 0, isPlaying: false })
      return
    }
    setPlaybackState({
      currentTime: Number.isFinite(element.currentTime) ? element.currentTime : 0,
      duration:
        Number.isFinite(element.duration) && element.duration > 0
          ? element.duration
          : activeItem?.duration ?? 0,
      isPlaying: !element.paused && !element.ended,
    })
  }, [activeItem?.duration])

  const setVideoRef = useCallback(
    (element: HTMLVideoElement | null) => {
      videoRef.current = element
      registerMediaElement('video', element)
      syncPlaybackState(element)
    },
    [registerMediaElement, syncPlaybackState],
  )

  const setAudioRef = useCallback(
    (element: HTMLAudioElement | null) => {
      audioRef.current = element
      registerMediaElement('audio', element)
      syncPlaybackState(element)
    },
    [registerMediaElement, syncPlaybackState],
  )

  useEffect(() => {
    setPlaybackState({
      currentTime: 0,
      duration: activeItem?.duration ?? 0,
      isPlaying: false,
    })
    setSelectedSubtitleKey('off')
  }, [activeItem?.id, activeItem?.duration])

  useEffect(() => {
    const video = videoRef.current
    if (!video) return
    if (selectedSubtitleKey === 'off') {
      for (let index = 0; index < video.textTracks.length; index += 1) {
        video.textTracks[index]!.mode = 'disabled'
      }
      return
    }
    const selectedTrack = subtitleTracks.find((track) => track.key === selectedSubtitleKey)
    for (let index = 0; index < video.textTracks.length; index += 1) {
      const track = video.textTracks[index]!
      track.mode = selectedTrack && track.label === selectedTrack.label ? 'showing' : 'disabled'
    }
  }, [selectedSubtitleKey, subtitleTracks])

  useEffect(() => {
    if (!isFullscreen || !isVideo) return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        exitFullscreen()
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [exitFullscreen, isFullscreen, isVideo])

  if (!activeItem || !isDockOpen || !activeMode) {
    return null
  }

  const posterUrl = activeItem.poster_path ? tmdbPosterUrl(activeItem.poster_path, 'w342') : ''
  const backdropUrl = activeItem.backdrop_path ? tmdbBackdropUrl(activeItem.backdrop_path, 'w780') : ''
  const progressMax = playbackState.duration > 0 ? playbackState.duration : Math.max(activeItem.duration, 0)
  const repeatLabel =
    repeatMode === 'one' ? 'Repeat track' : repeatMode === 'all' ? 'Repeat queue' : 'Repeat off'

  return (
    <section
      className={`playback-dock playback-dock--${activeMode} playback-dock--${viewMode}`}
      aria-label={isMusic ? 'Music player' : 'Playback dock'}
    >
      {isVideo && backdropUrl && (
        <div className="playback-dock__backdrop" aria-hidden="true">
          <img src={backdropUrl} alt="" />
        </div>
      )}

      <div className="playback-dock__shell">
        <div className="playback-dock__topbar">
          <div className="playback-dock__status">
            {isVideo && (
              <>
                <span className="status-dot" data-connected={wsConnected} />
                <span className="playback-dock__status-copy">
                  {lastEvent || (wsConnected ? 'Waiting for transcode updates' : 'WebSocket disconnected')}
                </span>
              </>
            )}
          </div>
          <div className="playback-dock__actions">
            {isVideo && (
              <button
                type="button"
                className="playback-dock__icon-button"
                onClick={isFullscreen ? exitFullscreen : enterFullscreen}
                aria-label={isFullscreen ? 'Return to docked player' : 'Open fullscreen player'}
                title={isFullscreen ? 'Return to docked player' : 'Open fullscreen player'}
              >
                {isFullscreen ? <Minimize2 className="size-4" /> : <Expand className="size-4" />}
              </button>
            )}
            <button
              type="button"
              className="playback-dock__icon-button"
              onClick={dismissDock}
              aria-label="Close player"
              title="Close player"
            >
              <X className="size-4" />
            </button>
          </div>
        </div>

        <div className="playback-dock__content">
          <div className="playback-dock__summary">
            <div className="playback-dock__artwork">
              {posterUrl ? <img src={posterUrl} alt="" /> : <img src="/placeholder-poster.png" alt="" />}
            </div>
            <div className="playback-dock__copy">
              <div className="playback-dock__eyebrow">
                {isVideo
                  ? getVideoMetadata(activeItem)
                  : getMusicMetadata(activeItem, queueIndex, queue.length)}
              </div>
              <h2 className="playback-dock__title">{activeItem.title}</h2>
              {isMusic && (
                <div className="playback-dock__subcopy">
                  {activeItem.album_artist && activeItem.album_artist !== activeItem.artist
                    ? `Album artist: ${activeItem.album_artist}`
                    : activeItem.release_year
                      ? `Released ${activeItem.release_year}`
                      : 'Docked playback'}
                </div>
              )}
              {isVideo && activeItem.overview && (
                <p className="playback-dock__overview">{activeItem.overview}</p>
              )}
              {isVideo && subtitleTracks.length > 0 && (
                <label className="playback-dock__subtitle-picker">
                  <span>Subtitles</span>
                  <select
                    value={selectedSubtitleKey}
                    onChange={(event) => setSelectedSubtitleKey(event.target.value)}
                  >
                    <option value="off">Off</option>
                    {subtitleTracks.map((track) => (
                      <option key={track.key} value={track.key}>
                        {track.label}
                      </option>
                    ))}
                  </select>
                </label>
              )}
            </div>
          </div>

          {isVideo && (
            <button
              type="button"
              className="playback-dock__surface"
              onClick={() => {
                if (!isFullscreen) enterFullscreen()
              }}
              aria-label={isFullscreen ? activeItem.title : `Open fullscreen player for ${activeItem.title}`}
            >
              <video
                key={activeItem.id}
                ref={setVideoRef}
                className="playback-dock__video"
                src={mediaStreamUrl(BASE_URL, activeItem.id)}
                autoPlay
                playsInline
                onLoadedMetadata={(event) => syncPlaybackState(event.currentTarget)}
                onTimeUpdate={(event) => syncPlaybackState(event.currentTarget)}
                onPlay={(event) => syncPlaybackState(event.currentTarget)}
                onPause={(event) => syncPlaybackState(event.currentTarget)}
                onVolumeChange={(event) => syncPlaybackState(event.currentTarget)}
              >
                {subtitleTracks.map((track) => (
                  <track key={track.key} kind="subtitles" src={track.src} label={track.label} />
                ))}
              </video>
              {!isFullscreen && (
                <span className="playback-dock__surface-hint">Click video to expand</span>
              )}
            </button>
          )}

          {isMusic && (
            <audio
              key={activeItem.id}
              ref={setAudioRef}
              className="playback-dock__audio"
              src={mediaStreamUrl(BASE_URL, activeItem.id)}
              autoPlay
              onLoadedMetadata={(event) => syncPlaybackState(event.currentTarget)}
              onTimeUpdate={(event) => syncPlaybackState(event.currentTarget)}
              onPlay={(event) => syncPlaybackState(event.currentTarget)}
              onPause={(event) => syncPlaybackState(event.currentTarget)}
              onVolumeChange={(event) => syncPlaybackState(event.currentTarget)}
              onEnded={() => {
                if (repeatMode === 'one' && audioRef.current) {
                  audioRef.current.currentTime = 0
                  void audioRef.current.play().catch(() => {})
                  return
                }
                playNextInQueue()
              }}
            />
          )}
        </div>

        <div className="playback-dock__transport">
          <div className="playback-dock__buttons">
            {isMusic && (
              <>
                <button
                  type="button"
                  className={`playback-dock__icon-button${shuffle ? ' is-active' : ''}`}
                  onClick={toggleShuffle}
                  aria-label={shuffle ? 'Disable shuffle' : 'Enable shuffle'}
                >
                  <Shuffle className="size-4" />
                </button>
                <button
                  type="button"
                  className="playback-dock__icon-button"
                  onClick={playPreviousInQueue}
                  aria-label="Previous track"
                >
                  <SkipBack className="size-4" />
                </button>
              </>
            )}

            <button
              type="button"
              className="playback-dock__play-button"
              onClick={togglePlayPause}
              aria-label={playbackState.isPlaying ? 'Pause playback' : 'Play playback'}
            >
              {playbackState.isPlaying ? <Pause className="size-5" /> : <Play className="size-5" />}
            </button>

            {isMusic && (
              <>
                <button
                  type="button"
                  className="playback-dock__icon-button"
                  onClick={playNextInQueue}
                  aria-label="Next track"
                >
                  <SkipForward className="size-4" />
                </button>
                <button
                  type="button"
                  className={`playback-dock__icon-button${repeatMode !== 'off' ? ' is-active' : ''}`}
                  onClick={cycleRepeatMode}
                  aria-label={repeatLabel}
                  title={repeatLabel}
                >
                  <Repeat className="size-4" />
                  <span className="playback-dock__repeat-copy">
                    {repeatMode === 'one' ? '1' : repeatMode === 'all' ? 'all' : 'off'}
                  </span>
                </button>
              </>
            )}
          </div>

          <div className="playback-dock__timeline">
            <span className="playback-dock__time">{formatClock(playbackState.currentTime)}</span>
            <input
              type="range"
              className="playback-dock__slider"
              aria-label="Seek playback"
              min={0}
              max={progressMax || 0}
              step={1}
              value={Math.min(playbackState.currentTime, progressMax || 0)}
              onChange={(event) => seekTo(Number(event.target.value))}
            />
            <span className="playback-dock__time">{formatClock(progressMax)}</span>
          </div>

          <div className="playback-dock__volume">
            <button
              type="button"
              className="playback-dock__icon-button"
              onClick={() => setMuted(!muted)}
              aria-label={muted || volume === 0 ? 'Unmute' : 'Mute'}
            >
              {muted || volume === 0 ? <VolumeX className="size-4" /> : <Volume2 className="size-4" />}
            </button>
            <input
              type="range"
              className="playback-dock__slider playback-dock__slider--volume"
              aria-label="Set volume"
              min={0}
              max={1}
              step={0.01}
              value={muted ? 0 : volume}
              onChange={(event) => setVolume(Number(event.target.value))}
            />
          </div>
        </div>
      </div>
    </section>
  )
}
