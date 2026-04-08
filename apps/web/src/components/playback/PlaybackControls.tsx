import {
  FastForward,
  Pause,
  Play,
  Repeat,
  Rewind,
  SkipBack,
  SkipForward,
  Volume2,
  VolumeX,
} from "lucide-react";
import { formatClock } from "@/lib/playback/playerMedia";
import { VIDEO_SKIP_BUTTON_SECONDS } from "./constants";

export type PlaybackControlsProps = {
  progressMax: number;
  isPlaying: boolean;
  onTogglePlayPause: () => void;
  seekTimeLabelSec: number;
  onSeekRelative: (deltaSeconds: number) => void;
  hasVideoQueueNavigation: boolean;
  onVideoPrevious: () => void;
  onPlayNextInQueue: () => void;
  hasNextQueueItem: boolean;
  onToggleQueueAutoplay: () => void;
  autoplayNextLabel: string;
  queueAutoplayActive: boolean;
  muted: boolean;
  volume: number;
  onToggleMute: () => void;
  onVolumeSliderChange: (value: number) => void;
  muteButtonLabel: string;
};

/** Play/pause, skip, clock, queue navigation, and volume (bottom bar, left cluster). */
export function PlaybackControls({
  progressMax,
  isPlaying,
  onTogglePlayPause,
  seekTimeLabelSec,
  onSeekRelative,
  hasVideoQueueNavigation,
  onVideoPrevious,
  onPlayNextInQueue,
  hasNextQueueItem,
  onToggleQueueAutoplay,
  autoplayNextLabel,
  queueAutoplayActive,
  muted,
  volume,
  onToggleMute,
  onVolumeSliderChange,
  muteButtonLabel,
}: PlaybackControlsProps) {
  return (
    <div className="fullscreen-player__controls-left">
      <button
        type="button"
        className="fullscreen-player__ctrl-btn"
        onClick={onTogglePlayPause}
        aria-label={isPlaying ? "Pause playback" : "Play playback"}
        title={isPlaying ? "Pause" : "Play"}
      >
        {isPlaying ? (
          <Pause className="size-[1.125rem]" strokeWidth={2.25} />
        ) : (
          <Play className="size-[1.125rem]" strokeWidth={2.25} />
        )}
      </button>
      <button
        type="button"
        className="fullscreen-player__ctrl-btn"
        onClick={() => onSeekRelative(-VIDEO_SKIP_BUTTON_SECONDS)}
        aria-label={`Seek back ${VIDEO_SKIP_BUTTON_SECONDS} seconds`}
        title={`Back ${VIDEO_SKIP_BUTTON_SECONDS}s`}
      >
        <Rewind className="size-[1.125rem]" strokeWidth={2.25} />
      </button>
      <button
        type="button"
        className="fullscreen-player__ctrl-btn"
        onClick={() => onSeekRelative(VIDEO_SKIP_BUTTON_SECONDS)}
        aria-label={`Seek forward ${VIDEO_SKIP_BUTTON_SECONDS} seconds`}
        title={`Forward ${VIDEO_SKIP_BUTTON_SECONDS}s`}
      >
        <FastForward className="size-[1.125rem]" strokeWidth={2.25} />
      </button>
      <span className="fullscreen-player__time">
        {formatClock(seekTimeLabelSec)} / {formatClock(progressMax)}
      </span>

      {hasVideoQueueNavigation && (
        <>
          <button
            type="button"
            className="fullscreen-player__ctrl-btn"
            onClick={onVideoPrevious}
            aria-label="Previous episode"
            title="Previous episode"
          >
            <SkipBack className="size-[1.125rem]" strokeWidth={2.25} />
          </button>

          <button
            type="button"
            className="fullscreen-player__ctrl-btn"
            onClick={onPlayNextInQueue}
            aria-label="Next episode"
            title="Next episode"
            disabled={!hasNextQueueItem}
          >
            <SkipForward className="size-[1.125rem]" strokeWidth={2.25} />
          </button>

          <button
            type="button"
            className={`fullscreen-player__ctrl-btn${queueAutoplayActive ? " is-active" : ""}`}
            onClick={onToggleQueueAutoplay}
            aria-label="Autoplay next episode"
            title={autoplayNextLabel}
            aria-pressed={queueAutoplayActive}
          >
            <Repeat className="size-[1.125rem]" strokeWidth={2.25} />
          </button>
        </>
      )}

      <div className="fullscreen-player__volume-group">
        <button
          type="button"
          className="fullscreen-player__ctrl-btn"
          onClick={onToggleMute}
          aria-label={muteButtonLabel}
          title={muteButtonLabel}
        >
          {muted || volume === 0 ? (
            <VolumeX className="size-[1.125rem]" strokeWidth={2.25} />
          ) : (
            <Volume2 className="size-[1.125rem]" strokeWidth={2.25} />
          )}
        </button>
        <input
          type="range"
          className="fullscreen-player__volume-slider"
          aria-label="Set volume"
          min={0}
          max={1}
          step={0.01}
          value={muted ? 0 : volume}
          onChange={(event) => onVolumeSliderChange(Number(event.target.value))}
        />
      </div>
    </div>
  );
}
