import type { MouseEvent, ReactNode } from "react";

export type PosterCardState = "default" | "identifying" | "identify-failed" | "review-needed";
export type PosterAspectRatio = "poster" | "cinema" | "square" | "landscape";

export type PosterGridItem = {
  key: string;
  title: string;
  subtitle: string;
  metaLine?: string;
  posterPath?: string;
  posterUrl?: string;
  ratingLabel?: string;
  ratingValue?: number;
  progressPercent?: number;
  cardState?: PosterCardState;
  statusLabel?: string;
  statusActionLabel?: string;
  statusActionDisabled?: boolean;
  href?: string;
  onClick?: () => void;
  onPlay?: () => void;
  onStatusAction?: () => void;
  onContextMenu?: (event: MouseEvent<HTMLDivElement>) => void;
  topBadge?: ReactNode;
};
