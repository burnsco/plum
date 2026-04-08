import { type ChangeEvent, type PointerEvent, type RefObject } from "react";

export type PlaybackTimelineProps = {
  progressMax: number;
  seekSliderRef: RefObject<HTMLInputElement | null>;
  seekSliderDisplayValue: number;
  onSeekPointerDown: (event: PointerEvent<HTMLInputElement>) => void;
  onSeekChange: (event: ChangeEvent<HTMLInputElement> | { currentTarget: HTMLInputElement }) => void;
};

/** Seek range for fullscreen video (scrub commits on pointer release). */
export function PlaybackTimeline({
  progressMax,
  seekSliderRef,
  seekSliderDisplayValue,
  onSeekPointerDown,
  onSeekChange,
}: PlaybackTimelineProps) {
  return (
    <div className="fullscreen-player__seek">
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
    </div>
  );
}
