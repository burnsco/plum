import {
  type CSSProperties,
  type ChangeEvent,
  type KeyboardEvent,
  type MouseEvent,
  type PointerEvent,
  type RefCallback,
  type RefObject,
  type SyntheticEvent,
} from "react";
import { Ratio, Volume2, VolumeX } from "lucide-react";
import type { MediaItem } from "../../api";
import type { AudioTrackOption, TrackMenuOption } from "../../lib/playback/playerMedia";
import { formatClock, getSeasonEpisodeLabel } from "../../lib/playback/playerMedia";
import type { SubtitleAppearance, VideoAspectMode } from "../../lib/playbackPreferences";
import { PlaybackControls } from "./PlaybackControls";
import { PlaybackDockShell } from "./PlaybackDockShell";
import { PlaybackInfoPanel } from "./PlaybackInfoPanel";
import { PlaybackTimeline } from "./PlaybackTimeline";
import { PlaybackTrackMenus } from "./PlaybackTrackMenus";
import { PlaybackVideoStage } from "./PlaybackVideoStage";
import { PlayerLoadingOverlay } from "./PlayerLoadingOverlay";
import { TrackMenu } from "./TrackMenu";

type ResumePrompt = {
  seconds: number;
  mediaId: number;
};

type PlaybackVideoWindowProps = {
  activeItem: MediaItem;
  playerRootRef: RefCallback<HTMLElement | null>;
  controlsVisible: boolean;
  videoAspectMode: VideoAspectMode;
  showPlayerLoadingOverlay: boolean;
  onMouseMove: () => void;
  onTogglePlayPause: () => void;
  onResetHideTimer: () => void;
  setVideoRef: (element: HTMLVideoElement | null) => void;
  videoSubtitleStyle: CSSProperties;
  jassubVideoElement: HTMLVideoElement | null;
  activeAssSource: string | null;
  activeAssFontUrls: readonly string[];
  managedSubtitleCueTexts: readonly string[];
  managedSubtitlePosition: SubtitleAppearance["position"];
  videoStreamOffsetSeconds: number;
  onAssStatusChange: (status: "loading" | "ready" | "error" | "timeout") => void;
  onVideoDoubleClick: () => void;
  onVideoLoadStart: () => void;
  onVideoLoadedMetadata: (element: HTMLVideoElement) => void;
  onVideoCanPlay: (element: HTMLVideoElement) => void;
  onVideoTimeUpdate: (event: SyntheticEvent<HTMLVideoElement>) => void;
  onVideoPlay: (event: SyntheticEvent<HTMLVideoElement>) => void;
  onVideoPlaying: () => void;
  onVideoPause: (event: SyntheticEvent<HTMLVideoElement>) => void;
  onVideoWaiting: (event: SyntheticEvent<HTMLVideoElement>) => void;
  onVideoSeeked: (event: SyntheticEvent<HTMLVideoElement>) => void;
  onVideoVolumeChange: (event: SyntheticEvent<HTMLVideoElement>) => void;
  onVideoError: () => void;
  onVideoEnded: (event: SyntheticEvent<HTMLVideoElement>) => void;
  playerLoadingLabel: string;
  titleDisplay: string;
  videoStatusMessage: string;
  wsConnected: boolean;
  browserFullscreenActive: boolean;
  onToggleBrowserFullscreen: () => void | Promise<void>;
  onClosePlayer: () => void;
  controlsRef: RefObject<HTMLDivElement | null>;
  onControlsMouseEnter: () => void;
  progressMax: number;
  seekTimeLabelSec: number;
  seekSliderRef: RefObject<HTMLInputElement | null>;
  seekSliderDisplayValue: number;
  onSeekSliderPointerDown: (event: PointerEvent<HTMLInputElement>) => void;
  onSeekSliderChange: (
    event: ChangeEvent<HTMLInputElement> | { currentTarget: HTMLInputElement },
  ) => void;
  aspectBtnRef: RefObject<HTMLButtonElement | null>;
  aspectMenuRef: RefObject<HTMLDivElement | null>;
  aspectMenuOpen: boolean;
  aspectTrackMenuOptions: TrackMenuOption[];
  onToggleAspectMenu: () => void;
  onSelectAspectMode: (mode: VideoAspectMode) => void;
  muted: boolean;
  volume: number;
  onToggleMuted: () => void;
  onVolumeChange: (volume: number) => void;
  isPlaying: boolean;
  onSeekRelative: (deltaSeconds: number) => void;
  hasVideoQueueNavigation: boolean;
  onVideoPrevious: () => void;
  onPlayNextInQueue: () => void;
  hasNextQueueItem: boolean;
  onToggleQueueAutoplay: () => void;
  autoplayNextLabel: string;
  queueAutoplayActive: boolean;
  subtitleBtnRef: RefObject<HTMLButtonElement | null>;
  subtitleMenuRef: RefObject<HTMLDivElement | null>;
  subtitleMenuOpen: boolean;
  onSubtitleButtonClick: () => void;
  subtitleMenuTrackOptions: TrackMenuOption[];
  selectedSubtitleKey: string;
  onSelectSubtitleTrack: (key: string) => void;
  audioBtnRef: RefObject<HTMLButtonElement | null>;
  audioMenuRef: RefObject<HTMLDivElement | null>;
  audioMenuOpen: boolean;
  onAudioButtonClick: () => void;
  audioTracks: AudioTrackOption[];
  selectedAudioKey: string;
  selectedAudioLabel: string;
  onSelectAudioTrack: (key: string) => void;
  playerSettingsBtnRef: RefObject<HTMLButtonElement | null>;
  playerSettingsMenuRef: RefObject<HTMLDivElement | null>;
  playerSettingsOpen: boolean;
  onPlayerSettingsButtonClick: () => void;
  subtitleAppearance: SubtitleAppearance;
  onSubtitleAppearanceChange: (value: SubtitleAppearance) => void;
  videoAutoplayEnabled: boolean;
  onVideoAutoplayEnabledChange: (enabled: boolean) => void;
  resumePrompt: ResumePrompt | null;
  onResumeFromProgress: () => void;
  onStartFromBeginning: () => void;
  upNextTarget: MediaItem | null;
  upNextBackdropUrl: string;
  upNextSecondsLeft: number;
  onConfirmUpNextNow: () => void;
  onDismissUpNext: () => void;
};

