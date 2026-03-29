import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { useEffect, useRef, useState, type ReactNode } from "react";

export function PageHeader({
  title,
  description,
  meta,
  actions,
  className,
}: {
  title: string;
  description?: string;
  meta?: React.ReactNode;
  actions?: React.ReactNode;
  className?: string;
}) {
  return (
    <header
      className={cn(
        "flex flex-col gap-4 border-b border-[var(--nebula-border)]/80 pb-4 md:flex-row md:items-end md:justify-between",
        className,
      )}
    >
      <div className="min-w-0 space-y-2">
        <h1 className="text-[1.875rem] font-semibold tracking-[-0.03em] text-[var(--nebula-text)]">
          {title}
        </h1>
        {description ? (
          <p className="max-w-3xl text-sm leading-6 text-[var(--nebula-muted)]">{description}</p>
        ) : null}
      </div>

      {(meta || actions) && (
        <div className="flex shrink-0 flex-col items-start gap-3 md:items-end">
          {meta ? <div className="text-sm text-[var(--nebula-muted)]">{meta}</div> : null}
          {actions ? <div className="flex flex-wrap gap-3">{actions}</div> : null}
        </div>
      )}
    </header>
  );
}

export function Surface({
  children,
  className,
  as: Comp = "section",
}: {
  children: React.ReactNode;
  className?: string;
  as?: keyof React.JSX.IntrinsicElements;
}) {
  return (
    <Comp
      className={cn(
        "rounded-[var(--radius-xl)] border border-[var(--nebula-border)] bg-[var(--nebula-panel)]/92 p-5 shadow-[0_18px_40px_rgba(3,8,20,0.14)]",
        className,
      )}
    >
      {children}
    </Comp>
  );
}

export function EmptyState({
  title,
  copy,
  action,
  className,
}: {
  title: string;
  copy: string;
  action?: React.ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "rounded-[var(--radius-xl)] border border-dashed border-[var(--nebula-border)] bg-[var(--nebula-panel)]/55 p-8",
        className,
      )}
    >
      <div className="max-w-2xl space-y-2">
        <h2 className="text-lg font-semibold text-[var(--nebula-text)]">{title}</h2>
        <p className="text-sm leading-6 text-[var(--nebula-muted)]">{copy}</p>
        {action ? <div className="pt-2">{action}</div> : null}
      </div>
    </div>
  );
}

export function InfoBadge({
  children,
  active = false,
  className,
}: {
  children: React.ReactNode;
  active?: boolean;
  className?: string;
}) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-[11px] font-semibold uppercase tracking-[0.16em]",
        active
          ? "border-[var(--nebula-accent)]/35 bg-[var(--nebula-accent-soft)] text-[var(--nebula-accent)]"
          : "border-[var(--nebula-border)] bg-[var(--nebula-panel-alt)]/80 text-[var(--nebula-muted)]",
        className,
      )}
    >
      {children}
    </span>
  );
}

export function HorizontalScrollRail({
  children,
  className,
  contentClassName,
  label,
}: {
  children: ReactNode;
  className?: string;
  contentClassName?: string;
  label: string;
}) {
  const viewportRef = useRef<HTMLDivElement>(null);
  const [canScrollLeft, setCanScrollLeft] = useState(false);
  const [canScrollRight, setCanScrollRight] = useState(false);

  useEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport) {
      return;
    }

    const updateState = () => {
      const maxScrollLeft = viewport.scrollWidth - viewport.clientWidth;
      setCanScrollLeft(viewport.scrollLeft > 8);
      setCanScrollRight(viewport.scrollLeft < maxScrollLeft - 8);
    };

    let frame = 0;
    const scheduleUpdate = () => {
      window.cancelAnimationFrame(frame);
      frame = window.requestAnimationFrame(updateState);
    };

    updateState();
    viewport.addEventListener("scroll", scheduleUpdate, { passive: true });
    window.addEventListener("resize", scheduleUpdate);

    const observer =
      typeof ResizeObserver !== "undefined" ? new ResizeObserver(scheduleUpdate) : null;
    observer?.observe(viewport);

    return () => {
      viewport.removeEventListener("scroll", scheduleUpdate);
      window.removeEventListener("resize", scheduleUpdate);
      observer?.disconnect();
      window.cancelAnimationFrame(frame);
    };
  }, []);

  const scrollByAmount = (direction: -1 | 1) => {
    const viewport = viewportRef.current;
    if (!viewport) {
      return;
    }

    const amount = Math.max(viewport.clientWidth * 0.8, 280);
    viewport.scrollBy({ left: amount * direction, behavior: "smooth" });
  };

  return (
    <div className={cn("relative", className)}>
      <div
        ref={viewportRef}
        aria-label={label}
        className={cn("horizontal-scroll-rail scroll-smooth", contentClassName)}
      >
        {children}
      </div>

      <Button
        type="button"
        variant="secondary"
        size="icon"
        aria-label={`Scroll ${label} left`}
        className={cn(
          "absolute left-2 top-1/2 -translate-y-1/2 shadow-lg transition-opacity",
          canScrollLeft ? "opacity-100" : "pointer-events-none opacity-0",
        )}
        onClick={() => scrollByAmount(-1)}
      >
        <ChevronLeft className="size-5" />
      </Button>

      <Button
        type="button"
        variant="secondary"
        size="icon"
        aria-label={`Scroll ${label} right`}
        className={cn(
          "absolute right-2 top-1/2 -translate-y-1/2 shadow-lg transition-opacity",
          canScrollRight ? "opacity-100" : "pointer-events-none opacity-0",
        )}
        onClick={() => scrollByAmount(1)}
      >
        <ChevronRight className="size-5" />
      </Button>
    </div>
  );
}
