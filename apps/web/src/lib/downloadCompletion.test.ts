import { describe, expect, it } from "vitest";
import { diffRemovedDownloads, downloadsToSnapshotMap } from "./downloadCompletion";

describe("diffRemovedDownloads", () => {
  it("returns empty when nothing was removed", () => {
    const items = [
      { id: "radarr:1", title: "A", media_type: "movie" as const, source: "radarr" as const, status_text: "x" },
    ];
    const prev = downloadsToSnapshotMap(items);
    expect(diffRemovedDownloads(prev, items)).toEqual([]);
  });

  it("detects removed ids and preserves title", () => {
    const prev = downloadsToSnapshotMap([
      { id: "radarr:1", title: "Film", media_type: "movie", source: "radarr", status_text: "x" },
      { id: "sonarr-tv:2", title: "Show", media_type: "tv", source: "sonarr-tv", status_text: "y" },
    ]);
    const next = [
      { id: "radarr:1", title: "Film", media_type: "movie" as const, source: "radarr" as const, status_text: "x" },
    ];
    expect(diffRemovedDownloads(prev, next)).toEqual([
      { id: "sonarr-tv:2", title: "Show", hadError: false },
    ]);
  });

  it("marks hadError when previous row had error_message", () => {
    const prev = new Map([
      ["radarr:9", { title: "Bad", error_message: "disk full" }],
    ]);
    expect(diffRemovedDownloads(prev, [])).toEqual([
      { id: "radarr:9", title: "Bad", hadError: true },
    ]);
  });

  it("uses Unknown title when title is blank", () => {
    const prev = new Map([["radarr:1", { title: "  ", error_message: undefined }]]);
    expect(diffRemovedDownloads(prev, [])).toEqual([
      { id: "radarr:1", title: "Unknown title", hadError: false },
    ]);
  });
});
