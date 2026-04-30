import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { PlaybackInfoPanel } from "./PlaybackInfoPanel";

function renderPanel(overrides: Partial<Parameters<typeof PlaybackInfoPanel>[0]> = {}) {
  return render(
    <PlaybackInfoPanel
      titleDisplay="Test Video"
      videoStatusMessage="Subtitle load failed. Try again."
      wsConnected
      browserFullscreenActive={false}
      showSubtitleRetry={false}
      onRetrySubtitle={vi.fn<() => void>()}
      onToggleBrowserFullscreen={vi.fn<() => void>()}
      onClosePlayer={vi.fn<() => void>()}
      {...overrides}
    />,
  );
}

describe("PlaybackInfoPanel", () => {
  it("shows retry control only for retryable subtitle loads", () => {
    const { rerender } = renderPanel();

    expect(screen.queryByRole("button", { name: "Retry subtitles" })).not.toBeInTheDocument();

    rerender(
      <PlaybackInfoPanel
        titleDisplay="Test Video"
        videoStatusMessage="Subtitle load failed. Try again."
        wsConnected
        browserFullscreenActive={false}
        showSubtitleRetry
        onRetrySubtitle={vi.fn<() => void>()}
        onToggleBrowserFullscreen={vi.fn<() => void>()}
        onClosePlayer={vi.fn<() => void>()}
      />,
    );

    expect(screen.getByRole("button", { name: "Retry subtitles" })).toBeInTheDocument();
  });

  it("invokes subtitle retry from the retry control", () => {
    const onRetrySubtitle = vi.fn<() => void>();
    renderPanel({ showSubtitleRetry: true, onRetrySubtitle });

    fireEvent.click(screen.getByRole("button", { name: "Retry subtitles" }));

    expect(onRetrySubtitle).toHaveBeenCalledTimes(1);
  });
});
