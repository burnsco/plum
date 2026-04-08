import {
  type KeyboardEvent,
  type MouseEvent,
  type ReactNode,
  type RefCallback,
} from "react";

export type PlaybackDockShellProps = {
  playerRootRef: RefCallback<HTMLElement | null>;
  controlsVisible: boolean;
  videoAspectMode: string;
  showPlayerLoadingOverlay: boolean;
  onMouseMove: (event: MouseEvent<HTMLElement>) => void;
  onClick: (event: MouseEvent<HTMLElement>) => void;
  onKeyDown: (event: KeyboardEvent<HTMLElement>) => void;
  children: ReactNode;
};

/** Full-viewport in-app video shell (theater); OS fullscreen is separate. */
export function PlaybackDockShell({
  playerRootRef,
  controlsVisible,
  videoAspectMode,
  showPlayerLoadingOverlay,
  onMouseMove,
  onClick,
  onKeyDown,
  children,
}: PlaybackDockShellProps) {
  return (
    <section
      ref={playerRootRef}
      className={`fullscreen-player fullscreen-player--aspect-${videoAspectMode}${
        controlsVisible ? "" : " fullscreen-player--hidden"
      }`}
      aria-label="Video player"
      aria-busy={showPlayerLoadingOverlay}
      role="button"
      tabIndex={0}
      onMouseMove={onMouseMove}
      onClick={onClick}
      onKeyDown={onKeyDown}
    >
      {children}
    </section>
  );
}
