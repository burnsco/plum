import { useCallback, useEffect, useState } from "react";
import { Pause, Play, Repeat, Shuffle, SkipBack, SkipForward, Volume2, VolumeX, X } from "lucide-react";
import { resolvePosterUrl } from "@plum/shared";
import { BASE_URL } from "../api";
import { usePlayer } from "../contexts/PlayerContext";
import { formatClock, getMusicMetadata } from "../lib/playback/playerMedia";

type Props = {
  visible: boolean;
};

export function MusicNowPlayingBar({ visible }: Props) {
  const {
    activeItem,
    activeMode,
    isDockOpen,
    queue,
    queueIndex,
    shuffle,
    repeatMode,
    volume,
    muted,
    togglePlayPause,
    seekTo,
    setMuted,
    setVolume,
    dismissDock,
    playNextInQueue,
    playPreviousInQueue,
    toggleShuffle,
    cycleRepeatMode,
  } = usePlayer();

  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [isPlaying, setIsPlaying] = useState(false);

  const syncFromAudio = useCallback(() => {
    const el = document.querySelector("audio.plum-music-audio") as HTMLAudioElement | null;
    if (!el) return;
    setCurrentTime(el.currentTime);
    setDuration(Number.isFinite(el.duration) ? el.duration : 0);
    setIsPlaying(!el.paused);
  }, []);

  useEffect(() => {
    if (!visible || activeMode !== "music" || !isDockOpen || !activeItem) return;
    const el = document.querySelector("audio.plum-music-audio") as HTMLAudioElement | null;
    if (!el) return;
    syncFromAudio();
    const onUpdate = () => syncFromAudio();
    el.addEventListener("timeupdate", onUpdate);
    el.addEventListener("loadedmetadata", onUpdate);
    el.addEventListener("play", onUpdate);
    el.addEventListener("pause", onUpdate);
    el.addEventListener("ended", onUpdate);
    return () => {
      el.removeEventListener("timeupdate", onUpdate);
      el.removeEventListener("loadedmetadata", onUpdate);
      el.removeEventListener("play", onUpdate);
      el.removeEventListener("pause", onUpdate);
      el.removeEventListener("ended", onUpdate);
    };
  }, [activeItem, activeMode, isDockOpen, syncFromAudio, visible]);

  if (!visible || activeMode !== "music" || !isDockOpen || !activeItem) {
    return null;
  }

  const posterUrl = resolvePosterUrl(activeItem.poster_url, activeItem.poster_path, "w200", BASE_URL);
  const repeatLabel =
    repeatMode === "one" ? "Repeat track" : repeatMode === "all" ? "Repeat queue" : "Repeat off";
  const muteButtonLabel = muted || volume === 0 ? "Unmute" : "Mute";
  const progressMax = duration > 0 ? duration : activeItem.duration ?? 0;
  const seekSliderValue = progressMax > 0 ? Math.min(currentTime, progressMax) : 0;

  return (
    <footer
      className="music-now-playing-bar"
      role="region"
      aria-label="Music player"
    >
      <div className="music-now-playing-bar__inner">
        <div className="music-now-playing-bar__track">
          <div className="music-now-playing-bar__artwork">
            {posterUrl ? (
              <img src={posterUrl} alt="" />
            ) : (
              <img src="/placeholder-poster.svg" alt="" />
            )}
          </div>
          <div className="music-now-playing-bar__copy">
            <p className="music-now-playing-bar__eyebrow">
              {getMusicMetadata(activeItem, queueIndex, queue.length)}
            </p>
            <p className="music-now-playing-bar__title">{activeItem.title}</p>
          </div>
        </div>

        <div className="music-now-playing-bar__controls">
          <div className="playback-dock__buttons music-now-playing-bar__buttons">
            <button
              type="button"
              className={`playback-dock__icon-button${shuffle ? " is-active" : ""}`}
              onClick={toggleShuffle}
              aria-label={shuffle ? "Disable shuffle" : "Enable shuffle"}
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
            <button
              type="button"
              className="playback-dock__play-button"
              onClick={togglePlayPause}
              aria-label={isPlaying ? "Pause playback" : "Play playback"}
              title={isPlaying ? "Pause" : "Play"}
            >
              {isPlaying ? (
                <Pause className="size-5" strokeWidth={2.25} />
              ) : (
                <Play className="size-5" strokeWidth={2.25} />
              )}
            </button>
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
              className={`playback-dock__icon-button${repeatMode !== "off" ? " is-active" : ""}`}
              onClick={cycleRepeatMode}
              aria-label={repeatLabel}
              title={repeatLabel}
            >
              <Repeat className="size-4" />
              <span className="playback-dock__repeat-copy">
                {repeatMode === "one" ? "1" : repeatMode === "all" ? "all" : "off"}
              </span>
            </button>
          </div>

          <div className="playback-dock__timeline music-now-playing-bar__timeline">
            <span className="playback-dock__time">{formatClock(currentTime)}</span>
            <input
              type="range"
              className="playback-dock__slider playback-dock__slider--seek"
              aria-label="Seek playback"
              min={0}
              max={progressMax || 0}
              step={0.1}
              value={seekSliderValue}
              onChange={(event) => seekTo(Number(event.target.value))}
            />
            <span className="playback-dock__time">{formatClock(progressMax)}</span>
          </div>
        </div>

        <div className="playback-dock__volume music-now-playing-bar__volume">
          <button
            type="button"
            className="playback-dock__icon-button"
            onClick={() => setMuted(!muted)}
            aria-label={muteButtonLabel}
            title={muteButtonLabel}
          >
            {muted || volume === 0 ? (
              <VolumeX className="size-4" strokeWidth={2.25} />
            ) : (
              <Volume2 className="size-4" strokeWidth={2.25} />
            )}
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
    </footer>
  );
}
