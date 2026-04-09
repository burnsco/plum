import { type ChangeEvent, type PointerEvent, type RefObject } from "react";
import { formatClock } from "@/lib/playback/playerMedia";

export type PlaybackTimelineProps = {
  progressMax: number;
  seekTimeLabelSec: number;
  seekSliderRef: RefObject<HTMLInputElement | null>;
  seekSliderDisplayValue: number;
  onSeekPointerDown: (event: PointerEvent<HTMLInputElement>) => void;
  onSeekChange: (event: ChangeEvent<HTMLInputElement> | { currentTarget: HTMLInputElement }) => void;
};

/** Seek range with flanking time labels for fullscreen video (scrub commits on pointer release). */
export function PlaybackTimeline({
  progressMax,
  seekTimeLabelSec,
  seekSliderRef,
  seekSliderDisplayValue,
  onSeekPointerDown,
  onSeekChange,
}: PlaybackTimelineProps) {
  const remaining = progressMax > 0 ? progressMax - seekTimeLabelSec : 0;
  return (
    <div className="fullscreen-player__seek">
      <span className="fullscreen-player__seek-time">{formatClock(seekTimeLabelSec)}</span>
      <input
        ref={seekSliderRef}
        type="range"
        className="fullscreen-player__seek-slider"
        aria-label="Seek playback"
        min={0}
        max={progressMax || 0}
        step={0.1}
        value={seekSliderDisplayValue}
        onPointerDown={onSeekPointerDown}
        onChange={onSeekChange}
        onInput={onSeekChange}
      />
      <span className="fullscreen-player__seek-time fullscreen-player__seek-time--end">
        -{formatClock(remaining)}
      </span>
    </div>
  );
}
