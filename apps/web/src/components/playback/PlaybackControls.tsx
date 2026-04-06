import { type PointerEvent, type RefObject } from "react";
import {
  FastForward,
  Pause,
  Play,
  Ratio,
  Repeat,
  Rewind,
  Settings,
  SkipBack,
  SkipForward,
  Volume2,
  VolumeX,
} from "lucide-react";
import { formatClock } from "@/lib/playback/playerMedia";
import type { AudioTrackOption, TrackMenuOption } from "@/lib/playback/playerMedia";
import type { SubtitleAppearance, VideoAspectMode } from "@/lib/playbackPreferences";
import { AudioTrackMenu } from "./AudioTrackMenu";
import { VIDEO_SKIP_BUTTON_SECONDS } from "./constants";
import { PlayerSettingsMenu } from "./PlayerSettingsMenu";
import { SubtitleTrackMenu } from "./SubtitleTrackMenu";
import { TrackMenu } from "./TrackMenu";

export type PlaybackControlsProps = {
  overlayRef: RefObject<HTMLDivElement | null>;
  onOverlayMouseEnter: () => void;
  progressMax: number;
  seekSliderRef: RefObject<HTMLInputElement | null>;
  seekSliderDisplayValue: number;
  onSeekPointerDown: (event: PointerEvent<HTMLInputElement>) => void;
  /** `change` and `input` on the range both route here (types differ between the two events). */
  onSeekChange: (event: { currentTarget: HTMLInputElement }) => void;
  isPlaying: boolean;
  onTogglePlayPause: () => void;
  seekTimeLabelSec: number;
  onSeekRelative: (deltaSeconds: number) => void;
  showAspectControls: boolean;
  aspectBtnRef: RefObject<HTMLButtonElement | null>;
  aspectMenuRef: RefObject<HTMLDivElement | null>;
  aspectMenuOpen: boolean;
  onAspectButtonClick: () => void;
  videoAspectMode: VideoAspectMode;
  aspectTrackMenuOptions: TrackMenuOption[];
  onSelectAspect: (key: string) => void;
  showSubtitleControls: boolean;
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
  showSettingsControls: boolean;
  playerSettingsBtnRef: RefObject<HTMLButtonElement | null>;
  playerSettingsMenuRef: RefObject<HTMLDivElement | null>;
  playerSettingsOpen: boolean;
  onPlayerSettingsButtonClick: () => void;
  subtitleAppearance: SubtitleAppearance;
  onSubtitleAppearanceChange: (value: SubtitleAppearance) => void;
  videoAutoplayEnabled: boolean;
  onVideoAutoplayEnabledChange: (enabled: boolean) => void;
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

/** Fullscreen player seek bar, transport, track menus, queue shortcuts, and volume. */
export function PlaybackControls(props: PlaybackControlsProps) {
  const {
    overlayRef,
    onOverlayMouseEnter,
    progressMax,
    seekSliderRef,
    seekSliderDisplayValue,
    onSeekPointerDown,
    onSeekChange,
    isPlaying,
    onTogglePlayPause,
    seekTimeLabelSec,
    onSeekRelative,
    showAspectControls,
    aspectBtnRef,
    aspectMenuRef,
    aspectMenuOpen,
    onAspectButtonClick,
    videoAspectMode,
    aspectTrackMenuOptions,
    onSelectAspect,
    showSubtitleControls,
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
    showSettingsControls,
    playerSettingsBtnRef,
    playerSettingsMenuRef,
    playerSettingsOpen,
    onPlayerSettingsButtonClick,
    subtitleAppearance,
    onSubtitleAppearanceChange,
    videoAutoplayEnabled,
    onVideoAutoplayEnabledChange,
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
  } = props;

  return (
    <div
      ref={overlayRef}
      className="fullscreen-player__controls"
      onMouseEnter={onOverlayMouseEnter}
    >
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

      <div className="fullscreen-player__controls-row">
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
        </div>

        <div className="fullscreen-player__controls-right">
          {showAspectControls && (
            <div className="fullscreen-player__aspect-wrap">
              <button
                ref={aspectBtnRef}
                type="button"
                className={`fullscreen-player__ctrl-btn${videoAspectMode !== "auto" ? " is-active" : ""}`}
                aria-label="Aspect ratio"
                title="Aspect ratio"
                onClick={onAspectButtonClick}
              >
                <Ratio className="size-[1.125rem]" strokeWidth={2.25} />
              </button>
              {aspectMenuOpen && (
                <TrackMenu
                  menuRef={aspectMenuRef}
                  options={aspectTrackMenuOptions}
                  selectedKey={videoAspectMode}
                  ariaLabel="Select aspect ratio"
                  onSelect={onSelectAspect}
                />
              )}
            </div>
          )}

          {showSubtitleControls && (
            <SubtitleTrackMenu
              btnRef={subtitleBtnRef}
              menuRef={subtitleMenuRef}
              open={subtitleMenuOpen}
              onButtonClick={onSubtitleButtonClick}
              options={subtitleMenuTrackOptions}
              selectedKey={selectedSubtitleKey}
              onSelectTrack={onSelectSubtitleTrack}
            />
          )}

          <AudioTrackMenu
            btnRef={audioBtnRef}
            menuRef={audioMenuRef}
            open={audioMenuOpen}
            onButtonClick={onAudioButtonClick}
            tracks={audioTracks}
            selectedKey={selectedAudioKey}
            selectedLabel={selectedAudioLabel}
            onSelectTrack={onSelectAudioTrack}
          />

          {showSettingsControls && (
            <div className="fullscreen-player__settings-wrap">
              <button
                ref={playerSettingsBtnRef}
                type="button"
                className="fullscreen-player__ctrl-btn"
                aria-label="Player settings"
                title="Player settings"
                onClick={onPlayerSettingsButtonClick}
              >
                <Settings className="size-[1.125rem]" strokeWidth={2.25} />
              </button>
              {playerSettingsOpen && (
                <PlayerSettingsMenu
                  menuRef={playerSettingsMenuRef}
                  preferences={subtitleAppearance}
                  videoAutoplayEnabled={videoAutoplayEnabled}
                  onChange={onSubtitleAppearanceChange}
                  onVideoAutoplayChange={onVideoAutoplayEnabledChange}
                />
              )}
            </div>
          )}

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
      </div>
    </div>
  );
}
