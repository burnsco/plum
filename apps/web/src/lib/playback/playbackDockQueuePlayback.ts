import type { MediaItem } from "../../api";
import type { MusicRepeatMode } from "../../contexts/playerTypes";

export function hasNextQueueItem(queueLength: number, queueIndex: number): boolean {
  return queueIndex < queueLength - 1;
}

/** Next item after `queueIndex`, or null if none. */
export function nextQueueItemAfterIndex(
  queue: readonly MediaItem[],
  queueIndex: number,
): MediaItem | null {
  if (queueIndex < 0 || queueIndex >= queue.length - 1) return null;
  return queue[queueIndex + 1] ?? null;
}

/** Item to offer for “Up next” after a video ends (autoplay path). */
export function resolveUpNextItemOnVideoEnd(args: {
  videoAutoplayEnabled: boolean;
  queue: readonly MediaItem[];
  queueIndex: number;
}): MediaItem | null {
  if (!args.videoAutoplayEnabled) return null;
  return nextQueueItemAfterIndex(args.queue, args.queueIndex);
}

export function shouldRestartCurrentVideoOnPrevious(
  currentTimeSeconds: number,
  restartThresholdSeconds: number,
): boolean {
  return currentTimeSeconds > restartThresholdSeconds;
}

export function musicPlaybackShouldLoopSameTrack(repeatMode: MusicRepeatMode): boolean {
  return repeatMode === "one";
}
