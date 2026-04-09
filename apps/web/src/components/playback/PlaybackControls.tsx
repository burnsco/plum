import {
  FastForward,
  Pause,
  Play,
  Repeat,
  Rewind,
  SkipBack,
  SkipForward,
} from "lucide-react";
import { VIDEO_SKIP_BUTTON_SECONDS } from "./constants";

export type PlaybackControlsProps = {
  isPlaying: boolean;
  onTogglePlayPause: () => void;
  onSeekRelative: (deltaSeconds: number) => void;
  hasVideoQueueNavigation: boolean;
  onVideoPrevious: () => void;
  onPlayNextInQueue: () => void;
  hasNextQueueItem: boolean;
  onToggleQueueAutoplay: () => void;
  autoplayNextLabel: string;
  queueAutoplayActive: boolean;
};

/** Center transport controls: skip prev, rewind, play/pause, fast forward, skip next, autoplay. */
export function PlaybackControls({
  isPlaying,
  onTogglePlayPause,
  onSeekRelative,
  hasVideoQueueNavigation,
  onVideoPrevious,
  onPlayNextInQueue,
  hasNextQueueItem,
  onToggleQueueAutoplay,
  autoplayNextLabel,
  queueAutoplayActive,
}: PlaybackControlsProps) {
  return (
    <div className="fullscreen-player__controls-center">
      {hasVideoQueueNavigation && (
        <button
          type="button"
          className="fullscreen-player__ctrl-btn"
          onClick={onVideoPrevious}
          aria-label="Previous episode"
          title="Previous episode"
        >
          <SkipBack className="size-[1.125rem]" strokeWidth={2.25} />
        </button>
      )}

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
        onClick={() => onSeekRelative(VIDEO_SKIP_BUTTON_SECONDS)}
        aria-label={`Seek forward ${VIDEO_SKIP_BUTTON_SECONDS} seconds`}
        title={`Forward ${VIDEO_SKIP_BUTTON_SECONDS}s`}
      >
        <FastForward className="size-[1.125rem]" strokeWidth={2.25} />
      </button>

      {hasVideoQueueNavigation && (
        <>
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
    </div>
  );
}
