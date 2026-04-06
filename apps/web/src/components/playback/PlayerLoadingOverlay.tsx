export function PlayerLoadingOverlay({
  label,
  fullscreen = false,
}: {
  label: string;
  fullscreen?: boolean;
}) {
  const ariaLabel = label.trim() !== "" ? label : "Loading";
  return (
    <div
      className={`player-loading-overlay${fullscreen ? " player-loading-overlay--fullscreen" : ""}`}
      role="status"
      aria-live="polite"
      aria-label={ariaLabel}
    >
      <div className="player-loading-overlay__spinner" aria-hidden="true" />
      {label.trim() !== "" ? (
        <span className="player-loading-overlay__label">{label}</span>
      ) : null}
    </div>
  );
}