/** Full-window video player presentation; playback state and side effects stay in the controller. */
export function PlaybackVideoWindow({
  activeItem,
  playerRootRef,
  controlsVisible,
  videoAspectMode,
  showPlayerLoadingOverlay,
  onMouseMove,
  onTogglePlayPause,
  onResetHideTimer,
  setVideoRef,
  videoSubtitleStyle,
  jassubVideoElement,
  activeAssSource,
  activeAssFontUrls,
  managedSubtitleCueTexts,
  managedSubtitlePosition,
  videoStreamOffsetSeconds,
  onAssStatusChange,
  onVideoDoubleClick,
  onVideoLoadStart,
  onVideoLoadedMetadata,
  onVideoCanPlay,
  onVideoTimeUpdate,
  onVideoPlay,
  onVideoPlaying,
  onVideoPause,
  onVideoWaiting,
  onVideoSeeked,
  onVideoVolumeChange,
  onVideoError,
  onVideoEnded,
  playerLoadingLabel,
  titleDisplay,
  videoStatusMessage,
  wsConnected,
  browserFullscreenActive,
  onToggleBrowserFullscreen,
  onClosePlayer,
  controlsRef,
  onControlsMouseEnter,
  progressMax,
  seekTimeLabelSec,
  seekSliderRef,
  seekSliderDisplayValue,
  onSeekSliderPointerDown,
  onSeekSliderChange,
  aspectBtnRef,
  aspectMenuRef,
  aspectMenuOpen,
  aspectTrackMenuOptions,
  onToggleAspectMenu,
  onSelectAspectMode,
  muted,
  volume,
  onToggleMuted,
  onVolumeChange,
  isPlaying,
  onSeekRelative,
  hasVideoQueueNavigation,
  onVideoPrevious,
  onPlayNextInQueue,
  hasNextQueueItem,
  onToggleQueueAutoplay,
  autoplayNextLabel,
  queueAutoplayActive,
  subtitleBtnRef,
  subtitleMenuRef,
  subtitleMenuOpen,
  onSubtitleButtonClick,
  subtitleMenuTrackOptions,
  selectedSubtitleKey,
  onSelectSubtitleTrack,
  audioBtnRef,
  audioMenuRef,
  audioMenuOpen,
  onAudioButtonClick,
  audioTracks,
  selectedAudioKey,
  selectedAudioLabel,
  onSelectAudioTrack,
  playerSettingsBtnRef,
  playerSettingsMenuRef,
  playerSettingsOpen,
  onPlayerSettingsButtonClick,
  subtitleAppearance,
  onSubtitleAppearanceChange,
  videoAutoplayEnabled,
  onVideoAutoplayEnabledChange,
  resumePrompt,
  onResumeFromProgress,
  onStartFromBeginning,
  upNextTarget,
  upNextBackdropUrl,
  upNextSecondsLeft,
  onConfirmUpNextNow,
  onDismissUpNext,
}: PlaybackVideoWindowProps) {
  const muteButtonLabel = muted || volume === 0 ? "Unmute" : "Mute";

  return (
    <PlaybackDockShell
      playerRootRef={playerRootRef}
      controlsVisible={controlsVisible}
      videoAspectMode={videoAspectMode}
      showPlayerLoadingOverlay={showPlayerLoadingOverlay}
      onMouseMove={onMouseMove}
      onClick={(event: MouseEvent<HTMLElement>) => {
        if (
          event.target === event.currentTarget ||
          (event.target as HTMLElement).tagName === "VIDEO"
        ) {
          onTogglePlayPause();
          onResetHideTimer();
        }
      }}
      onKeyDown={(event: KeyboardEvent<HTMLElement>) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          onTogglePlayPause();
          onResetHideTimer();
        }
      }}
    >
      <PlaybackVideoStage
        mediaItemId={activeItem.id}
        setVideoRef={setVideoRef}
        videoSubtitleStyle={videoSubtitleStyle}
        jassubVideoElement={jassubVideoElement}
        activeAssSource={activeAssSource}
        activeAssFontUrls={activeAssFontUrls}
        managedSubtitleCueTexts={managedSubtitleCueTexts}
        managedSubtitlePosition={managedSubtitlePosition}
        videoStreamOffsetSeconds={videoStreamOffsetSeconds}
        onAssStatusChange={onAssStatusChange}
        onVideoDoubleClick={onVideoDoubleClick}
        onLoadStart={onVideoLoadStart}
        onLoadedMetadata={onVideoLoadedMetadata}
        onCanPlay={onVideoCanPlay}
        onTimeUpdate={onVideoTimeUpdate}
        onPlay={onVideoPlay}
        onPlaying={onVideoPlaying}
        onPause={onVideoPause}
        onWaiting={onVideoWaiting}
        onSeeked={onVideoSeeked}
        onVolumeChange={onVideoVolumeChange}
        onError={onVideoError}
        onEnded={onVideoEnded}
      />
      {showPlayerLoadingOverlay && <PlayerLoadingOverlay label={playerLoadingLabel} fullscreen />}

      <PlaybackInfoPanel
        titleDisplay={titleDisplay}
        videoStatusMessage={videoStatusMessage}
        wsConnected={wsConnected}
        browserFullscreenActive={browserFullscreenActive}
        onToggleBrowserFullscreen={onToggleBrowserFullscreen}
        onClosePlayer={onClosePlayer}
      />

      <div
        ref={controlsRef}
        className="fullscreen-player__controls"
        onMouseEnter={onControlsMouseEnter}
      >
        <PlaybackTimeline
          progressMax={progressMax}
          seekTimeLabelSec={seekTimeLabelSec}
          seekSliderRef={seekSliderRef}
          seekSliderDisplayValue={seekSliderDisplayValue}
          onSeekPointerDown={onSeekSliderPointerDown}
          onSeekChange={onSeekSliderChange}
        />
        <div className="fullscreen-player__controls-row">
          <PlaybackLeftControls
            aspectBtnRef={aspectBtnRef}
            aspectMenuRef={aspectMenuRef}
            aspectMenuOpen={aspectMenuOpen}
            aspectTrackMenuOptions={aspectTrackMenuOptions}
            videoAspectMode={videoAspectMode}
            onToggleAspectMenu={onToggleAspectMenu}
            onSelectAspectMode={onSelectAspectMode}
            muted={muted}
            volume={volume}
            muteButtonLabel={muteButtonLabel}
            onToggleMuted={onToggleMuted}
            onVolumeChange={onVolumeChange}
          />

          <PlaybackControls
            isPlaying={isPlaying}
            onTogglePlayPause={onTogglePlayPause}
            onSeekRelative={onSeekRelative}
            hasVideoQueueNavigation={hasVideoQueueNavigation}
            onVideoPrevious={onVideoPrevious}
            onPlayNextInQueue={onPlayNextInQueue}
            hasNextQueueItem={hasNextQueueItem}
            onToggleQueueAutoplay={onToggleQueueAutoplay}
            autoplayNextLabel={autoplayNextLabel}
            queueAutoplayActive={queueAutoplayActive}
          />

          <PlaybackTrackMenus
            showSubtitleControls
            subtitleBtnRef={subtitleBtnRef}
            subtitleMenuRef={subtitleMenuRef}
            subtitleMenuOpen={subtitleMenuOpen}
            onSubtitleButtonClick={onSubtitleButtonClick}
            subtitleMenuTrackOptions={subtitleMenuTrackOptions}
            selectedSubtitleKey={selectedSubtitleKey}
            onSelectSubtitleTrack={onSelectSubtitleTrack}
            audioBtnRef={audioBtnRef}
            audioMenuRef={audioMenuRef}
            audioMenuOpen={audioMenuOpen}
            onAudioButtonClick={onAudioButtonClick}
            audioTracks={audioTracks}
            selectedAudioKey={selectedAudioKey}
            selectedAudioLabel={selectedAudioLabel}
            onSelectAudioTrack={onSelectAudioTrack}
            showSettingsControls
            playerSettingsBtnRef={playerSettingsBtnRef}
            playerSettingsMenuRef={playerSettingsMenuRef}
            playerSettingsOpen={playerSettingsOpen}
            onPlayerSettingsButtonClick={onPlayerSettingsButtonClick}
            subtitleAppearance={subtitleAppearance}
            onSubtitleAppearanceChange={onSubtitleAppearanceChange}
            videoAutoplayEnabled={videoAutoplayEnabled}
            onVideoAutoplayEnabledChange={onVideoAutoplayEnabledChange}
          />
        </div>
      </div>
      <PlaybackResumePromptOverlay
        resumePrompt={resumePrompt}
        onResumeFromProgress={onResumeFromProgress}
        onStartFromBeginning={onStartFromBeginning}
      />
      <PlaybackUpNextOverlay
        upNextTarget={upNextTarget}
        upNextBackdropUrl={upNextBackdropUrl}
        upNextSecondsLeft={upNextSecondsLeft}
        onConfirmUpNextNow={onConfirmUpNextNow}
        onDismissUpNext={onDismissUpNext}
      />
    </PlaybackDockShell>
  );
}

