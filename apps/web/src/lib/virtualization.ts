import { useEffect, useRef, useState, type RefObject } from "react";

function resolveScrollParent(element: HTMLElement | null) {
  return element?.closest(".main-content") as HTMLElement | null;
}

export function useVirtualContainerMetrics<T extends HTMLElement>(ref: RefObject<T | null>) {
  const [scrollElement, setScrollElement] = useState<HTMLElement | null>(null);
  const [width, setWidth] = useState(0);
  const [scrollMargin, setScrollMargin] = useState(0);

  useEffect(() => {
    const element = ref.current;
    if (!element) return;

    const parent = resolveScrollParent(element);
    setScrollElement(parent);

    const updateMetrics = () => {
      setWidth(element.getBoundingClientRect().width);
      if (!parent) {
        setScrollMargin(0);
        return;
      }
      const elementRect = element.getBoundingClientRect();
      const parentRect = parent.getBoundingClientRect();
      setScrollMargin(elementRect.top - parentRect.top + parent.scrollTop);
    };

    updateMetrics();
    window.addEventListener("resize", updateMetrics);

    if (typeof ResizeObserver === "undefined") {
      return () => window.removeEventListener("resize", updateMetrics);
    }

    const observer = new ResizeObserver(() => updateMetrics());
    observer.observe(element);
    if (parent) observer.observe(parent);

    return () => {
      window.removeEventListener("resize", updateMetrics);
      observer.disconnect();
    };
  }, [ref]);

  return { scrollElement, width, scrollMargin };
}

export function useLoadMoreTrigger({
  root,
  enabled,
  onLoadMore,
  rootMargin = "400px 0px",
}: {
  root: HTMLElement | null;
  enabled: boolean;
  onLoadMore?: () => void;
  rootMargin?: string;
}) {
  const sentinelRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const sentinel = sentinelRef.current;
    if (!enabled || sentinel == null || onLoadMore == null) {
      return;
    }

    if (typeof IntersectionObserver === "undefined") {
      onLoadMore();
      return;
    }

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          onLoadMore();
        }
      },
      {
        root,
        rootMargin,
      },
    );

    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [enabled, onLoadMore, root, rootMargin]);

  return sentinelRef;
}
