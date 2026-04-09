import { type RefObject } from "react";
import { Settings } from "lucide-react";
import type { AudioTrackOption, TrackMenuOption } from "@/lib/playback/playerMedia";
import type { SubtitleAppearance } from "@/lib/playbackPreferences";
import { AudioTrackMenu } from "./AudioTrackMenu";
import { PlayerSettingsMenu } from "./PlayerSettingsMenu";
import { SubtitleTrackMenu } from "./SubtitleTrackMenu";

export type PlaybackTrackMenusProps = {
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
};

/** Audio, subtitle, and settings popovers for fullscreen video (right cluster). */
export function PlaybackTrackMenus({
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
}: PlaybackTrackMenusProps) {
  return (
    <div className="fullscreen-player__controls-right">
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
    </div>
  );
}