type PlaybackLeftControlsProps = {
  aspectBtnRef: RefObject<HTMLButtonElement | null>;
  aspectMenuRef: RefObject<HTMLDivElement | null>;
  aspectMenuOpen: boolean;
  aspectTrackMenuOptions: TrackMenuOption[];
  videoAspectMode: VideoAspectMode;
  onToggleAspectMenu: () => void;
  onSelectAspectMode: (mode: VideoAspectMode) => void;
  muted: boolean;
  volume: number;
  muteButtonLabel: string;
  onToggleMuted: () => void;
  onVolumeChange: (volume: number) => void;
};

function PlaybackLeftControls({
  aspectBtnRef,
  aspectMenuRef,
  aspectMenuOpen,
  aspectTrackMenuOptions,
  videoAspectMode,
  onToggleAspectMenu,
  onSelectAspectMode,
  muted,
  volume,
  muteButtonLabel,
  onToggleMuted,
  onVolumeChange,
}: PlaybackLeftControlsProps) {
  return (
    <div className="fullscreen-player__controls-left">
      <div className="fullscreen-player__aspect-wrap">
        <button
          ref={aspectBtnRef}
          type="button"
          className={`fullscreen-player__ctrl-btn${videoAspectMode !== "auto" ? " is-active" : ""}`}
          aria-label="Aspect ratio"
          title="Aspect ratio"
          onClick={onToggleAspectMenu}
        >
          <Ratio className="size-[1.125rem]" strokeWidth={2.25} />
        </button>
        {aspectMenuOpen && (
          <TrackMenu
            menuRef={aspectMenuRef}
            options={aspectTrackMenuOptions}
            selectedKey={videoAspectMode}
            ariaLabel="Select aspect ratio"
            onSelect={(key) => onSelectAspectMode(key as VideoAspectMode)}
          />
        )}
      </div>

      <div className="fullscreen-player__volume-group">
        <button
          type="button"
          className="fullscreen-player__ctrl-btn"
          onClick={onToggleMuted}
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
          onChange={(event) => onVolumeChange(Number(event.target.value))}
        />
      </div>
    </div>
  );
}

