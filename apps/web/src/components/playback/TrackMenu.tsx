import { type RefObject } from "react";
import type { TrackMenuOption } from "@/lib/playback/playerMedia";

/** Popover listbox for aspect, subtitle, and audio track selection (docked & fullscreen player). */
export function TrackMenu({
  options,
  selectedKey,
  onSelect,
  menuRef,
  position = "above",
  /** `end`: align menu to the control's right edge (e.g. right-cluster track pickers). */
  menuAlign = "center",
  ariaLabel,
  offLabel,
}: {
  options: TrackMenuOption[];
  selectedKey: string;
  onSelect: (key: string) => void;
  menuRef: RefObject<HTMLDivElement | null>;
  position?: "above" | "below";
  menuAlign?: "center" | "end";
  ariaLabel: string;
  offLabel?: string;
}) {
  const alignClass =
    menuAlign === "end" ? " subtitle-menu--align-end" : "";
  return (
    <div
      ref={menuRef}
      className={`subtitle-menu subtitle-menu--${position}${alignClass}`}
      role="listbox"
      aria-label={ariaLabel}
    >
      {offLabel && (
        <button
          type="button"
          role="option"
          aria-selected={selectedKey === "off"}
          className={`subtitle-menu__item${selectedKey === "off" ? " is-selected" : ""}`}
          onClick={() => onSelect("off")}
        >
          <span className="subtitle-menu__check">{selectedKey === "off" ? "✓" : ""}</span>
          <span>{offLabel}</span>
        </button>
      )}
      {options.map((option) => (
        <button
          key={option.key}
          type="button"
          role="option"
          aria-selected={selectedKey === option.key}
          disabled={option.disabled}
          className={`subtitle-menu__item${selectedKey === option.key ? " is-selected" : ""}`}
          onClick={() => onSelect(option.key)}
        >
          <span className="subtitle-menu__check">{selectedKey === option.key ? "✓" : ""}</span>
          <span>{option.label}</span>
        </button>
      ))}
    </div>
  );
}
