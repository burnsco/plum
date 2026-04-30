import { type CSSProperties, type SyntheticEvent } from "react";
import { JassubRenderer } from "@/components/JassubRenderer";
import type { SubtitleAppearance } from "../../lib/playbackPreferences";

type PlaybackVideoStageProps = {
  mediaItemId: number;
  setVideoRef: (element: HTMLVideoElement | null) => void;
  videoSubtitleStyle: CSSProperties;
  jassubVideoElement: HTMLVideoElement | null;
  activeAssSource: string | null;
  activeAssFontUrls: readonly string[];
  jassubReloadKey: number;
  managedSubtitleCueTexts: readonly string[];
  managedSubtitlePosition: SubtitleAppearance["position"];
  videoStreamOffsetSeconds: number;
  onAssStatusChange: (status: "loading" | "ready" | "error" | "timeout") => void;
  onVideoDoubleClick: () => void;
  onLoadStart: () => void;
  onLoadedMetadata: (element: HTMLVideoElement) => void;
  onCanPlay: (element: HTMLVideoElement) => void;
  onTimeUpdate: (event: SyntheticEvent<HTMLVideoElement>) => void;
  onPlay: (event: SyntheticEvent<HTMLVideoElement>) => void;
  onPlaying: () => void;
  onPause: (event: SyntheticEvent<HTMLVideoElement>) => void;
  onWaiting: (event: SyntheticEvent<HTMLVideoElement>) => void;
  onSeeked: (event: SyntheticEvent<HTMLVideoElement>) => void;
  onVolumeChange: (event: SyntheticEvent<HTMLVideoElement>) => void;
  onError: () => void;
  onEnded: (event: SyntheticEvent<HTMLVideoElement>) => void;
};

/** Core `<video>` surface plus ASS subtitle renderer; HLS/direct source wiring lives in `PlaybackDock`. */
export function PlaybackVideoStage({
  mediaItemId,
  setVideoRef,
  videoSubtitleStyle,
  jassubVideoElement,
  activeAssSource,
  activeAssFontUrls,
  jassubReloadKey,
  managedSubtitleCueTexts,
  managedSubtitlePosition,
  videoStreamOffsetSeconds,
  onAssStatusChange,
  onVideoDoubleClick,
  onLoadStart,
  onLoadedMetadata,
  onCanPlay,
  onTimeUpdate,
  onPlay,
  onPlaying,
  onPause,
  onWaiting,
  onSeeked,
  onVolumeChange,
  onError,
  onEnded,
}: PlaybackVideoStageProps) {
  return (
    <div className="fullscreen-player__video-stage">
      <div className="fullscreen-player__video-frame">
        <video
          key={mediaItemId}
          ref={setVideoRef}
          className="fullscreen-player__video"
          style={videoSubtitleStyle}
          crossOrigin="use-credentials"
          autoPlay
          playsInline
          onDoubleClick={onVideoDoubleClick}
          onLoadStart={onLoadStart}
          onLoadedMetadata={(event) => onLoadedMetadata(event.currentTarget)}
          onCanPlay={(event) => onCanPlay(event.currentTarget)}
          onTimeUpdate={onTimeUpdate}
          onPlay={onPlay}
          onPlaying={onPlaying}
          onPause={onPause}
          onWaiting={onWaiting}
          onSeeked={onSeeked}
          onVolumeChange={onVolumeChange}
          onError={onError}
          onEnded={onEnded}
        />
        {managedSubtitleCueTexts.length > 0 && (
          <div
            className={`fullscreen-player__manual-subtitles fullscreen-player__manual-subtitles--${managedSubtitlePosition}`}
            aria-live="off"
          >
            <div className="fullscreen-player__manual-subtitle-cue">
              {managedSubtitleCueTexts.join("\n")}
            </div>
          </div>
        )}
      </div>
      <JassubRenderer
        key={`${activeAssSource ?? "off"}:${jassubReloadKey}`}
        videoElement={jassubVideoElement}
        assSrc={activeAssSource}
        fontUrls={activeAssFontUrls}
        timeOffsetSeconds={videoStreamOffsetSeconds}
        onStatusChange={onAssStatusChange}
      />
    </div>
  );
}
