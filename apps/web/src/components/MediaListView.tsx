import { resolvePosterUrl } from "@plum/shared";
import { Play, Star } from "lucide-react";
import { Link } from "react-router-dom";
import { BASE_URL } from "@/api";
import { cn } from "@/lib/utils";
import type { PosterGridItem } from "./types";

function RowHitArea({ item }: { item: PosterGridItem }) {
  if (item.href) {
    return <Link to={item.href} className="absolute inset-0 z-10" aria-label={item.title} />;
  }

  if (item.onClick) {
    return (
      <button
        type="button"
        className="absolute inset-0 z-10 cursor-pointer"
        aria-label={item.title}
        onClick={item.onClick}
      />
    );
  }

  return null;
}

/** Detail view: horizontal card with poster on left, metadata on right */
function DetailCard({ item }: { item: PosterGridItem }) {
  const posterUrl = resolvePosterUrl(item.posterUrl, item.posterPath, "w200", BASE_URL);

  return (
    <div
      className={cn(
        "group relative flex gap-4 rounded-[var(--radius-lg)] border border-[var(--plum-border)] bg-[rgba(18,18,18,0.96)] p-3 transition-all hover:border-[rgba(255,255,255,0.12)] hover:bg-[rgba(24,24,24,0.98)]",
        item.href || item.onClick ? "cursor-pointer" : "",
      )}
      onContextMenu={item.onContextMenu}
    >
      <RowHitArea item={item} />

      {/* Poster thumbnail */}
      <div className="relative aspect-[2/3] w-16 shrink-0 overflow-hidden rounded-[var(--radius-md)] bg-[rgba(255,255,255,0.05)]">
        {posterUrl ? (
          <img
            src={posterUrl}
            alt=""
            className="h-full w-full object-cover"
          />
        ) : (
          <div className="h-full w-full bg-[rgba(255,255,255,0.04)]" />
        )}

        {/* Play overlay */}
        {item.onPlay && (
          <button
            type="button"
            aria-label={`Play ${item.title}`}
            onClick={(e) => {
              e.preventDefault();
              e.stopPropagation();
              item.onPlay?.();
            }}
            className="absolute inset-0 z-20 flex items-center justify-center bg-black/50 opacity-0 transition-opacity group-hover:opacity-100"
          >
            <Play className="size-5 fill-white text-white" />
          </button>
        )}
      </div>

      {/* Info */}
      <div className="flex min-w-0 flex-1 flex-col gap-1 py-0.5">
        <div className="text-sm font-semibold text-[var(--plum-text)] truncate">{item.title}</div>
        <div className="text-xs text-[var(--plum-muted)]">{item.subtitle}</div>
        {item.metaLine && (
          <div className="text-xs text-[var(--plum-muted)] truncate">{item.metaLine}</div>
        )}
        {item.ratingValue && (
          <div className="mt-auto flex items-center gap-1 text-xs text-[#f7c44f]">
            <Star className="size-3 fill-current" />
            <span>{item.ratingValue.toFixed(1)}</span>
            {item.ratingLabel && (
              <span className="text-[var(--plum-muted)]">· {item.ratingLabel}</span>
            )}
          </div>
        )}
        {item.progressPercent != null && item.progressPercent > 0 && item.progressPercent < 95 && (
          <div className="h-0.5 w-full rounded-full bg-[rgba(255,255,255,0.1)]">
            <div
              className="h-full rounded-full bg-[#f7c44f]"
              style={{ width: `${item.progressPercent}%` }}
            />
          </div>
        )}
        {item.statusLabel && (
          <div className="text-xs text-[var(--plum-muted)] italic">{item.statusLabel}</div>
        )}
        {item.statusActionLabel && item.onStatusAction && (
          <div className="pt-1">
            <button
              type="button"
              disabled={item.statusActionDisabled}
              onClick={(e) => {
                e.preventDefault();
                e.stopPropagation();
                item.onStatusAction?.();
              }}
              className="relative z-20 inline-flex items-center rounded-full border border-[var(--plum-border)] px-3 py-1 text-[11px] font-medium text-[var(--plum-text)] transition-colors hover:border-[rgba(255,255,255,0.16)] hover:bg-[rgba(255,255,255,0.06)] disabled:cursor-not-allowed disabled:opacity-50"
            >
              {item.statusActionLabel}
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

/** Table row — compact single-line row, poster as small thumbnail */
function TableRow({
  item,
  index,
}: {
  item: PosterGridItem;
  index: number;
}) {
  const posterUrl = resolvePosterUrl(item.posterUrl, item.posterPath, "w200", BASE_URL);

  return (
    <div
      className={cn(
        "group relative grid items-center gap-3 border-b border-[var(--plum-border)] px-1 py-2.5 transition-colors hover:bg-[rgba(255,255,255,0.03)]",
        "grid-cols-[auto_2rem_minmax(0,1fr)_auto_auto]",
      )}
      onContextMenu={item.onContextMenu}
    >
      <RowHitArea item={item} />

      {/* Index */}
      <div className="w-6 text-right text-xs text-[var(--plum-muted)] tabular-nums">
        {index + 1}
      </div>

      {/* Poster thumbnail */}
      <div className="aspect-[2/3] w-8 shrink-0 overflow-hidden rounded bg-[rgba(255,255,255,0.05)]">
        {posterUrl ? (
          <img
            src={posterUrl}
            alt=""
            className="h-full w-full object-cover"
          />
        ) : (
          <div className="h-full w-full bg-[rgba(255,255,255,0.04)]" />
        )}
      </div>

      {/* Title + subtitle */}
      <div className="min-w-0">
        <div className="truncate text-sm font-medium text-[var(--plum-text)]">{item.title}</div>
        <div className="truncate text-xs text-[var(--plum-muted)]">{item.subtitle}</div>
        {item.statusLabel ? (
          <div className="truncate text-[11px] italic text-[var(--plum-muted)]">
            {item.statusLabel}
          </div>
        ) : null}
      </div>

      {/* Rating */}
      {item.ratingValue ? (
        <div className="flex items-center gap-1 text-xs text-[#f7c44f]">
          <Star className="size-3 fill-current" />
          {item.ratingValue.toFixed(1)}
        </div>
      ) : (
        <div />
      )}

      {/* Play button */}
      <div className="relative z-20 flex items-center justify-end gap-1">
        {item.statusActionLabel && item.onStatusAction ? (
          <button
            type="button"
            disabled={item.statusActionDisabled}
            aria-label={item.statusActionLabel}
            onClick={(e) => {
              e.preventDefault();
              e.stopPropagation();
              item.onStatusAction?.();
            }}
            className="inline-flex h-7 items-center rounded-full border border-[var(--plum-border)] px-2.5 text-[11px] font-medium text-[var(--plum-text)] transition-colors hover:border-[rgba(255,255,255,0.16)] hover:bg-[rgba(255,255,255,0.06)] disabled:cursor-not-allowed disabled:opacity-50"
          >
            {item.statusActionLabel}
          </button>
        ) : null}
        {item.onPlay ? (
          <button
            type="button"
            aria-label={`Play ${item.title}`}
            onClick={(e) => {
              e.preventDefault();
              e.stopPropagation();
              item.onPlay?.();
            }}
            className="flex size-7 items-center justify-center rounded-full border border-transparent text-[var(--plum-muted)] opacity-0 transition-all hover:border-[var(--plum-border)] hover:text-[var(--plum-text)] group-hover:opacity-100"
          >
            <Play className="size-3.5 fill-current" />
          </button>
        ) : (
          <div />
        )}
      </div>
    </div>
  );
}

/** Detail view: 2-column grid of horizontal detail cards */
export function MediaDetailView({ items }: { items: PosterGridItem[] }) {
  return (
    <div className="mt-3 grid grid-cols-1 gap-2 sm:grid-cols-2">
      {items.map((item) => (
        <DetailCard key={item.key} item={item} />
      ))}
    </div>
  );
}

/** Table view: compact flat list with index, thumbnail, title, rating */
export function MediaTableView({ items }: { items: PosterGridItem[] }) {
  return (
    <div className="mt-3 divide-y-0 rounded-[var(--radius-lg)] border border-[var(--plum-border)] bg-[rgba(14,14,14,0.98)]">
      {/* Header */}
      <div className="grid grid-cols-[auto_2rem_minmax(0,1fr)_auto_auto] items-center gap-3 border-b border-[var(--plum-border)] px-1 py-2">
        <div className="w-6 text-right text-[10px] font-semibold uppercase tracking-wider text-[var(--plum-muted)]">#</div>
        <div />
        <div className="text-[10px] font-semibold uppercase tracking-wider text-[var(--plum-muted)]">Title</div>
        <div className="text-[10px] font-semibold uppercase tracking-wider text-[var(--plum-muted)]">Rating</div>
        <div />
      </div>
      {items.map((item, i) => (
        <TableRow key={item.key} item={item} index={i} />
      ))}
    </div>
  );
}
