import { Maximize2, Minimize2, X } from "lucide-react";

export type PlaybackInfoPanelProps = {
  titleDisplay: string;
  videoStatusMessage: string;
  wsConnected: boolean;
  browserFullscreenActive: boolean;
  onToggleBrowserFullscreen: () => void | Promise<void>;
  onClosePlayer: () => void;
};

/** Top title bar, stream status, fullscreen and close actions. */
export function PlaybackInfoPanel({
  titleDisplay,
  videoStatusMessage,
  wsConnected,
  browserFullscreenActive,
  onToggleBrowserFullscreen,
  onClosePlayer,
}: PlaybackInfoPanelProps) {
  return (
    <div className="fullscreen-player__top-bar">
      <div className="fullscreen-player__top-bar-lead" aria-hidden="true" />
      <div className="fullscreen-player__title-area">
        <h2 className="fullscreen-player__title">{titleDisplay}</h2>
        <div className="fullscreen-player__status">
          {videoStatusMessage && (
            <>
              <span className="status-dot" data-connected={wsConnected} />
              <span>{videoStatusMessage}</span>
            </>
          )}
        </div>
      </div>
      <div className="fullscreen-player__top-bar-tail">
        <div className="fullscreen-player__top-bar-actions">
          <button
            type="button"
            className={`fullscreen-player__close-btn${browserFullscreenActive ? " is-active" : ""}`}
            onClick={() => {
              void onToggleBrowserFullscreen();
            }}
            aria-label={
              browserFullscreenActive ? "Exit full screen" : "Full screen on this display"
            }
            title={
              browserFullscreenActive
                ? "Exit full screen"
                : "Full screen on this display (hides browser UI)"
            }
          >
            {browserFullscreenActive ? (
              <Minimize2 className="size-5" strokeWidth={2.25} />
            ) : (
              <Maximize2 className="size-5" strokeWidth={2.25} />
            )}
          </button>
          <button
            type="button"
            className="fullscreen-player__close-btn"
            onClick={(event) => {
              event.stopPropagation();
              onClosePlayer();
            }}
            aria-label="Close player"
            title="Close player"
          >
            <X className="size-5" strokeWidth={2.25} />
          </button>
        </div>
      </div>
    </div>
  );
}
