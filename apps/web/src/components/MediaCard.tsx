import { resolvePosterUrl } from "@plum/shared";
import { Play } from "lucide-react";
import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { BASE_URL } from "../api";
import type { PosterGridItem } from "./types";

export function MediaCard({
  item,
  className,
}: {
  item: PosterGridItem;
  className?: string;
}) {
  const posterUrl = resolvePosterUrl(item.posterUrl, item.posterPath, "w200", BASE_URL);
  const [posterErrored, setPosterErrored] = useState(false);
  const cardState = item.cardState ?? "default";
  const hasInlineAction = item.actionLabel != null;
  const progressPercent =
    item.progressPercent != null
      ? Math.max(0, Math.min(100, item.progressPercent))
      : 0;

  const showIdentifyingShell = cardState === "identifying" && (!posterUrl || posterErrored);
  const showFailedShell = cardState === "identify-failed" && (!posterUrl || posterErrored);
  const showPlaceholderPoster = cardState === "default" && (!posterUrl || posterErrored);

  useEffect(() => {
    setPosterErrored(false);
  }, [posterUrl]);

  return (
    <div className={`show-card ${className ?? ""}`} onContextMenu={item.onContextMenu}>
      {item.href ? (
        <Link
          to={item.href}
          className={`show-card-hit-area${hasInlineAction ? " show-card-hit-area--with-inline-action" : ""}`}
          aria-label={item.title}
        />
      ) : item.onClick ? (
        <button
          type="button"
          className={`show-card-hit-area show-card-hit-area-button${hasInlineAction ? " show-card-hit-area--with-inline-action" : ""}`}
          aria-label={item.title}
          onClick={item.onClick}
        />
      ) : (
        <div
          className={`show-card-hit-area${hasInlineAction ? " show-card-hit-area--with-inline-action" : ""}`}
          aria-hidden="true"
        />
      )}

      {item.onPlay && cardState === "default" && (
        <button
          type="button"
          className="show-card-play-button"
          aria-label={`Play ${item.title}`}
          onClick={(event) => {
            event.preventDefault();
            event.stopPropagation();
            item.onPlay?.();
          }}
        >
          <Play className="size-5 fill-current" />
        </button>
      )}

      <div className="show-card-content">
        <div
          className={`show-card-poster${cardState === "identifying" ? " show-card-poster--identifying" : ""}`}
        >
          {showIdentifyingShell ? (
            <div
              className="show-card-poster-shell show-card-poster-shell--identifying"
              aria-hidden="true"
            />
          ) : showFailedShell ? (
            <div
              className="show-card-poster-shell show-card-poster-shell--failed"
              aria-hidden="true"
            />
          ) : showPlaceholderPoster ? (
            <img src="/placeholder-poster.svg" alt="" />
          ) : (
            <img src={posterUrl} alt="" onError={() => setPosterErrored(true)} />
          )}

          {cardState !== "default" && (
            <div className={`show-card-status show-card-status--${cardState}`}>
              {item.statusLabel && (
                <span className="show-card-status-label">{item.statusLabel}</span>
              )}
              {item.statusActionLabel && item.onStatusAction && (
                <button
                  type="button"
                  className="show-card-status-action"
                  disabled={item.statusActionDisabled}
                  onClick={(event) => {
                    event.preventDefault();
                    event.stopPropagation();
                    item.onStatusAction?.();
                  }}
                >
                  {item.statusActionLabel}
                </button>
              )}
            </div>
          )}

          {item.topBadge ? (
            <div className="absolute inset-x-0 top-0 flex items-start justify-between gap-2 p-3">
              <div className="flex flex-wrap items-start gap-2">{item.topBadge}</div>
            </div>
          ) : null}

          {progressPercent > 0 && progressPercent < 95 && (
            <div className="show-card-progress" aria-hidden="true">
              <div className="show-card-progress__value" style={{ width: `${progressPercent}%` }} />
            </div>
          )}
        </div>

        <div className="show-card-info">
          <div className="show-card-title">{item.title}</div>
          <div className="show-card-count">{item.subtitle}</div>
          <div className="show-card-meta">
            {item.ratingValue ? (
              <span className="show-card-imdb">
                <span className="show-card-imdb__mark">{item.ratingLabel ?? "Rating"}</span>
                <span>{item.ratingValue.toFixed(1)}</span>
              </span>
            ) : (
              <span className="show-card-meta__copy show-card-meta__copy--empty" aria-hidden="true">
                &nbsp;
              </span>
            )}
            {item.metaLine ? <span className="show-card-meta__copy">{item.metaLine}</span> : null}
          </div>
          {item.actionLabel ? (
            <div className="show-card-action-row">
              <button
                type="button"
                className={`show-card-inline-action show-card-inline-action--${item.actionTone ?? "default"}`}
                disabled={item.actionDisabled}
                onClick={(event) => {
                  event.preventDefault();
                  event.stopPropagation();
                  item.onAction?.();
                }}
              >
                {item.actionLabel}
              </button>
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
}

export default MediaCard;
