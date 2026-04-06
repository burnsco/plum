import { Volume2 } from "lucide-react";
import { type RefObject } from "react";
import type { AudioTrackOption } from "@/lib/playback/playerMedia";
import { TrackMenu } from "./TrackMenu";

export function AudioTrackMenu({
  btnRef,
  menuRef,
  open,
  onButtonClick,
  tracks,
  selectedKey,
  selectedLabel,
  onSelectTrack,
}: {
  btnRef: RefObject<HTMLButtonElement | null>;
  menuRef: RefObject<HTMLDivElement | null>;
  open: boolean;
  onButtonClick: () => void;
  tracks: AudioTrackOption[];
  selectedKey: string;
  selectedLabel: string;
  onSelectTrack: (key: string) => void;
}) {
  if (tracks.length <= 1) return null;

  return (
    <div className="fullscreen-player__audio-wrap">
      <button
        ref={btnRef}
        type="button"
        className="fullscreen-player__ctrl-btn"
        aria-label={`Audio track: ${selectedLabel}`}
        title={`Audio: ${selectedLabel}`}
        onClick={onButtonClick}
      >
        <Volume2 className="size-[1.125rem]" strokeWidth={2.25} />
      </button>
      {open && (
        <TrackMenu
          menuRef={menuRef}
          options={tracks}
          selectedKey={selectedKey}
          ariaLabel="Select audio track"
          onSelect={onSelectTrack}
        />
      )}
    </div>
  );
}
