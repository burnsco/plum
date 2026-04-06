import { type CSSProperties, type SyntheticEvent } from "react";
import { JassubRenderer } from "@/components/JassubRenderer";

type PlaybackVideoStageProps = {
  mediaItemId: number;
  setVideoRef: (element: HTMLVideoElement | null) => void;
  videoSubtitleStyle: CSSProperties;
  jassubVideoElement: HTMLVideoElement | null;
  activeAssSource: string | null;
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
      </div>
      <JassubRenderer videoElement={jassubVideoElement} assSrc={activeAssSource} />
    </div>
  );
}