function PlaybackResumePromptOverlay({
  resumePrompt,
  onResumeFromProgress,
  onStartFromBeginning,
}: {
  resumePrompt: ResumePrompt | null;
  onResumeFromProgress: () => void;
  onStartFromBeginning: () => void;
}) {
  if (resumePrompt == null) return null;

  return (
    <div
      className="playback-resume-prompt"
      role="dialog"
      aria-modal="true"
      aria-label="Resume playback"
    >
      <div className="playback-resume-prompt__scrim" />
      <div className="playback-resume-prompt__content">
        <p className="playback-resume-prompt__eyebrow">Resume playback</p>
        <div className="playback-resume-prompt__actions">
          <button
            type="button"
            className="playback-resume-prompt__resume"
            onClick={onResumeFromProgress}
          >
            Resume from {formatClock(resumePrompt.seconds)}
          </button>
          <button
            type="button"
            className="playback-resume-prompt__restart"
            onClick={onStartFromBeginning}
          >
            Start from beginning
          </button>
        </div>
      </div>
    </div>
  );
}

function PlaybackUpNextOverlay({
  upNextTarget,
  upNextBackdropUrl,
  upNextSecondsLeft,
  onConfirmUpNextNow,
  onDismissUpNext,
}: {
  upNextTarget: MediaItem | null;
  upNextBackdropUrl: string;
  upNextSecondsLeft: number;
  onConfirmUpNextNow: () => void;
  onDismissUpNext: () => void;
}) {
  if (upNextTarget == null) return null;

  const upNextSeasonLabel = getSeasonEpisodeLabel(upNextTarget);

  return (
    <div className="playback-up-next" role="dialog" aria-modal="true" aria-label="Up next">
      {upNextBackdropUrl ? (
        <img src={upNextBackdropUrl} alt="" className="playback-up-next__bg" />
      ) : (
        <div className="playback-up-next__bg playback-up-next__bg--empty" aria-hidden />
      )}
      <div className="playback-up-next__scrim" />
      <div className="playback-up-next__content">
        <p className="playback-up-next__eyebrow">Up next</p>
        <h2 className="playback-up-next__title">{upNextTarget.title}</h2>
        {upNextSeasonLabel ? <p className="playback-up-next__meta">{upNextSeasonLabel}</p> : null}
        <p className="playback-up-next__timer">
          Starting in <span className="playback-up-next__timer-value">{upNextSecondsLeft}</span>s
        </p>
        <div className="playback-up-next__actions">
          <button type="button" className="playback-up-next__play-now" onClick={onConfirmUpNextNow}>
            Play now
          </button>
          <button type="button" className="playback-up-next__cancel" onClick={onDismissUpNext}>
            Cancel
          </button>
        </div>
      </div>
    </div>
  );
}
