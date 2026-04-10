import { Subtitles } from "lucide-react";
import { type RefObject } from "react";
import type { TrackMenuOption } from "@/lib/playback/playerMedia";
import { TrackMenu } from "./TrackMenu";

export function SubtitleTrackMenu({
  btnRef,
  menuRef,
  open,
  onButtonClick,
  options,
  selectedKey,
  onSelectTrack,
}: {
  btnRef: RefObject<HTMLButtonElement | null>;
  menuRef: RefObject<HTMLDivElement | null>;
  open: boolean;
  onButtonClick: () => void;
  options: TrackMenuOption[];
  selectedKey: string;
  onSelectTrack: (key: string) => void;
}) {
  return (
    <div className="fullscreen-player__subtitle-wrap">
      <button
        ref={btnRef}
        type="button"
        className={`fullscreen-player__ctrl-btn${selectedSubtitleActive(selectedKey) ? " is-active" : ""}`}
        aria-label="Subtitles"
        title="Subtitles"
        onClick={onButtonClick}
      >
        <Subtitles className="size-[1.125rem]" strokeWidth={2.25} />
      </button>
      {open && (
        <TrackMenu
          menuRef={menuRef}
          menuAlign="end"
          options={options}
          selectedKey={selectedKey}
          ariaLabel="Select subtitle track"
          offLabel="Off"
          onSelect={onSelectTrack}
        />
      )}
    </div>
  );
}

function selectedSubtitleActive(selectedKey: string): boolean {
  return selectedKey !== "off";
}
