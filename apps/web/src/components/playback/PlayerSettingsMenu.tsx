import { type RefObject } from "react";
import {
  subtitlePositionOptions,
  subtitleSizeOptions,
  type SubtitleAppearance,
} from "@/lib/playbackPreferences";

export function PlayerSettingsMenu({
  menuRef,
  preferences,
  videoAutoplayEnabled,
  onChange,
  onVideoAutoplayChange,
}: {
  menuRef: RefObject<HTMLDivElement | null>;
  preferences: SubtitleAppearance;
  videoAutoplayEnabled: boolean;
  onChange: (value: SubtitleAppearance) => void;
  onVideoAutoplayChange: (enabled: boolean) => void;
}) {
  return (
    <div
      ref={menuRef}
      className="player-settings-menu"
      role="dialog"
      aria-label="Player settings"
    >
      <div className="player-settings-menu__field">
        <span id="player-settings-subtitle-size">Subtitle size</span>
        <div
          className="player-settings-menu__choice-row player-settings-menu__choice-row--thirds"
          role="group"
          aria-labelledby="player-settings-subtitle-size"
        >
          {subtitleSizeOptions.map((option) => (
            <button
              key={option.value}
              type="button"
              className={`player-settings-menu__choice${preferences.size === option.value ? " is-active" : ""}`}
              onClick={() => onChange({ ...preferences, size: option.value })}
            >
              {option.label}
            </button>
          ))}
        </div>
      </div>

      <div className="player-settings-menu__field">
        <span id="player-settings-subtitle-location">Subtitle location</span>
        <div
          className="player-settings-menu__choice-row"
          role="group"
          aria-labelledby="player-settings-subtitle-location"
        >
          {subtitlePositionOptions.map((option) => (
            <button
              key={option.value}
              type="button"
              className={`player-settings-menu__choice${preferences.position === option.value ? " is-active" : ""}`}
              onClick={() => onChange({ ...preferences, position: option.value })}
            >
              {option.label}
            </button>
          ))}
        </div>
      </div>

      <label className="player-settings-menu__field">
        <span>Subtitle color</span>
        <input
          type="color"
          value={preferences.color}
          onChange={(event) =>
            onChange({
              ...preferences,
              color: event.target.value,
            })
          }
        />
      </label>

      <label className="player-settings-menu__field player-settings-menu__checkbox-row">
        <input
          type="checkbox"
          checked={videoAutoplayEnabled}
          onChange={(event) => onVideoAutoplayChange(event.target.checked)}
        />
        <span>Autoplay next</span>
      </label>
    </div>
  );
}
